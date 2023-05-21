package computing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lagrangedao/go-computing-provider/conf"

	"github.com/circonus-labs/circonus-gometrics/api/config"
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
	"k8s.io/apimachinery/pkg/util/intstr"
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
	logs.GetLogger().Infof("Job received: %+v", jobData)

	jobSourceURI := jobData.JobSourceURI
	creator, spaceName, err := getSpaceName(jobSourceURI)
	if err != nil {
		logs.GetLogger().Errorf("Failed get space name: %v", err)
		return
	}

	delayTask, err := celeryService.DelayTask(constants.TASK_DEPLOY, creator, spaceName, jobSourceURI, jobData.Hardware, jobData.Duration)
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
		url := result.(string)
		jobData.JobResultURI = url
		submitJob(&jobData)
		logs.GetLogger().Infof("update Job received: %+v", jobData)
	}()

	c.JSON(http.StatusOK, jobData)
}

func submitJob(jobData *models.JobData) {
	logs.GetLogger().Printf("submitting job...")
	oldMask := syscall.Umask(0)
	defer syscall.Umask(oldMask)

	fileCachePath := conf.GetConfig().Mcs.FileCachePath
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

func DeploySpaceTask(creator, spaceName, jobSourceURI, hardware string, duration int) string {
	logs.GetLogger().Infof("Processing job: %s", jobSourceURI)
	imageName, dockerfilePath := BuildSpaceTaskImage(spaceName, jobSourceURI)

	resource, ok := common.HardwareResource[hardware]
	if !ok {
		logs.GetLogger().Warnf("not found resource.")
		return ""
	}

	creator = strings.ToLower(creator)
	spaceName = strings.ToLower(spaceName)
	resultUrl := runContainerToK8s(creator, spaceName, imageName, dockerfilePath, resource, duration)
	logs.GetLogger().Infof("Job: %s, running at: %s", jobSourceURI, resultUrl)
	return resultUrl
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

func runContainerToK8s(creator, spaceName, imageName, dockerfilePath string, res common.Resource, duration int) string {
	exposedPort, err := ExtractExposedPort(dockerfilePath)
	if err != nil {
		logs.GetLogger().Infof("Failed to extract exposed port: %v", err)
		return ""
	}

	containerPort, err := strconv.ParseInt(exposedPort, 10, 64)
	if err != nil {
		logs.GetLogger().Errorf("Failed to convert exposed port: %v", err)
		return ""
	}

	k8sService := NewK8sService()

	nameSpace := constants.K8S_NAMESPACE_NAME_PREFIX + strings.ToLower(creator)
	if _, err = k8sService.GetNameSpace(context.TODO(), nameSpace, metaV1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			namespace := &coreV1.Namespace{
				ObjectMeta: metaV1.ObjectMeta{
					Name: nameSpace,
				},
			}
			createdNamespace, err := k8sService.CreateNameSpace(context.TODO(), namespace, metaV1.CreateOptions{})
			if err != nil {
				logs.GetLogger().Errorf("Failed create namespace, error: %+v", err)
				return ""
			}
			logs.GetLogger().Infof("create namespace successfully, namespace: %s", createdNamespace.Name)
		} else {
			logs.GetLogger().Error(err)
			return ""
		}
	}

	createDeployment, err := k8sService.CreateDeployment(context.TODO(), nameSpace, DeploymentReq{
		NameSpace:     nameSpace,
		DeployName:    constants.K8S_DEPLOY_NAME_PREFIX + spaceName,
		ContainerName: constants.K8S_CONTAINER_NAME_PREFIX + spaceName,
		ImageName:     imageName,
		Label:         map[string]string{"app": spaceName},
		ContainerPort: int32(containerPort),
		Res:           res,
	})
	if err != nil {
		logs.GetLogger().Error(err)
		return ""
	}
	logs.GetLogger().Infof("Created deployment: %s", createDeployment.GetObjectMeta().GetName())
	watchContainerRunningTime(string(createDeployment.GetObjectMeta().GetUID()), nameSpace, spaceName, duration)

	service := &coreV1.Service{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      constants.K8S_SERVICE_NAME_PREFIX + spaceName,
			Namespace: nameSpace,
		},
		Spec: coreV1.ServiceSpec{
			Type: coreV1.ServiceTypeNodePort,
			Ports: []coreV1.ServicePort{
				{
					Name:       "http",
					Port:       int32(containerPort),
					TargetPort: intstr.FromInt(int(containerPort)),
					Protocol:   coreV1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": spaceName,
			},
		},
	}
	createService, err := k8sService.CreateService(context.TODO(), nameSpace, service, metaV1.CreateOptions{})
	if err != nil {
		logs.GetLogger().Error(err)
		return ""
	}
	logs.GetLogger().Infof("Created service %s", createService.GetObjectMeta().GetName())

	service, err = k8sService.GetServiceByName(context.TODO(), nameSpace, constants.K8S_SERVICE_NAME_PREFIX+spaceName, metaV1.GetOptions{})
	if err != nil {
		logs.GetLogger().Error(err)
		return ""
	}
	port := service.Spec.Ports[0].NodePort
	fmt.Printf("Service is exposed at %s:%d\n", config.Host, port)

	//hostIp, err := k8sService.GetNodeList()
	//if err != nil {
	//	logs.GetLogger().Error(err)
	//	return ""
	//}
	url := "http://127.0.0.1" + ":" + strconv.Itoa(int(port))

	return url
}

func DeleteSpaceTask(c *gin.Context) {
	spaceName := c.Param("space_name")
	spaceName = strings.ToLower(spaceName)
	var creatorWallet struct {
		UserAddr string `json:"user_addr"`
	}
	err := c.ShouldBindJSON(&creatorWallet)
	if err != nil {
		c.JSON(http.StatusBadRequest, common.CreateErrorResponse("400", "required user address"))
	}

	creator := constants.K8S_NAMESPACE_NAME_PREFIX + strings.ToLower(creatorWallet.UserAddr)
	deleteJob(creator, spaceName)
}

func deleteJob(namespace, spaceName string) {
	k8sService := NewK8sService()
	serviceName := constants.K8S_SERVICE_NAME_PREFIX + spaceName
	if err := k8sService.DeleteService(context.TODO(), namespace, serviceName); err != nil {
		logs.GetLogger().Error(err)
		return
	}
	logs.GetLogger().Infof("Deleted service %s finished", serviceName)

	deployName := constants.K8S_DEPLOY_NAME_PREFIX + spaceName
	if err := k8sService.DeleteDeployment(context.TODO(), namespace, deployName); err != nil {
		logs.GetLogger().Error(err)
		return
	}
	logs.GetLogger().Infof("Deleted service %s finished", deployName)
}

func watchContainerRunningTime(key, namespace, spaceName string, time int) {
	fields := map[string]string{
		"k8s_namespace": namespace,
		"space_name":    spaceName,
	}
	args := []interface{}{key}
	for key, val := range fields {
		args = append(args, key, val)
	}
	conn := redisPool.Get()

	_, err := conn.Do("HMSET", args...)
	if err != nil {
		logs.GetLogger().Errorf("Failed save redis key, key: %s, error: %+v", key, err)
		return
	}

	_, err = conn.Do("EXPIRE", key, time)
	if err != nil {
		logs.GetLogger().Errorf("Failed expire redis key, key: %s, error: %+v", key, err)
		return
	}

	go func() {
		psc := redis.PubSubConn{Conn: redisPool.Get()}
		psc.PSubscribe("__keyevent@0__:expired")
		for {
			switch n := psc.Receive().(type) {
			case redis.Message:
				if n.Channel == "__keyevent@0__:expired" && string(n.Data) == key {
					logs.GetLogger().Infof("The namespace: %s, spacename: %s, job has reached its runtime and will stop running.", namespace, spaceName)
					deleteJob(namespace, spaceName)
				}
			case redis.Subscription:
				logs.GetLogger().Infof("Subscribe %s", n.Channel)
			case error:
				return
			}
		}
	}()
}
