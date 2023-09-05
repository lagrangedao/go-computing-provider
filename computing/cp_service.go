package computing

import (
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
	"log"
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

	multiAddressSplit := strings.Split(conf.GetConfig().API.MultiAddress, "/")
	jobSourceUri := jobData.JobSourceURI
	spaceUuid := jobSourceUri[strings.LastIndex(jobSourceUri, "/")+1:]
	wsUrl := fmt.Sprintf("ws://%s:%s/api/v1/computing/lagrange/spaces/log?space_id=%s", multiAddressSplit[2], multiAddressSplit[4], spaceUuid)
	jobData.BuildLog = wsUrl + "&type=build"
	jobData.ContainerLog = wsUrl + "&type=container"

	if err = submitJob(&jobData); err != nil {
		jobData.JobResultURI = ""
	}
	updateJobStatus(jobData.UUID, models.JobUploadResult)
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

	conn, err := upgrade.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Error upgrading connection:", err)
		return
	}
	defer conn.Close()
	handleConnection(conn, spaceDetail, logType)
}

func handleConnection(conn *websocket.Conn, spaceDetail models.CacheSpaceDetail, logType string) {
	client := NewWsClient(conn)

	if logType == "build" {
		buildLogPath := filepath.Join("build", spaceDetail.WalletAddress, "spaces", spaceDetail.SpaceName, docker.BuildFileName)
		if _, err := os.Stat(buildLogPath); err != nil {
			return
		}
		logFile, _ := os.Open(buildLogPath)
		defer logFile.Close()

		client.HandleLogs(logFile)

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

			client.HandleLogs(podLogs)
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

	spacePath := filepath.Join("build", walletAddress, "spaces", spaceName)
	os.RemoveAll(spacePath)
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

func dockerfileToK8s(jobUuid, hostName, creatorWallet, spaceUuid, imageName, dockerfilePath string, hardwareResource models.Resource, duration int) {
	exposedPort, err := docker.ExtractExposedPort(dockerfilePath)
	if err != nil {
		logs.GetLogger().Infof("Failed to extract exposed port: %v", err)
		return
	}
	containerPort, err := strconv.ParseInt(exposedPort, 10, 64)
	if err != nil {
		logs.GetLogger().Errorf("Failed to convert exposed port: %v", err)
		return
	}

	// first delete old resource
	k8sNameSpace := constants.K8S_NAMESPACE_NAME_PREFIX + creatorWallet
	deleteJob(k8sNameSpace, spaceUuid)

	if err := deployNamespace(creatorWallet); err != nil {
		logs.GetLogger().Error(err)
		return
	}

	memQuantity, err := resource.ParseQuantity(fmt.Sprintf("%d%s", hardwareResource.Memory.Quantity, hardwareResource.Memory.Unit))
	if err != nil {
		logs.GetLogger().Errorf("get memory failed, error: %+v", err)
		return
	}

	storageQuantity, err := resource.ParseQuantity(fmt.Sprintf("%d%s", hardwareResource.Storage.Quantity, hardwareResource.Storage.Unit))
	if err != nil {
		logs.GetLogger().Errorf("get storage failed, error: %+v", err)
		return
	}

	// create deployment
	k8sService := NewK8sService()
	deployment := &appV1.Deployment{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      constants.K8S_DEPLOY_NAME_PREFIX + spaceUuid,
			Namespace: k8sNameSpace,
		},
		Spec: appV1.DeploymentSpec{
			Selector: &metaV1.LabelSelector{
				MatchLabels: map[string]string{"lad_app": spaceUuid},
			},

			Template: coreV1.PodTemplateSpec{
				ObjectMeta: metaV1.ObjectMeta{
					Labels:    map[string]string{"lad_app": spaceUuid},
					Namespace: k8sNameSpace,
				},

				Spec: coreV1.PodSpec{
					NodeSelector: generateLabel(hardwareResource.Gpu.Unit),
					Containers: []coreV1.Container{{
						Name:            constants.K8S_CONTAINER_NAME_PREFIX + spaceUuid,
						Image:           imageName,
						ImagePullPolicy: coreV1.PullIfNotPresent,
						Ports: []coreV1.ContainerPort{{
							ContainerPort: int32(containerPort),
						}},
						Env: []coreV1.EnvVar{
							{
								Name:  "wallet_address",
								Value: creatorWallet,
							},
							{
								Name:  "space_uuid",
								Value: spaceUuid,
							},
							{
								Name:  "result_url",
								Value: hostName,
							},
							{
								Name:  "job_uuid",
								Value: jobUuid,
							},
						},
						Resources: coreV1.ResourceRequirements{
							Limits: coreV1.ResourceList{
								coreV1.ResourceCPU:              *resource.NewQuantity(hardwareResource.Cpu.Quantity, resource.DecimalSI),
								coreV1.ResourceMemory:           memQuantity,
								coreV1.ResourceEphemeralStorage: storageQuantity,
								"nvidia.com/gpu":                resource.MustParse(fmt.Sprintf("%d", hardwareResource.Gpu.Quantity)),
							},
							Requests: coreV1.ResourceList{
								coreV1.ResourceCPU:              *resource.NewQuantity(hardwareResource.Cpu.Quantity, resource.DecimalSI),
								coreV1.ResourceMemory:           memQuantity,
								coreV1.ResourceEphemeralStorage: storageQuantity,
								"nvidia.com/gpu":                resource.MustParse(fmt.Sprintf("%d", hardwareResource.Gpu.Quantity)),
							},
						},
					}},
				},
			},
		}}
	createDeployment, err := k8sService.CreateDeployment(context.TODO(), k8sNameSpace, deployment)
	if err != nil {
		logs.GetLogger().Error(err)
		return
	}

	updateJobStatus(jobUuid, models.JobPullImage)
	logs.GetLogger().Infof("Created deployment: %s", createDeployment.GetObjectMeta().GetName())

	if _, err := deployK8sResource(k8sNameSpace, spaceUuid, hostName, containerPort); err != nil {
		logs.GetLogger().Error(err)
		return
	}
	updateJobStatus(jobUuid, models.JobDeployToK8s)

	watchContainerRunningTime(jobUuid, k8sNameSpace, spaceUuid, int64(duration))
	return
}

func yamlToK8s(jobUuid, creatorWallet, spaceUuid, yamlPath, hostName string, hardwareResource models.Resource, duration int) {
	k8sNameSpace := constants.K8S_NAMESPACE_NAME_PREFIX + creatorWallet
	deleteJob(k8sNameSpace, spaceUuid)

	containerResources, err := yaml.HandlerYaml(yamlPath)
	if err != nil {
		logs.GetLogger().Error(err)
		return
	}

	if err := deployNamespace(creatorWallet); err != nil {
		logs.GetLogger().Error(err)
		return
	}

	memQuantity, err := resource.ParseQuantity(fmt.Sprintf("%d%s", hardwareResource.Memory.Quantity, hardwareResource.Memory.Unit))
	if err != nil {
		logs.GetLogger().Error("get memory failed, error: %+v", err)
		return
	}

	storageQuantity, err := resource.ParseQuantity(fmt.Sprintf("%d%s", hardwareResource.Storage.Quantity, hardwareResource.Storage.Unit))
	if err != nil {
		logs.GetLogger().Error("get storage failed, error: %+v", err)
		return
	}

	k8sService := NewK8sService()
	for _, cr := range containerResources {
		for i, envVar := range cr.Env {
			if strings.Contains(envVar.Name, "NEXTAUTH_URL") {
				cr.Env[i].Value = "https://" + hostName
				break
			}
		}

		var volumeMount []coreV1.VolumeMount
		var volumes []coreV1.Volume
		if cr.VolumeMounts.Path != "" {
			fileNameWithoutExt := filepath.Base(cr.VolumeMounts.Name[:len(cr.VolumeMounts.Name)-len(filepath.Ext(cr.VolumeMounts.Name))])
			configMap, err := k8sService.CreateConfigMap(context.TODO(), k8sNameSpace, spaceUuid, filepath.Dir(yamlPath), cr.VolumeMounts.Name)
			if err != nil {
				logs.GetLogger().Error(err)
				return
			}
			configName := configMap.GetName()
			volumes = []coreV1.Volume{
				{
					Name: spaceUuid + "-" + fileNameWithoutExt,
					VolumeSource: coreV1.VolumeSource{
						ConfigMap: &coreV1.ConfigMapVolumeSource{
							LocalObjectReference: coreV1.LocalObjectReference{
								Name: configName,
							},
						},
					},
				},
			}
			volumeMount = []coreV1.VolumeMount{
				{
					Name:      spaceUuid + "-" + fileNameWithoutExt,
					MountPath: cr.VolumeMounts.Path,
				},
			}
		}

		var containers []coreV1.Container
		for _, depend := range cr.Depends {
			var handler = new(coreV1.ExecAction)
			handler.Command = depend.ReadyCmd
			containers = append(containers, coreV1.Container{
				Name:            spaceUuid + "-" + depend.Name,
				Image:           depend.ImageName,
				Command:         depend.Command,
				Args:            depend.Args,
				Env:             depend.Env,
				Ports:           depend.Ports,
				ImagePullPolicy: coreV1.PullIfNotPresent,
				Resources:       coreV1.ResourceRequirements{},
				ReadinessProbe: &coreV1.Probe{
					ProbeHandler: coreV1.ProbeHandler{
						Exec: handler,
					},
					InitialDelaySeconds: 5,
					PeriodSeconds:       5,
				},
			})
		}

		cr.Env = append(cr.Env, []coreV1.EnvVar{
			{
				Name:  "wallet_address",
				Value: creatorWallet,
			},
			{
				Name:  "space_uuid",
				Value: spaceUuid,
			},
			{
				Name:  "result_url",
				Value: hostName,
			},
			{
				Name:  "job_uuid",
				Value: jobUuid,
			},
		}...)

		containers = append(containers, coreV1.Container{
			Name:            spaceUuid + "-" + cr.Name,
			Image:           cr.ImageName,
			Command:         cr.Command,
			Args:            cr.Args,
			Env:             cr.Env,
			Ports:           cr.Ports,
			ImagePullPolicy: coreV1.PullIfNotPresent,
			Resources: coreV1.ResourceRequirements{
				Limits: coreV1.ResourceList{
					coreV1.ResourceCPU:              *resource.NewQuantity(hardwareResource.Cpu.Quantity, resource.DecimalSI),
					coreV1.ResourceMemory:           memQuantity,
					coreV1.ResourceEphemeralStorage: storageQuantity,
					"nvidia.com/gpu":                resource.MustParse(fmt.Sprintf("%d", hardwareResource.Gpu.Quantity)),
				},
				Requests: coreV1.ResourceList{
					coreV1.ResourceCPU:              *resource.NewQuantity(hardwareResource.Cpu.Quantity, resource.DecimalSI),
					coreV1.ResourceMemory:           memQuantity,
					coreV1.ResourceEphemeralStorage: storageQuantity,
					"nvidia.com/gpu":                resource.MustParse(fmt.Sprintf("%d", hardwareResource.Gpu.Quantity)),
				},
			},
			VolumeMounts: volumeMount,
		})

		deployment := &appV1.Deployment{
			TypeMeta: metaV1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
			ObjectMeta: metaV1.ObjectMeta{
				Name:      constants.K8S_DEPLOY_NAME_PREFIX + spaceUuid,
				Namespace: k8sNameSpace,
			},

			Spec: appV1.DeploymentSpec{
				Selector: &metaV1.LabelSelector{
					MatchLabels: map[string]string{"lad_app": spaceUuid},
				},
				Template: coreV1.PodTemplateSpec{
					ObjectMeta: metaV1.ObjectMeta{
						Labels:    map[string]string{"lad_app": spaceUuid},
						Namespace: k8sNameSpace,
					},
					Spec: coreV1.PodSpec{
						NodeSelector: generateLabel(hardwareResource.Gpu.Unit),
						Containers:   containers,
						Volumes:      volumes,
					},
				},
			}}

		createDeployment, err := k8sService.CreateDeployment(context.TODO(), k8sNameSpace, deployment)
		if err != nil {
			logs.GetLogger().Error(err)
			return
		}

		updateJobStatus(jobUuid, models.JobPullImage)
		logs.GetLogger().Infof("Created deployment: %s", createDeployment.GetObjectMeta().GetName())

		serviceIp, err := deployK8sResource(k8sNameSpace, spaceUuid, hostName, int64(cr.Ports[0].ContainerPort))
		if err != nil {
			logs.GetLogger().Error(err)
			return
		}
		updateJobStatus(jobUuid, models.JobDeployToK8s)

		if cr.ModelSetting.TargetDir != "" && len(cr.ModelSetting.Resources) > 0 {
			for _, res := range cr.ModelSetting.Resources {
				go func(res yaml.ModelResource) {
					downloadModelUrl(k8sNameSpace, spaceUuid, serviceIp, []string{"wget", res.Url, "-O", filepath.Join(cr.ModelSetting.TargetDir, res.Name)})
				}(res)
			}
		}

		watchContainerRunningTime(jobUuid, k8sNameSpace, spaceUuid, int64(duration))
	}
}

func deployNamespace(creatorWallet string) error {
	k8sNameSpace := constants.K8S_NAMESPACE_NAME_PREFIX + creatorWallet
	k8sService := NewK8sService()
	// create namespace
	if _, err := k8sService.GetNameSpace(context.TODO(), k8sNameSpace, metaV1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			namespace := &coreV1.Namespace{
				ObjectMeta: metaV1.ObjectMeta{
					Name: k8sNameSpace,
					Labels: map[string]string{
						"lab-ns": creatorWallet,
					},
				},
			}
			createdNamespace, err := k8sService.CreateNameSpace(context.TODO(), namespace, metaV1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed create namespace, error: %w", err)
			}
			logs.GetLogger().Infof("create namespace successfully, namespace: %s", createdNamespace.Name)

			//networkPolicy, err := k8sService.CreateNetworkPolicy(context.TODO(), k8sNameSpace)
			//if err != nil {
			//	return fmt.Errorf("failed create networkPolicy, error: %w", err)
			//}
			//logs.GetLogger().Infof("create networkPolicy successfully, networkPolicyName: %s", networkPolicy.Name)
		} else {
			return err
		}
	}
	return nil
}

func deployK8sResource(k8sNameSpace, spaceUuid, hostName string, containerPort int64) (string, error) {
	k8sService := NewK8sService()

	// create service
	createService, err := k8sService.CreateService(context.TODO(), k8sNameSpace, spaceUuid, int32(containerPort))
	if err != nil {
		return "", fmt.Errorf("failed creata service, error: %w", err)
	}
	logs.GetLogger().Infof("Created service successfully: %s", createService.GetObjectMeta().GetName())

	serviceIp := fmt.Sprintf("http://%s:%d", createService.Spec.ClusterIP, createService.Spec.Ports[0].Port)

	// create ingress
	createIngress, err := k8sService.CreateIngress(context.TODO(), k8sNameSpace, spaceUuid, hostName, int32(containerPort))
	if err != nil {
		return "", fmt.Errorf("failed creata ingress, error: %w", err)
	}
	logs.GetLogger().Infof("Created Ingress successfully: %s", createIngress.GetObjectMeta().GetName())
	return serviceIp, nil
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

func watchContainerRunningTime(key, namespace, spaceUuid string, runTime int64) {
	conn := redisPool.Get()
	_, err := conn.Do("SET", key, "wait-delete", "EX", runTime)
	if err != nil {
		logs.GetLogger().Errorf("Failed set redis key and expire time, key: %s, error: %+v", key, err)
		return
	}

	fullArgs := []interface{}{constants.REDIS_FULL_PREFIX + key}
	fields := map[string]string{
		"k8s_namespace": namespace,
		"expire_time":   strconv.Itoa(int(time.Now().Unix() + runTime)),
		"space_uuid":    spaceUuid,
	}

	for key, val := range fields {
		fullArgs = append(fullArgs, key, val)
	}
	conn.Do("HSET", fullArgs...)

	go func() {
		psc := redis.PubSubConn{Conn: redisPool.Get()}
		psc.PSubscribe("__keyevent@0__:expired")
		for {
			switch n := psc.Receive().(type) {
			case redis.Message:
				if n.Channel == "__keyevent@0__:expired" && string(n.Data) == key {
					logs.GetLogger().Infof("The namespace: %s, spaceUuid: %s, job has reached its runtime and will stop running.", namespace, spaceUuid)
					deleteJob(namespace, spaceUuid)
					redisPool.Get().Do("DEL", constants.REDIS_FULL_PREFIX+key)
				}
			case redis.Subscription:
				logs.GetLogger().Infof("Subscribe %s", n.Channel)
			case error:
				return
			}
		}
	}()
}

func downloadModelUrl(namespace, spaceUuid, serviceIp string, podCmd []string) {
	k8sService := NewK8sService()
	podName, err := k8sService.WaitForPodRunning(namespace, spaceUuid, serviceIp)
	if err != nil {
		logs.GetLogger().Error(err)
		return
	}

	if err = k8sService.PodDoCommand(namespace, podName, "", podCmd); err != nil {
		logs.GetLogger().Error(err)
		return
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
