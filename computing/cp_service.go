package computing

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-computing-provider/common"
	"go-computing-provider/constants"
	"go-computing-provider/models"
	"go-mcs-sdk/mcs/api/common/logs"
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
	result := processJob(jobData)

	c.JSON(http.StatusOK, result)
}

func processJob(jobData models.JobData) interface{} {
	// Add your job processing logic here
	logs.GetLogger().Infof("Processing job: %s", jobData.JobSourceURI)
	jobSourceURI := jobData.JobSourceURI
	imageName, dockerfilePath := BuildSpaceTask(jobSourceURI)
	url := RunContainer(imageName, dockerfilePath)

	//spaceName, err := getSpaceName(jobSourceURI)
	//if err != nil {
	//	logs.GetLogger().Errorf("Error get space name: %v", err)
	//	return ""
	//}
	//url := runContainerToK8s(imageName, dockerfilePath, spaceName)

	submitJob(&jobData)

	logs.GetLogger().Infof("Running at: %s", url)

	return nil
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

	mcsOssFile, err := NewStorageService().UploadFileToBucket(jobDetailFile, taskDetailFilePath, false)
	if err != nil {
		logs.GetLogger().Errorf("Failed upload file to bucket, error: %v", err)
		return
	}
	mcsFileJson, _ := json.Marshal(mcsOssFile)
	logs.GetLogger().Printf("Job submitted to IPFS %s", string(mcsFileJson))
	jobData.JobResultURI = "" + "/ipfs/" + mcsOssFile.PayloadCid
}
