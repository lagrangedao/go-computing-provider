package computing

import (
	"context"
	"encoding/json"
	"github.com/filswan/go-mcs-sdk/mcs/api/common/logs"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lagrangedao/go-computing-provider/common"
	"github.com/lagrangedao/go-computing-provider/constants"
	"github.com/lagrangedao/go-computing-provider/models"
	coreV1 "k8s.io/api/core/v1"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"
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

	// TODO: Async Processing the job
	jobSourceURI := jobData.JobSourceURI
	spaceName, err := getSpaceName(jobSourceURI)
	if err != nil {
		logs.GetLogger().Errorf("Failed get space name: %v", err)
		return
	}

	delayTask, err := celeryService.DelayTask(constants.TASK_DEPLOY, spaceName, jobSourceURI)
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

	c.JSON(http.StatusOK, "job submitted")
}

func submitJob(jobData *models.JobData) {
	logs.GetLogger().Printf("submitting job...")
	oldMask := syscall.Umask(0)
	defer syscall.Umask(oldMask)

	fileCachePath := os.Getenv("FILE_CACHE_PATH")
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

func DeploySpaceTask(spaceName, jobSourceURI string) string {
	logs.GetLogger().Infof("Processing job: %s", jobSourceURI)
	imageName, dockerfilePath := BuildSpaceTaskImage(spaceName, jobSourceURI)
	resultUrl := runContainerToK8s(imageName, dockerfilePath, spaceName)
	logs.GetLogger().Infof("Job: %s, running at: %s", jobSourceURI, resultUrl)
	return resultUrl
}

func DeleteSpaceTask(c *gin.Context) {
	spaceName := c.Param("space_name")
	k8sService := NewK8sService()
	if err := k8sService.DeleteService(context.TODO(), coreV1.NamespaceDefault, spaceName); err != nil {
		logs.GetLogger().Error(err)
		return
	}

	if err := k8sService.DeleteDeployment(context.TODO(), coreV1.NamespaceDefault, "cp-worker-"+spaceName); err != nil {
		logs.GetLogger().Error(err)
		return
	}
}
