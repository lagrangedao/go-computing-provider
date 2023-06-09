package conf

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

var config *ComputeNode

// ComputeNode is a compute node config
type ComputeNode struct {
	API      API
	LAD      LAD
	MCS      MCS
	Registry Registry
}

type API struct {
	Port          int
	MultiAddress  string
	RedisUrl      string
	RedisPassword string
	Domain        string
}

type LAD struct {
	ServerUrl   string
	AccessToken string
}

type MCS struct {
	ApiKey        string
	AccessToken   string
	BucketName    string
	Network       string
	FileCachePath string
}

type Registry struct {
	ServerAddress string
	UserName      string
	Password      string
}

func InitConfig() error {
	currentDir, _ := os.Getwd()
	configFile := filepath.Join(currentDir, "config.toml")

	_, err := toml.DecodeFile(configFile, &config)
	if err != nil {
		return fmt.Errorf("Failed load config file, path: %s, error: %w", configFile, err)
	}
	return nil
}

func GetConfig() *ComputeNode {
	return config
}
