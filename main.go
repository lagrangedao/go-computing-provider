package main

import (
	"github.com/filswan/go-swan-lib/logs"
	"github.com/gin-gonic/gin"
	cors "github.com/itsjamie/gin-cors"
	"github.com/joho/godotenv"
	"go-computing-provider/initializer"
	"go-computing-provider/routers"
	"os"
	"strconv"
	"time"
)

func LoadEnv() {
	err := godotenv.Load(".env")
	if err != nil {
		logs.GetLogger().Error(err)
	}

	logs.GetLogger().Info("name: ", os.Getenv("privateKey"))
}

func main() {
	logs.GetLogger().Info("Start in computing provider mode.")
	initializer.ProjectInit()

	r := gin.Default()
	r.Use(cors.Middleware(cors.Config{
		Origins:         "*",
		Methods:         "GET, PUT, POST, DELETE",
		RequestHeaders:  "Origin, Authorization, Content-Type",
		ExposedHeaders:  "",
		MaxAge:          50 * time.Second,
		ValidateHeaders: false,
	}))

	v1 := r.Group("/api/v1")
	routers.CPManager(v1.Group("/computing"))
	err := r.Run(":" + strconv.Itoa(8085))
	if err != nil {
		logs.GetLogger().Fatal(err)
	}
}
