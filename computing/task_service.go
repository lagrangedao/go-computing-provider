package computing

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/filswan/go-mcs-sdk/mcs/api/common/logs"
	"github.com/gomodule/redigo/redis"
	"github.com/lagrangedao/go-computing-provider/conf"
	"github.com/lagrangedao/go-computing-provider/constants"
	"github.com/lagrangedao/go-computing-provider/docker"
	"github.com/lagrangedao/go-computing-provider/models"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"strings"
	"sync"
	"time"
)

var runTaskGpuResource sync.Map
var deployingChan = make(chan models.Job, 10)

type ScheduleTask struct {
	TaskMap sync.Map
}

func NewScheduleTask() *ScheduleTask {
	return &ScheduleTask{}
}

func (s *ScheduleTask) Run() {
	for {
		select {
		case job := <-deployingChan:
			s.TaskMap.Store(job.Uuid, job)

		case <-time.After(15 * time.Second):
			s.TaskMap.Range(func(key, value any) bool {
				jobUuid := key.(string)
				job := value.(models.Job)
				reportJobStatus(jobUuid, job.Status)
				return true
			})
		case <-time.After(10 * time.Minute):
			s.TaskMap.Range(func(key, value any) bool {
				job := value.(models.Job)
				job.Count++

				if job.Count > 20 {
					s.TaskMap.Delete(job.Uuid)
					return true
				}

				if job.Status != models.JobDeployToK8s {
					return true
				}

				response, err := http.Get(job.Url)
				if err != nil {
					return true
				}
				defer response.Body.Close()

				if response.StatusCode == 200 {
					s.TaskMap.Delete(job.Uuid)
				}
				return true
			})
		}
	}
}

func reportJobStatus(jobUuid string, jobStatus models.JobStatus) {
	reqParam := map[string]interface{}{
		"job_uuid": jobUuid,
		"status":   jobStatus,
	}

	payload, err := json.Marshal(reqParam)
	if err != nil {
		logs.GetLogger().Errorf("Failed convert to json, error: %+v", err)
		return
	}

	client := &http.Client{}
	url := conf.GetConfig().LAG.ServerUrl + "/job/status"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		logs.GetLogger().Errorf("Error creating request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+conf.GetConfig().LAG.AccessToken)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		logs.GetLogger().Errorf("Failed send a request, error: %+v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logs.GetLogger().Debugf("report job status Failed. uuid: %s, status: %s", jobUuid, jobStatus)
		return
	}
	return
}

func RunSyncTask(nodeId string) {
	go func() {
		k8sService := NewK8sService()
		nodes, err := k8sService.k8sClient.CoreV1().Nodes().List(context.TODO(), metaV1.ListOptions{})
		if err != nil {
			logs.GetLogger().Error(err)
			return
		}

		nodeGpuInfoMap, err := k8sService.GetPodLog(context.TODO())
		if err != nil {
			logs.GetLogger().Error(err)
			return
		}

		logs.GetLogger().Infof("collect all node: %d", len(nodes.Items))
		for _, node := range nodes.Items {
			cpNode := node
			if gpu, ok := nodeGpuInfoMap[cpNode.Name]; ok {
				var gpuInfo struct {
					Gpu models.Gpu `json:"gpu"`
				}
				if err = json.Unmarshal([]byte(gpu.String()), &gpuInfo); err != nil {
					logs.GetLogger().Errorf("convert to json, nodeName %s, error: %+v", cpNode.Name, err)
					continue
				}
				for _, detail := range gpuInfo.Gpu.Details {
					if err = k8sService.AddNodeLabel(cpNode.Name, detail.ProductName); err != nil {
						logs.GetLogger().Errorf("add node label, nodeName %s, gpuName: %s, error: %+v", cpNode.Name, detail.ProductName, err)
						continue
					}
				}
			}
		}
	}()

	go func() {
		defer func() {
			if err := recover(); err != nil {
				logs.GetLogger().Errorf("Failed report cp resource's summary, error: %+v", err)
			}
		}()

		location, err := getLocation()
		if err != nil {
			logs.GetLogger().Error(err)
		}

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			reportClusterResource(location, nodeId)
		}

	}()

	go func() {
		defer func() {
			if err := recover(); err != nil {
				logs.GetLogger().Errorf("Failed report provider bid status, error: %+v", err)
			}
		}()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		logs.GetLogger().Infof("provider status: %s", models.ActiveStatus)

		for range ticker.C {
			providerStatus, err := checkClusterProviderStatus()
			if err != nil {
				logs.GetLogger().Errorf("check cluster resource failed, error: %+v", err)
				return
			}
			if providerStatus == models.InactiveStatus {
				logs.GetLogger().Infof("provider status: %s", providerStatus)
			}
			updateProviderInfo(nodeId, "", "", providerStatus)
		}

	}()

	watchExpiredTask()
	watchNameSpaceForDeleted()
}

func reportClusterResource(location, nodeId string) {
	k8sService := NewK8sService()
	statisticalSources, err := k8sService.StatisticalSources(context.TODO())
	if err != nil {
		logs.GetLogger().Errorf("Failed k8s statistical sources, error: %+v", err)
		return
	}
	clusterSource := models.ClusterResource{
		NodeId:      nodeId,
		Region:      location,
		ClusterInfo: statisticalSources,
	}

	payload, err := json.Marshal(clusterSource)
	if err != nil {
		logs.GetLogger().Errorf("Failed convert to json, error: %+v", err)
		return
	}

	client := &http.Client{}
	url := conf.GetConfig().LAG.ServerUrl + "/cp/summary"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		logs.GetLogger().Errorf("Error creating request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+conf.GetConfig().LAG.AccessToken)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		logs.GetLogger().Errorf("Failed send a request, error: %+v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logs.GetLogger().Errorf("report cluster node resources failed, status code: %d", resp.StatusCode)
		return
	}
}

func watchExpiredTask() {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				logs.GetLogger().Errorf("catch panic error: %+v", err)
			}
		}()

		var deleteKey []string
		for range ticker.C {
			conn := redisPool.Get()

			prefix := constants.REDIS_FULL_PREFIX + "*"
			keys, err := redis.Strings(conn.Do("KEYS", prefix))
			if err != nil {
				logs.GetLogger().Errorf("Failed get redis %s prefix, error: %+v", prefix, err)
				return
			}
			for _, key := range keys {
				jobMetadata, err := retrieveJobMetadata(key)
				if err != nil {
					logs.GetLogger().Errorf("Failed get redis key data, key: %s, error: %+v", key, err)
					return
				}

				if time.Now().Unix() > jobMetadata.ExpireTime {
					namespace := constants.K8S_NAMESPACE_NAME_PREFIX + strings.ToLower(jobMetadata.WalletAddress)
					expireTimeStr := time.Unix(jobMetadata.ExpireTime, 0).Format("2006-01-02 15:04:05")
					logs.GetLogger().Infof("<timer-task> redis-key: %s, namespace: %s, spaceUuid: %s,expireTime: %s. the job starting terminated", key, namespace, jobMetadata.SpaceUuid, expireTimeStr)

					deleteJob(jobMetadata.WalletAddress, namespace, jobMetadata.SpaceUuid, jobMetadata.SpaceName)
					deleteKey = append(deleteKey, key)
				}

			}
			conn.Do("DEL", redis.Args{}.AddFlat(deleteKey)...)
			if len(deleteKey) > 0 {
				logs.GetLogger().Infof("Delete redis keys finished, keys: %+v", deleteKey)
				deleteKey = nil
			}
		}
	}()
}

func watchNameSpaceForDeleted() {
	ticker := time.NewTicker(20 * time.Hour)
	go func() {
		defer func() {
			if err := recover(); err != nil {
				logs.GetLogger().Errorf("catch panic error: %+v", err)
			}
		}()

		for range ticker.C {
			service := NewK8sService()
			namespaces, err := service.ListNamespace(context.TODO())
			if err != nil {
				logs.GetLogger().Errorf("Failed get all namespace, error: %+v", err)
				continue
			}

			for _, namespace := range namespaces {
				getPods, err := service.GetPods(namespace, "")
				if err != nil {
					logs.GetLogger().Errorf("Failed get pods form namespace,namepace: %s, error: %+v", namespace, err)
					continue
				}
				if !getPods && strings.HasPrefix(namespace, constants.K8S_NAMESPACE_NAME_PREFIX) {
					if err = service.DeleteNameSpace(context.TODO(), namespace); err != nil {
						logs.GetLogger().Errorf("Failed delete namespace, namepace: %s, error: %+v", namespace, err)
					}
				}
			}
			docker.NewDockerService().CleanResource()
		}
	}()
}
