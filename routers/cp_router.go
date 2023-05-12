package routers

import (
	"github.com/gin-gonic/gin"
	"go-computing-provider/computing"
)

func CPManager(router *gin.RouterGroup) {

	router.GET("/host/info", computing.GetServiceProviderInfo)
	router.POST("/lagrange/jobs", computing.ReceiveJob)
	router.DELETE("/lagrange/space/:task_name", computing.DeleteSpaceTask)

}
