package routers

import (
	"github.com/gin-gonic/gin"
	"go-computing-provider/common"
	"go-computing-provider/computing"
	"net/http"
)

func GetServiceProviderInfo(c *gin.Context) {
	info := computing.GetServiceProviderInfo()
	c.JSON(http.StatusOK, common.CreateSuccessResponse(info))
}

func CPManager(router *gin.RouterGroup) {

	router.GET("/host/info", GetServiceProviderInfo)
}
