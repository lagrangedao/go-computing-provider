package computing

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lagrangedao/go-computing-provider/conf"

	"github.com/filswan/go-mcs-sdk/mcs/api/common/logs"
	"github.com/gin-gonic/gin"
	"github.com/gomodule/redigo/redis"
	"github.com/google/uuid"
	"github.com/lagrangedao/go-computing-provider/common"
	"github.com/lagrangedao/go-computing-provider/constants"
	"github.com/lagrangedao/go-computing-provider/models"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetServiceProviderInfo(c *gin.Context) {
	info := new(models.HostInfo)
	//info.SwanProviderVersion = common.GetVersion()
	info.OperatingSystem = runtime.GOOS
	info.Architecture = runtime.GOARCH
	info.CPUCores = runtime.NumCPU()
	c.JSON(http.StatusOK, common.CreateSuccessResponse(info))
}

func ReceiveJob(c *gin.Context) {
	var jobData models.JobData

	if err := c.ShouldBindJSON(&jobData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	logs.GetLogger().Infof("Job received: %s", jobData.JobSourceURI)

	jobSourceURI := jobData.JobSourceURI
	creator, spaceName, err := getSpaceName(jobSourceURI)
	if err != nil {
		logs.GetLogger().Errorf("Failed get space name: %v", err)
		return
	}

	hostPrefix := generateString(10)
	delayTask, err := celeryService.DelayTask(constants.TASK_DEPLOY, creator, spaceName, jobSourceURI, jobData.Hardware, hostPrefix, jobData.Duration)
	if err != nil {
		logs.GetLogger().Errorf("Failed sync delpoy task, error: %v", err)
		return
	}
	go func() {
		result, err := delayTask.Get(180 * time.Second)
		if err != nil {
			logs.GetLogger().Errorf("Failed get sync task result, error: %v", err)
			return
		}
		logs.GetLogger().Infof("Job: %s, service running successfully, job_result_url: %s", jobSourceURI, result.(string))
	}()

	jobData.JobResultURI = fmt.Sprintf("https://%s%s", hostPrefix, conf.GetConfig().API.Domain)
	submitJob(&jobData)

	c.JSON(http.StatusOK, jobData)
}

func submitJob(jobData *models.JobData) {
	logs.GetLogger().Printf("submitting job...")
	oldMask := syscall.Umask(0)
	defer syscall.Umask(oldMask)

	fileCachePath := conf.GetConfig().MCS.FileCachePath
	folderPath := "jobs"
	jobDetailFile := filepath.Join(folderPath, uuid.NewString()+".json")
	os.MkdirAll(filepath.Join(fileCachePath, folderPath), os.ModePerm)
	taskDetailFilePath := filepath.Join(fileCachePath, jobDetailFile)

	jobData.Status = constants.BiddingSubmitted
	jobData.UpdatedAt = strconv.FormatInt(time.Now().Unix(), 10)
	bytes, err := json.Marshal(jobData)
	if err != nil {
		logs.GetLogger().Errorf("Failed Marshal JobData, error: %v", err)
		return
	}
	if err = os.WriteFile(taskDetailFilePath, bytes, os.ModePerm); err != nil {
		logs.GetLogger().Errorf("Failed jobData write to file, error: %v", err)
		return
	}

	storageService := NewStorageService()
	mcsOssFile, err := storageService.UploadFileToBucket(jobDetailFile, taskDetailFilePath, true)
	if err != nil {
		logs.GetLogger().Errorf("Failed upload file to bucket, error: %v", err)
		return
	}
	mcsFileJson, _ := json.Marshal(mcsOssFile)
	logs.GetLogger().Printf("Job submitted to IPFS %s", string(mcsFileJson))

	gatewayUrl, err := storageService.GetGatewayUrl()
	if err != nil {
		logs.GetLogger().Errorf("Failed get mcs ipfs gatewayUrl, error: %v", err)
		return
	}
	jobData.JobResultURI = *gatewayUrl + "/ipfs/" + mcsOssFile.PayloadCid
}

func RestartJob(c *gin.Context) {
	var jobData models.JobData

	if err := c.ShouldBindJSON(&jobData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	logs.GetLogger().Infof("Job received: %+v", jobData)

	jobSourceURI := jobData.JobSourceURI
	creator, spaceName, err := getSpaceName(jobSourceURI)
	if err != nil {
		logs.GetLogger().Errorf("Failed get space name: %v", err)
		return
	}

	hostPrefix := generateString(10)
	delayTask, err := celeryService.DelayTask(constants.TASK_DEPLOY, creator, spaceName, jobSourceURI, jobData.Hardware, hostPrefix, jobData.Duration)
	if err != nil {
		logs.GetLogger().Errorf("Failed sync delpoy task, error: %v", err)
		return
	}
	logs.GetLogger().Infof("delayTask detail info: %+v", delayTask)

	go func() {
		result, err := delayTask.Get(180 * time.Second)
		if err != nil {
			logs.GetLogger().Errorf("Failed get sync task result, error: %v", err)
			return
		}
		logs.GetLogger().Infof("Job: %s, service running successfully, job_result_url: %s", jobSourceURI, result.(string))
	}()

	jobData.JobResultURI = fmt.Sprintf("https://%s%s", hostPrefix, conf.GetConfig().API.Domain)
	submitJob(&jobData)
	logs.GetLogger().Infof("update Job received: %+v", jobData)

	c.JSON(http.StatusOK, jobData)
}

func DeleteJob(c *gin.Context) {
	var deleteJobReq models.DeleteJobReq
	if err := c.ShouldBindJSON(&deleteJobReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	logs.GetLogger().Infof("Job delete req: %+v", deleteJobReq)
	if deleteJobReq.CreatorWallet == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "creator_wallet is required"})
	}
	if deleteJobReq.SpaceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "space_name is required"})
	}

	k8sNameSpace := constants.K8S_NAMESPACE_NAME_PREFIX + strings.ToLower(deleteJobReq.CreatorWallet)
	deleteJob(k8sNameSpace, deleteJobReq.SpaceName)
	c.JSON(http.StatusOK, common.CreateSuccessResponse("deleted success"))
}

func DeploySpaceTask(creator, spaceName, jobSourceURI, hardware, hostPrefix string, duration int) string {
	logs.GetLogger().Infof("Processing job: %s", jobSourceURI)
	imageName, dockerfilePath := BuildSpaceTaskImage(spaceName, jobSourceURI)
	resource, ok := common.HardwareResource[hardware]
	if !ok {
		logs.GetLogger().Warnf("not found resource.")
		return ""
	}
	creator = strings.ToLower(creator)
	spaceName = strings.ToLower(spaceName)
	hostName := hostPrefix + conf.GetConfig().API.Domain
	runContainerToK8s(hostName, creator, spaceName, imageName, dockerfilePath, resource, duration)
	return hostName
}

type DeploymentReq struct {
	NameSpace     string
	DeployName    string
	ContainerName string
	ImageName     string
	Label         map[string]string
	ContainerPort int32
	Res           common.Resource
}

func runContainerToK8s(hostName, creator, spaceName, imageName, dockerfilePath string, res common.Resource, duration int) {
	exposedPort, err := ExtractExposedPort(dockerfilePath)
	if err != nil {
		logs.GetLogger().Infof("Failed to extract exposed port: %v", err)
		return
	}
	containerPort, err := strconv.ParseInt(exposedPort, 10, 64)
	if err != nil {
		logs.GetLogger().Errorf("Failed to convert exposed port: %v", err)
		return
	}

	k8sService := NewK8sService()
	// create namespace
	k8sNameSpace := constants.K8S_NAMESPACE_NAME_PREFIX + strings.ToLower(creator)

	// first delete old resource
	deleteJob(k8sNameSpace, spaceName)

	if _, err = k8sService.GetNameSpace(context.TODO(), k8sNameSpace, metaV1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			namespace := &coreV1.Namespace{
				ObjectMeta: metaV1.ObjectMeta{
					Name: k8sNameSpace,
					Labels: map[string]string{
						"lab-ns": strings.ToLower(creator),
					},
				},
			}
			createdNamespace, err := k8sService.CreateNameSpace(context.TODO(), namespace, metaV1.CreateOptions{})
			if err != nil {
				logs.GetLogger().Errorf("Failed create namespace, error: %+v", err)
				return
			}
			logs.GetLogger().Infof("create namespace successfully, namespace: %s", createdNamespace.Name)

			//networkPolicy, err := k8sService.CreateNetworkPolicy(context.TODO(), k8sNameSpace)
			//if err != nil {
			//	logs.GetLogger().Errorf("Failed create networkPolicy, error: %+v", err)
			//	return
			//}
			//logs.GetLogger().Infof("create networkPolicy successfully, networkPolicyName: %s", networkPolicy.Name)
		} else {
			logs.GetLogger().Error(err)
			return
		}
	}

	// create deployment
	createDeployment, err := k8sService.CreateDeployment(context.TODO(), k8sNameSpace, DeploymentReq{
		NameSpace:     k8sNameSpace,
		DeployName:    constants.K8S_DEPLOY_NAME_PREFIX + spaceName,
		ContainerName: constants.K8S_CONTAINER_NAME_PREFIX + spaceName,
		ImageName:     imageName,
		Label:         map[string]string{"lad_app": spaceName},
		ContainerPort: int32(containerPort),
		Res:           res,
	})
	if err != nil {
		logs.GetLogger().Error(err)
		return
	}
	logs.GetLogger().Infof("Created deployment: %s", createDeployment.GetObjectMeta().GetName())

	// create service
	createService, err := k8sService.CreateService(context.TODO(), k8sNameSpace, spaceName, int32(containerPort))
	if err != nil {
		logs.GetLogger().Error(err)
		return
	}
	logs.GetLogger().Infof("Created service successfully: %s", createService.GetObjectMeta().GetName())

	// create ingress
	createIngress, err := k8sService.CreateIngress(context.TODO(), k8sNameSpace, spaceName, hostName, int32(containerPort))
	if err != nil {
		logs.GetLogger().Error(err)
		return
	}
	logs.GetLogger().Infof("Created Ingress successfully: %s", createIngress.GetObjectMeta().GetName())

	// watch running time and release resources when expired
	watchContainerRunningTime(string(createDeployment.GetObjectMeta().GetUID()), k8sNameSpace, spaceName, int64(duration))
	return
}

func deleteJob(namespace, spaceName string) {
	deployName := constants.K8S_DEPLOY_NAME_PREFIX + spaceName
	serviceName := constants.K8S_SERVICE_NAME_PREFIX + spaceName
	ingressName := constants.K8S_INGRESS_NAME_PREFIX + spaceName

	k8sService := NewK8sService()
	if err := k8sService.DeleteIngress(context.TODO(), namespace, ingressName); err != nil && !errors.IsNotFound(err) {
		logs.GetLogger().Errorf("Failed delete ingress, ingressName: %s, error: %+v", deployName, err)
		return
	}
	logs.GetLogger().Infof("Deleted ingress %s finished", ingressName)

	if err := k8sService.DeleteService(context.TODO(), namespace, serviceName); err != nil && !errors.IsNotFound(err) {
		logs.GetLogger().Errorf("Failed delete service, serviceName: %s, error: %+v", serviceName, err)
		return
	}
	logs.GetLogger().Infof("Deleted service %s finished", serviceName)

	dockerService := NewDockerService()
	deployImageIds, err := k8sService.GetDeploymentImages(context.TODO(), namespace, deployName)
	if err != nil && !errors.IsNotFound(err) {
		logs.GetLogger().Errorf("Failed get deploy imageIds, deployName: %s, error: %+v", deployName, err)
		return
	}
	for _, imageId := range deployImageIds {
		err = dockerService.RemoveImage(imageId)
		if err != nil {
			logs.GetLogger().Errorf("Failed delete unused image, imageId: %s, error: %+v", imageId, err)
			continue
		}
	}

	if err := k8sService.DeleteDeployment(context.TODO(), namespace, deployName); err != nil && !errors.IsNotFound(err) {
		logs.GetLogger().Errorf("Failed delete deployment, deployName: %s, error: %+v", deployName, err)
		return
	}
	time.Sleep(6 * time.Second)
	logs.GetLogger().Infof("Deleted deployment %s finished", deployName)

	if err := k8sService.DeleteDeployRs(context.TODO(), namespace, spaceName); err != nil && !errors.IsNotFound(err) {
		logs.GetLogger().Errorf("Failed delete eplicationController, spaceName: %s, error: %+v", spaceName, err)
		return
	}

	if err := k8sService.DeletePod(context.TODO(), namespace, spaceName); err != nil && !errors.IsNotFound(err) {
		logs.GetLogger().Errorf("Failed delete pods, spaceName: %s, error: %+v", spaceName, err)
		return
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	var count = 0
	for {
		<-ticker.C
		count++
		if count >= 20 {
			break
		}
		getPods, err := k8sService.GetPods(namespace, spaceName)
		if err != nil && !errors.IsNotFound(err) {
			logs.GetLogger().Errorf("Failed get pods form namespace, namepace: %s, error: %+v", namespace, err)
			continue
		}
		if !getPods {
			logs.GetLogger().Infof("Deleted all resource finised. spaceName %s", spaceName)
			break
		}
	}
}

func watchContainerRunningTime(key, namespace, spaceName string, runTime int64) {
	conn := redisPool.Get()
	_, err := conn.Do("SET", key, "wait-delete", "EX", runTime)
	if err != nil {
		logs.GetLogger().Errorf("Failed set redis key and expire time, key: %s, error: %+v", key, err)
		return
	}

	fullArgs := []interface{}{constants.REDIS_FULL_PREFIX + key}
	fields := map[string]string{
		"k8s_namespace": namespace,
		"space_name":    spaceName,
		"expire_time":   strconv.Itoa(int(time.Now().Unix() + runTime)),
	}
	for key, val := range fields {
		fullArgs = append(fullArgs, key, val)
	}
	conn.Do("HSET", fullArgs...)

	go func() {
		psc := redis.PubSubConn{Conn: redisPool.Get()}
		psc.PSubscribe("__keyevent@0__:expired")
		for {
			switch n := psc.Receive().(type) {
			case redis.Message:
				if n.Channel == "__keyevent@0__:expired" && string(n.Data) == key {
					logs.GetLogger().Infof("The namespace: %s, spacename: %s, job has reached its runtime and will stop running.", namespace, spaceName)
					deleteJob(namespace, spaceName)
					redisPool.Get().Do("DEL", constants.REDIS_FULL_PREFIX+key)
				}
			case redis.Subscription:
				logs.GetLogger().Infof("Subscribe %s", n.Channel)
			case error:
				return
			}
		}
	}()
}

func WatchExpiredTask() {
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
			cursor := "0"
			prefix := constants.REDIS_FULL_PREFIX + "*"
			values, err := redis.Values(conn.Do("SCAN", cursor, "MATCH", prefix))
			if err != nil {
				logs.GetLogger().Errorf("Failed scan redis %s prefix, error: %+v", prefix, err)
				return
			}

			cursor, _ = redis.String(values[0], nil)
			keys, _ := redis.Strings(values[1], nil)
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
						logs.GetLogger().Infof("The namespace: %s, spacename: %s, job has reached its runtime and will stop running.", namespace, spaceName)
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

			if cursor == "0" {
				break
			}
		}
	}()
}

func WatchNameSpaceForDeleted() {
	ticker := time.NewTicker(24 * time.Hour)
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
		}
	}()
}

func generateString(length int) string {
	characters := "abcdefghijklmnopqrstuvwxyz"
	numbers := "0123456789"
	source := characters + numbers
	result := make([]byte, length)
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < length; i++ {
		result[i] = source[rand.Intn(len(source))]
	}
	return string(result)
}
