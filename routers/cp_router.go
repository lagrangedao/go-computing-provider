package routers

import (
	"github.com/filswan/go-swan-lib/logs"
	"github.com/gin-gonic/gin"
	"go-computing-provider/common"
	"go-computing-provider/computing"
	"go-computing-provider/models"
	"log"
	"net/http"
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
	logs.GetLogger().Info("Processing job: %+v\n", jobData)
	return nil
}
func CPManager(router *gin.RouterGroup) {

	router.GET("/host/info", GetServiceProviderInfo)
	router.POST("/lagrange/jobs", receiveJob)
}
