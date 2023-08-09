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
	"strconv"
	"strings"
	"sync"
	"time"
)

var runTaskGpuResource sync.Map

func RunSyncTask() {
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

		for _, node := range nodes.Items {
			cpNode := node
			if gpu, ok := nodeGpuInfoMap[cpNode.Name]; ok {
				var gpuInfo struct {
					Gpu models.Gpu `json:"gpu"`
				}
				if err = json.Unmarshal([]byte(gpu.String()), &gpuInfo); err != nil {
					logs.GetLogger().Error(err)
					return
				}
				for _, detail := range gpuInfo.Gpu.Details {
					if err = k8sService.AddNodeLabel(cpNode.Name, detail.ProductName); err != nil {
						logs.GetLogger().Error(err)
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
		nodeId, _, _ := generateNodeID()

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			reportClusterResource(location, nodeId)
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
	url := conf.GetConfig().LAD.ServerUrl + "/cp/summary"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		logs.GetLogger().Errorf("Error creating request: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+conf.GetConfig().LAD.AccessToken)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		logs.GetLogger().Errorf("Failed send a request, error: %+v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logs.GetLogger().Errorf("The request url: %s, returns a non-200 status code: %d", url, resp.StatusCode)
		return
	}
	logs.GetLogger().Info("report cluster node resources successfully")
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
				args := []interface{}{key}
				args = append(args, "k8s_namespace", "space_name", "expire_time")
				valuesStr, err := redis.Strings(conn.Do("HMGET", args...))
				if err != nil {
					logs.GetLogger().Errorf("Failed get redis key data, key: %s, error: %+v", key, err)
					return
				}

				if len(valuesStr) >= 3 {
					namespace := valuesStr[0]
					spaceName := valuesStr[1]
					expireTimeStr := valuesStr[2]
					expireTime, err := strconv.ParseInt(strings.TrimSpace(expireTimeStr), 10, 64)
					if err != nil {
						logs.GetLogger().Errorf("Failed convert time str: [%s], error: %+v", expireTimeStr, err)
						return
					}
					if time.Now().Unix() > expireTime {
						logs.GetLogger().Infof("<timer-task> redis-key: %s, namespace: %s, spacename: %s, the job has expired, and the service starting terminated", key, namespace, spaceName)
						deleteJob(namespace, spaceName)
						deleteKey = append(deleteKey, key)
					}
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
