package routers

import (
	"encoding/json"
	"github.com/filswan/go-swan-lib/logs"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go-computing-provider/common"
	"go-computing-provider/computing"
	"go-computing-provider/constants"
	"go-computing-provider/models"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func GetServiceProviderInfo(c *gin.Context) {
	info := computing.GetServiceProviderInfo()
	c.JSON(http.StatusOK, common.CreateSuccessResponse(info))
}

func receiveJob(c *gin.Context) {
	var jobData models.JobData

	if err := c.ShouldBindJSON(&jobData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Job received: %+v\n", jobData)

	// TODO: Async Processing the job
	result := processJob(jobData)

	c.JSON(http.StatusOK, result)
}

func processJob(jobData models.JobData) interface{} {
	// Add your job processing logic here
	log.Printf("Processing job: %+v\n", jobData)
	jobSourceURI := jobData.JobSourceURI
	imageName, dockerfilePath := computing.BuildSpaceTask(jobSourceURI)
	url := computing.RunContainer(imageName, dockerfilePath)
	submitJob(&jobData)

	log.Printf("Running at: %s", url)

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

	mcsOssFile, err := computing.NewStorageService().UploadFileToBucket(jobDetailFile, taskDetailFilePath, false)
	if err != nil {

	}
	mcsFileJson, _ := json.Marshal(mcsOssFile)
	logs.GetLogger().Printf("Job submitted to IPFS %s", string(mcsFileJson))
	jobData.JobResultURI = "" + "/ipfs/" + mcsOssFile.PayloadCid
}

func CPManager(router *gin.RouterGroup) {

	router.GET("/host/info", GetServiceProviderInfo)
	router.POST("/lagrange/jobs", receiveJob)
}
