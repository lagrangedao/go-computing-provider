package computing

import (
	"github.com/filswan/go-mcs-sdk/mcs/api/bucket"
	"github.com/filswan/go-mcs-sdk/mcs/api/common/logs"
	"github.com/filswan/go-mcs-sdk/mcs/api/user"
	"os"
	"strings"
	"sync"
)

var storage *StorageService
var once sync.Once

type StorageService struct {
	McsApiKey      string `json:"mcs_api_key"`
	McsAccessToken string `json:"mcs_access_token"`
	NetWork        string `json:"net_work"`
	BucketName     string `json:"bucket_name"`
}

func NewStorageService() *StorageService {
	once.Do(func() {
		storage = &StorageService{
			McsApiKey:      os.Getenv("MCS_API_KEY"),
			McsAccessToken: os.Getenv("MCS_ACCESS_TOKEN"),
			NetWork:        os.Getenv("MSC_NETWORK"),
			BucketName:     os.Getenv("MCS_BUCKET"),
		}
	})
	return storage
}

func (storage *StorageService) UploadFileToBucket(objectName, filePath string, replace bool) (*bucket.OssFile, error) {
	logs.GetLogger().Infof("uploading file to bucket, objectName: %s, filePath: %s", objectName, filePath)
	mcsClient, err := user.LoginByApikey(storage.McsApiKey, storage.McsAccessToken, storage.NetWork)
	if err != nil {
		logs.GetLogger().Errorf("Failed creating mcsClient, error: %v", err)
		return nil, err
	}
	buketClient := bucket.GetBucketClient(*mcsClient)

	//if _, err := buketClient.CreateFolder(storage.BucketName, filepath.Dir(objectName), ""); err != nil {
	//	logs.GetLogger().Errorf("Failed create folder, error: %v", err)
	//	return nil, err
	//}

	file, err := buketClient.GetFile(storage.BucketName, objectName)
	if err != nil && !strings.Contains(err.Error(), "record not found") {
		logs.GetLogger().Errorf("Failed get file form bucket, error: %v", err)
		return nil, err
	}

	if file != nil {
		if err = buketClient.DeleteFile(storage.BucketName, objectName); err != nil {
			logs.GetLogger().Errorf("Failed delete file form bucket, error: %v", err)
			return nil, err
		}
	}

	if err := buketClient.UploadFile(storage.BucketName, objectName, filePath, replace); err != nil {
		logs.GetLogger().Errorf("Failed upload file to bucket, error: %v", err)
		return nil, err
	}

	mcsOssFile, err := buketClient.GetFile(storage.BucketName, objectName)
	if err != nil {
		logs.GetLogger().Errorf("Failed get file form bucket, error: %v", err)
		return nil, err
	}
	return mcsOssFile, nil
}
