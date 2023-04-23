package initializer

import (
	"github.com/filswan/go-swan-lib/logs"
	"github.com/joho/godotenv"
	"go-computing-provider/computing"
	"os"
)

func LoadEnv() {
	err := godotenv.Load(".env")
	if err != nil {
		logs.GetLogger().Error(err)
	}

	logs.GetLogger().Info("name: ", os.Getenv("MCS_BUCKET"))
}

func ProjectInit() {
	LoadEnv()
	computing.InitComputingProvider()
}
