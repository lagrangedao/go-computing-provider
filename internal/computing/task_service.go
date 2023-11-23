package computing

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/filswan/go-mcs-sdk/mcs/api/common/logs"
	"github.com/gomodule/redigo/redis"
	"github.com/lagrangedao/go-computing-provider/conf"
	"github.com/lagrangedao/go-computing-provider/constants"
	models2 "github.com/lagrangedao/go-computing-provider/internal/models"
	"k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"strings"
	"sync"
	"time"
)

var runTaskGpuResource sync.Map
var deployingChan = make(chan models2.Job)

type ScheduleTask struct {
	TaskMap sync.Map
}

func NewScheduleTask() *ScheduleTask {
	return &ScheduleTask{}
}

func (s *ScheduleTask) Run() {
	go func() {
		ticker := time.NewTicker(3 * time.Minute)
		for {
			select {
			case <-ticker.C:
				s.TaskMap.Range(func(key, value any) bool {
					job := value.(*models2.Job)
					job.Count++
					s.TaskMap.Store(job.Uuid, job)

					if job.Count > 50 {
						s.TaskMap.Delete(job.Uuid)
						return true
					}

					if job.Status != models2.JobDeployToK8s {
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
	}()

	for {
		select {
		case job := <-deployingChan:
			s.TaskMap.Store(job.Uuid, &job)
		case <-time.After(15 * time.Second):
			s.TaskMap.Range(func(key, value any) bool {
				jobUuid := key.(string)
				job := value.(*models2.Job)
				reportJobStatus(jobUuid, job.Status)
				return true
			})
		}
	}
}

func reportJobStatus(jobUuid string, jobStatus models2.JobStatus) {
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
		return
	}

	logs.GetLogger().Debugf("report job status successfully. uuid: %s, status: %s", jobUuid, jobStatus)
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
					Gpu models2.Gpu `json:"gpu"`
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

		logs.GetLogger().Infof("provider status: %s", models2.ActiveStatus)

		for range ticker.C {
			providerStatus, err := checkClusterProviderStatus()
			if err != nil {
				logs.GetLogger().Errorf("check cluster resource failed, error: %+v", err)
				return
			}
			if providerStatus == models2.InactiveStatus {
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
	clusterSource := models2.ClusterResource{
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
		var deleteKey []string
		for range ticker.C {
			go func() {
				defer func() {
					if err := recover(); err != nil {
						logs.GetLogger().Errorf("catch panic error: %+v", err)
					}
				}()
				conn := redisPool.Get()
				prefix := constants.REDIS_FULL_PREFIX + "*"
				keys, err := redis.Strings(conn.Do("KEYS", prefix))
				if err != nil {
					logs.GetLogger().Errorf("Failed get redis %s prefix, error: %+v", prefix, err)
					return
				}
				for _, key := range keys {
					jobMetadata, err := RetrieveJobMetadata(key)
					if err != nil {
						logs.GetLogger().Errorf("Failed get redis key data, key: %s, error: %+v", key, err)
						return
					}

					if time.Now().Unix() > jobMetadata.ExpireTime {
						namespace := constants.K8S_NAMESPACE_NAME_PREFIX + strings.ToLower(jobMetadata.WalletAddress)
						expireTimeStr := time.Unix(jobMetadata.ExpireTime, 0).Format("2006-01-02 15:04:05")
						logs.GetLogger().Infof("<timer-task> redis-key: %s, namespace: %s,expireTime: %s. the job starting terminated", key, namespace, expireTimeStr)
						if err = deleteJob(namespace, jobMetadata.SpaceUuid); err == nil {
							deleteKey = append(deleteKey, key)
							continue
						}
					}

					k8sNameSpace := constants.K8S_NAMESPACE_NAME_PREFIX + strings.ToLower(jobMetadata.WalletAddress)
					deployName := constants.K8S_DEPLOY_NAME_PREFIX + jobMetadata.SpaceUuid
					service := NewK8sService()
					if _, err = service.k8sClient.AppsV1().Deployments(k8sNameSpace).Get(context.TODO(), deployName, metaV1.GetOptions{}); err != nil && errors.IsNotFound(err) {
						deleteKey = append(deleteKey, key)
						continue
					}
				}
				conn.Do("DEL", redis.Args{}.AddFlat(deleteKey)...)
				if len(deleteKey) > 0 {
					logs.GetLogger().Infof("Delete redis keys finished, keys: %+v", deleteKey)
					deleteKey = nil
				}
			}()
		}
	}()
}

func watchNameSpaceForDeleted() {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			go func() {
				defer func() {
					if err := recover(); err != nil {
						logs.GetLogger().Errorf("catch panic error: %+v", err)
					}
				}()
				service := NewK8sService()
				namespaces, err := service.ListNamespace(context.TODO())
				if err != nil {
					logs.GetLogger().Errorf("Failed get all namespace, error: %+v", err)
					return
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
				NewDockerService().CleanResource()
			}()
		}
	}()
}
