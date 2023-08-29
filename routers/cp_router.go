package routers

import (
	"github.com/gin-gonic/gin"
	"github.com/lagrangedao/go-computing-provider/computing"
)

func CPManager(router *gin.RouterGroup) {

	router.GET("/host/info", computing.GetServiceProviderInfo)
	router.POST("/lagrange/jobs", computing.ReceiveJob)
	router.POST("/lagrange/jobs/redeploy", computing.RedeployJob)
	router.DELETE("/lagrange/jobs", computing.DeleteJob)
	router.GET("/lagrange/cp", computing.StatisticalSources)
	router.POST("/lagrange/jobs/renew", computing.ReNewJob)
	router.GET("/lagrange/jobs/logs", computing.GetSpaceLog)
}
