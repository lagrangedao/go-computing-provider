package routers

import (
	"github.com/gin-gonic/gin"
	"github.com/lagrangedao/go-computing-provider/computing"
)

func CPManager(router *gin.RouterGroup) {

	router.GET("/host/info", computing.GetServiceProviderInfo)
	router.POST("/lagrange/jobs", computing.ReceiveJob)
	router.DELETE("/lagrange/space/:space_name", computing.DeleteSpaceTask)

}
