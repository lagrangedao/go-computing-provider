package computing

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/filswan/go-mcs-sdk/mcs/api/common/logs"
	"github.com/gin-gonic/gin"
	"github.com/gomodule/redigo/redis"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/lagrangedao/go-computing-provider/common"
	"github.com/lagrangedao/go-computing-provider/conf"
	"github.com/lagrangedao/go-computing-provider/constants"
	"github.com/lagrangedao/go-computing-provider/docker"
	"github.com/lagrangedao/go-computing-provider/models"
	"io"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func GetServiceProviderInfo(c *gin.Context) {
	info := new(models.HostInfo)
	//info.SwanProviderVersion = common.GetVersion()
	info.OperatingSystem = runtime.GOOS
	info.Architecture = runtime.GOARCH
	info.CPUCores = runtime.NumCPU()
	c.JSON(http.StatusOK, common.CreateSuccessResponse(info))
}

func ReceiveJob(c *gin.Context) {
	var jobData models.JobData
	if err := c.ShouldBindJSON(&jobData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	logs.GetLogger().Infof("Job received Data: %+v", jobData)

	var hostName string
	prefixStr := generateString(10)
	if strings.HasPrefix(conf.GetConfig().API.Domain, ".") {
		hostName = prefixStr + conf.GetConfig().API.Domain
	} else {
		hostName = strings.Join([]string{prefixStr, conf.GetConfig().API.Domain}, ".")
	}

	delayTask, err := celeryService.DelayTask(constants.TASK_DEPLOY, jobData.JobSourceURI, hostName, jobData.Duration, jobData.UUID)
	if err != nil {
		logs.GetLogger().Errorf("Failed sync delpoy task, error: %v", err)
		return
	}
	go func() {
		result, err := delayTask.Get(180 * time.Second)
		if err != nil {
			logs.GetLogger().Errorf("Failed get sync task result, error: %v", err)
			return
		}
		logs.GetLogger().Infof("Job_uuid: %s, service running successfully, job_result_url: %s", jobData.UUID, result.(string))
	}()

	jobData.JobResultURI = fmt.Sprintf("https://%s", hostName)
	if err = submitJob(&jobData); err != nil {
		jobData.JobResultURI = ""
	} else {
		updateJobStatus(jobData.UUID, models.JobUploadResult)
	}
	c.JSON(http.StatusOK, jobData)
}

func submitJob(jobData *models.JobData) error {
	logs.GetLogger().Printf("submitting job...")
	oldMask := syscall.Umask(0)
	defer syscall.Umask(oldMask)

	fileCachePath := conf.GetConfig().MCS.FileCachePath
	folderPath := "jobs"
	jobDetailFile := filepath.Join(folderPath, uuid.NewString()+".json")
	os.MkdirAll(filepath.Join(fileCachePath, folderPath), os.ModePerm)
	taskDetailFilePath := filepath.Join(fileCachePath, jobDetailFile)

	jobData.Status = constants.BiddingSubmitted
	jobData.UpdatedAt = strconv.FormatInt(time.Now().Unix(), 10)
	bytes, err := json.Marshal(jobData)
	if err != nil {
		logs.GetLogger().Errorf("Failed Marshal JobData, error: %v", err)
		return err
	}
	if err = os.WriteFile(taskDetailFilePath, bytes, os.ModePerm); err != nil {
		logs.GetLogger().Errorf("Failed jobData write to file, error: %v", err)
		return err
	}

	storageService := NewStorageService()
	mcsOssFile, err := storageService.UploadFileToBucket(jobDetailFile, taskDetailFilePath, true)
	if err != nil {
		logs.GetLogger().Errorf("Failed upload file to bucket, error: %v", err)
		return err
	}
	mcsFileJson, _ := json.Marshal(mcsOssFile)
	logs.GetLogger().Printf("Job submitted to IPFS %s", string(mcsFileJson))

	gatewayUrl, err := storageService.GetGatewayUrl()
	if err != nil {
		logs.GetLogger().Errorf("Failed get mcs ipfs gatewayUrl, error: %v", err)
		return err
	}
	jobData.JobResultURI = *gatewayUrl + "/ipfs/" + mcsOssFile.PayloadCid
	return nil
}

func RedeployJob(c *gin.Context) {
	var jobData models.JobData

	if err := c.ShouldBindJSON(&jobData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	logs.GetLogger().Infof("redeploy Job received: %+v", jobData)

	var hostName string
	if jobData.JobResultURI != "" {
		resp, err := http.Get(jobData.JobResultURI)
		if err != nil {
			logs.GetLogger().Errorf("error making request to Space API: %+v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				logs.GetLogger().Errorf("error closed resp Space API: %+v", err)
			}
		}(resp.Body)
		logs.GetLogger().Infof("Space API response received. Response: %d", resp.StatusCode)
		if resp.StatusCode != http.StatusOK {
			logs.GetLogger().Errorf("space API response not OK. Status Code: %d", resp.StatusCode)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}

		var hostInfo struct {
			JobResultUri string `json:"job_result_uri"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&hostInfo); err != nil {
			logs.GetLogger().Errorf("error decoding Space API response JSON: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		hostName = strings.ReplaceAll(hostInfo.JobResultUri, "https://", "")
	} else {
		hostName = generateString(10) + conf.GetConfig().API.Domain
	}

	delayTask, err := celeryService.DelayTask(constants.TASK_DEPLOY, jobData.JobResultURI, hostName, jobData.Duration, jobData.UUID)
	if err != nil {
		logs.GetLogger().Errorf("Failed sync delpoy task, error: %v", err)
		return
	}
	logs.GetLogger().Infof("delayTask detail info: %+v", delayTask)

	go func() {
		result, err := delayTask.Get(180 * time.Second)
		if err != nil {
			logs.GetLogger().Errorf("Failed get sync task result, error: %v", err)
			return
		}
		logs.GetLogger().Infof("Job: %s, service running successfully, job_result_url: %s", jobData.JobResultURI, result.(string))
	}()

	jobData.JobResultURI = fmt.Sprintf("https://%s", hostName)
	if err = submitJob(&jobData); err != nil {
		jobData.JobResultURI = ""
	} else {
		updateJobStatus(jobData.UUID, models.JobUploadResult)
	}
	c.JSON(http.StatusOK, jobData)
}

func ReNewJob(c *gin.Context) {
	var jobData struct {
		SpaceUuid string `json:"space_uuid"`
		Duration  int    `json:"duration"`
	}

	if err := c.ShouldBindJSON(&jobData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	logs.GetLogger().Infof("renew Job received: %+v", jobData)

	if strings.TrimSpace(jobData.SpaceUuid) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required field: space_uuid"})
	}

	if jobData.Duration == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required field: duration"})
	}

	redisKey := constants.REDIS_FULL_PREFIX + jobData.SpaceUuid
	spaceDetail, err := retrieveJobMetadata(redisKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "not found data"})
	}

	leftTime := spaceDetail.ExpireTime - time.Now().Unix()
	if leftTime < 0 {
		c.JSON(http.StatusOK, map[string]string{
			"status":  "failed",
			"message": "The job was terminated due to its expiration date",
		})
	} else {
		fullArgs := []interface{}{redisKey}
		fields := map[string]string{
			"wallet_address": spaceDetail.WalletAddress,
			"space_name":     spaceDetail.SpaceName,
			"expire_time":    strconv.Itoa(int(time.Now().Unix()) + int(leftTime) + jobData.Duration),
			"space_uuid":     spaceDetail.SpaceUuid,
		}
		for key, val := range fields {
			fullArgs = append(fullArgs, key, val)
		}
		redisConn := redisPool.Get()
		defer redisConn.Close()

		redisConn.Do("HSET", fullArgs...)
		redisConn.Do("SET", jobData.SpaceUuid, "wait-delete", "EX", int(leftTime)+jobData.Duration)
	}
	c.JSON(http.StatusOK, common.CreateSuccessResponse("success"))
}

func DeleteJob(c *gin.Context) {
	creatorWallet := c.Query("creator_wallet")
	spaceUuid := c.Query("space_uuid")

	if creatorWallet == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "creator_wallet is required"})
	}
	if spaceUuid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "space_uuid is required"})
	}

	k8sNameSpace := constants.K8S_NAMESPACE_NAME_PREFIX + strings.ToLower(creatorWallet)

	redisKey := constants.REDIS_FULL_PREFIX + spaceUuid
	spaceDetail, err := retrieveJobMetadata(redisKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "not found data"})
	}
	deleteJob(creatorWallet, k8sNameSpace, spaceUuid, spaceDetail.SpaceName)

	c.JSON(http.StatusOK, common.CreateSuccessResponse("deleted success"))
}

func StatisticalSources(c *gin.Context) {
	location, err := getLocation()
	if err != nil {
		logs.GetLogger().Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed get location info"})
	}

	k8sService := NewK8sService()
	statisticalSources, err := k8sService.StatisticalSources(context.TODO())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}

	nodeID, _, _ := generateNodeID()
	c.JSON(http.StatusOK, models.ClusterResource{
		NodeId:      nodeID,
		Region:      location,
		ClusterInfo: statisticalSources,
	})
}

func GetSpaceLog(c *gin.Context) {
	var wsUpgrade = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := wsUpgrade.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logs.GetLogger().Errorf("Failed to set websocket upgrade: %+v", err)
		return
	}

	spaceUuid := c.Query("space_id")
	logType := c.Query("type")
	if strings.TrimSpace(spaceUuid) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required field: space_id"})
	}

	if strings.TrimSpace(logType) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required field: type"})
	}

	if strings.TrimSpace(logType) != "build" && strings.TrimSpace(logType) != "container" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing required field: type"})
	}

	redisKey := constants.REDIS_FULL_PREFIX + spaceUuid
	spaceDetail, err := retrieveJobMetadata(redisKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "not found data"})
	}

	handleConnection(conn, spaceDetail, logType)

}

func handleConnection(conn *websocket.Conn, spaceDetail models.CacheSpaceDetail, logType string) {
	defer conn.Close()

	if logType == "build" {
		buildLogPath := filepath.Join("build", spaceDetail.WalletAddress, "spaces", spaceDetail.SpaceName, docker.BuildFileName)
		sendBuildLogs(conn, buildLogPath)
	} else if logType == "container" {
		k8sNameSpace := constants.K8S_NAMESPACE_NAME_PREFIX + strings.ToLower(spaceDetail.WalletAddress)

		k8sService := NewK8sService()
		pods, err := k8sService.k8sClient.CoreV1().Pods(k8sNameSpace).List(context.TODO(), metaV1.ListOptions{
			LabelSelector: fmt.Sprintf("lad_app=%s", spaceDetail.SpaceUuid),
		})
		if err != nil {
			logs.GetLogger().Errorf("Error listing Pods: %v", err)
			return
		}

		if len(pods.Items) > 0 {
			req := k8sService.k8sClient.CoreV1().Pods(k8sNameSpace).GetLogs(pods.Items[0].Name, &v1.PodLogOptions{
				Container:  "",
				Follow:     true,
				Timestamps: true,
			})

			podLogs, err := req.Stream(context.Background())
			if err != nil {
				logs.GetLogger().Errorf("Error opening log stream: %v", err)
				return
			}
			defer podLogs.Close()

			reader := bufio.NewReader(podLogs)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						break
					}
					logs.GetLogger().Errorf("Error reading log: %v", err)
					return
				}

				if err = conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
					logs.GetLogger().Errorf("Error sending log to WebSocket: %v", err)
					return
				}

				if _, _, err = conn.ReadMessage(); err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
						logs.GetLogger().Warn("Client closed the connection")
					} else {
						logs.GetLogger().Errorf("WebSocket error: %v", err)
					}
					return
				}
			}
		}
	}
}

func DeploySpaceTask(jobSourceURI, hostName string, duration int, jobUuid string) string {
	defer func() {
		if err := recover(); err != nil {
			logs.GetLogger().Errorf("deploy space task painc, error: %+v", err)
			return
		}
	}()
	var gpuName string
	defer func() {
		if gpuName != "" {
			count, ok := runTaskGpuResource.Load(gpuName)
			if ok && count.(int) > 0 {
				runTaskGpuResource.Store(gpuName, count.(int)-1)
			} else {
				runTaskGpuResource.Delete(gpuName)
			}
		}
	}()

	resp, err := http.Get(jobSourceURI)
	if err != nil {
		logs.GetLogger().Errorf("error making request to Space API: %+v", err)
		return ""
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logs.GetLogger().Errorf("error closed resp Space API: %+v", err)
		}
	}(resp.Body)
	logs.GetLogger().Infof("Space API response received. Response: %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		logs.GetLogger().Errorf("space API response not OK. Status Code: %d", resp.StatusCode)
		return ""
	}

	var spaceJson models.SpaceJSON
	if err := json.NewDecoder(resp.Body).Decode(&spaceJson); err != nil {
		logs.GetLogger().Errorf("error decoding Space API response JSON: %v", err)
		return ""
	}

	walletAddress := spaceJson.Data.Owner.PublicAddress
	spaceName := spaceJson.Data.Space.Name
	spaceUuid := strings.ToLower(spaceJson.Data.Space.Uuid)
	spaceHardware := spaceJson.Data.Space.ActiveOrder.Config

	logs.GetLogger().Infof("uuid: %s, spaceName: %s, hardwareName: %s", spaceUuid, spaceName, spaceHardware.Description)
	if len(spaceHardware.Description) == 0 {
		return ""
	}

	deploy := NewDeploy(jobUuid, hostName, walletAddress, spaceHardware.Description, int64(duration))
	deploy.WithSpaceInfo(spaceUuid, spaceName)

	if deploy.hardwareResource.Gpu.Unit != "" {
		gpuName = strings.ReplaceAll(deploy.hardwareResource.Gpu.Unit, " ", "-")
		count, ok := runTaskGpuResource.Load(gpuName)
		if ok {
			runTaskGpuResource.Store(gpuName, count.(int)+1)
		} else {
			runTaskGpuResource.Store(gpuName, 1)
		}
	}

	updateJobStatus(jobUuid, models.JobDownloadSource)
	containsYaml, yamlPath, imagePath, err := BuildSpaceTaskImage(spaceUuid, spaceJson.Data.Files)
	if err != nil {
		logs.GetLogger().Error(err)
		return ""
	}

	if containsYaml {
		deploy.WithYamlInfo(yamlPath).YamlToK8s()
	} else {
		imageName, dockerfilePath := BuildImagesByDockerfile(jobUuid, spaceUuid, spaceName, imagePath)
		deploy.WithDockerfile(imageName, dockerfilePath).DockerfileToK8s()
	}
	return hostName
}

func deleteJob(walletAddress, namespace, spaceUuid, spaceName string) {
	deployName := constants.K8S_DEPLOY_NAME_PREFIX + spaceUuid
	serviceName := constants.K8S_SERVICE_NAME_PREFIX + spaceUuid
	ingressName := constants.K8S_INGRESS_NAME_PREFIX + spaceUuid

	k8sService := NewK8sService()
	if err := k8sService.DeleteIngress(context.TODO(), namespace, ingressName); err != nil && !errors.IsNotFound(err) {
		logs.GetLogger().Errorf("Failed delete ingress, ingressName: %s, error: %+v", deployName, err)
		return
	}
	logs.GetLogger().Infof("Deleted ingress %s finished", ingressName)

	if err := k8sService.DeleteService(context.TODO(), namespace, serviceName); err != nil && !errors.IsNotFound(err) {
		logs.GetLogger().Errorf("Failed delete service, serviceName: %s, error: %+v", serviceName, err)
		return
	}
	logs.GetLogger().Infof("Deleted service %s finished", serviceName)

	dockerService := docker.NewDockerService()
	deployImageIds, err := k8sService.GetDeploymentImages(context.TODO(), namespace, deployName)
	if err != nil && !errors.IsNotFound(err) {
		logs.GetLogger().Errorf("Failed get deploy imageIds, deployName: %s, error: %+v", deployName, err)
		return
	}
	for _, imageId := range deployImageIds {
		dockerService.RemoveImage(imageId)
	}

	if err := k8sService.DeleteDeployment(context.TODO(), namespace, deployName); err != nil && !errors.IsNotFound(err) {
		logs.GetLogger().Errorf("Failed delete deployment, deployName: %s, error: %+v", deployName, err)
		return
	}
	time.Sleep(6 * time.Second)
	logs.GetLogger().Infof("Deleted deployment %s finished", deployName)

	if err := k8sService.DeleteDeployRs(context.TODO(), namespace, spaceUuid); err != nil && !errors.IsNotFound(err) {
		logs.GetLogger().Errorf("Failed delete ReplicaSetsController, spaceUuid: %s, error: %+v", spaceUuid, err)
		return
	}

	if err := k8sService.DeletePod(context.TODO(), namespace, spaceUuid); err != nil && !errors.IsNotFound(err) {
		logs.GetLogger().Errorf("Failed delete pods, spaceUuid: %s, error: %+v", spaceUuid, err)
		return
	}

	spacePath := filepath.Join("build", walletAddress, "spaces", spaceName)
	os.RemoveAll(spacePath)

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	var count = 0
	for {
		<-ticker.C
		count++
		if count >= 20 {
			break
		}
		getPods, err := k8sService.GetPods(namespace, spaceUuid)
		if err != nil && !errors.IsNotFound(err) {
			logs.GetLogger().Errorf("Failed get pods form namespace, namepace: %s, error: %+v", namespace, err)
			continue
		}
		if !getPods {
			logs.GetLogger().Infof("Deleted all resource finised. spaceUuid: %s", spaceUuid)
			break
		}
	}
}

func updateJobStatus(jobUuid string, jobStatus models.JobStatus) {
	go func() {
		deployingChan <- models.Job{
			Uuid:   jobUuid,
			Status: jobStatus,
		}
	}()
}

func generateString(length int) string {
	characters := "abcdefghijklmnopqrstuvwxyz"
	numbers := "0123456789"
	source := characters + numbers
	result := make([]byte, length)
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < length; i++ {
		result[i] = source[rand.Intn(len(source))]
	}
	return string(result)
}

func getLocation() (string, error) {
	publicIpAddress, err := getLocalIPAddress()
	if err != nil {
		return "", err
	}
	logs.GetLogger().Infof("publicIpAddress: %s", publicIpAddress)

	resp, err := http.Get("http://ip-api.com/json/" + publicIpAddress)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var ipInfo struct {
		Country     string `json:"country"`
		CountryCode string `json:"countryCode"`
		City        string `json:"city"`
		Region      string `json:"region"`
		RegionName  string `json:"regionName"`
	}
	if err = json.Unmarshal(body, &ipInfo); err != nil {
		return "", err
	}

	return strings.TrimSpace(ipInfo.RegionName) + "-" + ipInfo.CountryCode, nil
}

func getLocalIPAddress() (string, error) {
	req, err := http.NewRequest("GET", "https://ipapi.co/ip", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.212 Safari/537.36")

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	ipBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(ipBytes)), nil
}

func retrieveJobMetadata(key string) (models.CacheSpaceDetail, error) {
	redisConn := redisPool.Get()
	defer redisConn.Close()

	args := append([]interface{}{key}, "wallet_address", "space_name", "expire_time", "space_uuid")
	valuesStr, err := redis.Strings(redisConn.Do("HMGET", args...))
	if err != nil {
		logs.GetLogger().Errorf("Failed get redis key data, key: %s, error: %+v", key, err)
		return models.CacheSpaceDetail{}, err
	}

	var (
		walletAddress string
		spaceName     string
		spaceUuid     string
		expireTime    int64
	)

	if len(valuesStr) >= 3 {
		walletAddress = valuesStr[0]
		spaceName = valuesStr[1]

		expireTimeStr := valuesStr[2]
		spaceUuid = valuesStr[3]
		expireTime, err = strconv.ParseInt(strings.TrimSpace(expireTimeStr), 10, 64)
		if err != nil {
			logs.GetLogger().Errorf("Failed convert time str: [%s], error: %+v", expireTimeStr, err)
			return models.CacheSpaceDetail{}, err
		}
	}

	return models.CacheSpaceDetail{
		WalletAddress: walletAddress,
		SpaceName:     spaceName,
		SpaceUuid:     spaceUuid,
		ExpireTime:    expireTime,
	}, nil
}

func sendBuildLogs(conn *websocket.Conn, logPath string) {
	buildLog, err := os.Open(logPath)
	if err != nil {
		logs.GetLogger().Errorf("Failed to open build log file: %+v", err)
		return
	}
	defer buildLog.Close()

	scanner := bufio.NewScanner(buildLog)
	for scanner.Scan() {
		err := conn.WriteMessage(websocket.TextMessage, scanner.Bytes())
		if err != nil {
			logs.GetLogger().Errorf("Failed to send build log: %+v", err)
			return
		}
	}
}
