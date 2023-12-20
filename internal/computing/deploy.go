package computing

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/filswan/go-mcs-sdk/mcs/api/common/logs"
	"github.com/gomodule/redigo/redis"
	"github.com/lagrangedao/go-computing-provider/constants"
	"github.com/lagrangedao/go-computing-provider/internal/models"
	"github.com/lagrangedao/go-computing-provider/internal/yaml"
	"github.com/lagrangedao/go-computing-provider/util"
	appV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Deploy struct {
	jobUuid           string
	hostName          string
	walletAddress     string
	spaceUuid         string
	spaceName         string
	image             string
	dockerfilePath    string
	yamlPath          string
	duration          int64
	hardwareResource  models.Resource
	modelsSettingFile string
	k8sNameSpace      string
	SpacePath         string
	TaskType          string
	DeployName        string
	hardwareDesc      string
	taskUuid          string
}

func NewDeploy(jobUuid, hostName, walletAddress, hardwareDesc string, duration int64, taskUuid string) *Deploy {
	taskType, hardwareDetail := getHardwareDetail(hardwareDesc)
	return &Deploy{
		jobUuid:          jobUuid,
		hostName:         hostName,
		walletAddress:    walletAddress,
		duration:         duration,
		hardwareResource: hardwareDetail,
		TaskType:         taskType,
		k8sNameSpace:     constants.K8S_NAMESPACE_NAME_PREFIX + strings.ToLower(walletAddress),
		hardwareDesc:     hardwareDesc,
		taskUuid:         taskUuid,
	}
}

func (d *Deploy) WithSpaceInfo(spaceUuid, spaceName string) *Deploy {
	d.spaceUuid = spaceUuid
	d.spaceName = spaceName
	return d
}

func (d *Deploy) WithYamlInfo(yamlPath string) *Deploy {
	d.yamlPath = yamlPath
	return d
}

func (d *Deploy) WithDockerfile(image, dockerfilePath string) *Deploy {
	d.image = image
	d.dockerfilePath = dockerfilePath
	return d
}

func (d *Deploy) WithSpacePath(spacePath string) *Deploy {
	d.SpacePath = spacePath
	return d
}

func (d *Deploy) WithModelSettingFile(modelsSettingFile string) *Deploy {
	d.modelsSettingFile = modelsSettingFile
	return d
}

func (d *Deploy) DockerfileToK8s() {
	exposedPort, err := ExtractExposedPort(d.dockerfilePath)
	if err != nil {
		logs.GetLogger().Infof("Failed to extract exposed port: %v", err)
		return
	}
	containerPort, err := strconv.ParseInt(exposedPort, 10, 64)
	if err != nil {
		logs.GetLogger().Errorf("Failed to convert exposed port: %v", err)
		return
	}

	deleteJob(d.k8sNameSpace, d.spaceUuid)

	if err := d.deployNamespace(); err != nil {
		logs.GetLogger().Error(err)
		return
	}

	k8sService := NewK8sService()
	deployment := &appV1.Deployment{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      constants.K8S_DEPLOY_NAME_PREFIX + d.spaceUuid,
			Namespace: d.k8sNameSpace,
		},
		Spec: appV1.DeploymentSpec{
			Selector: &metaV1.LabelSelector{
				MatchLabels: map[string]string{"lad_app": d.spaceUuid},
			},

			Template: coreV1.PodTemplateSpec{
				ObjectMeta: metaV1.ObjectMeta{
					Labels:    map[string]string{"lad_app": d.spaceUuid},
					Namespace: d.k8sNameSpace,
				},

				Spec: coreV1.PodSpec{
					NodeSelector: generateLabel(d.hardwareResource.Gpu.Unit),
					Containers: []coreV1.Container{{
						Name:            constants.K8S_CONTAINER_NAME_PREFIX + d.spaceUuid,
						Image:           d.image,
						ImagePullPolicy: coreV1.PullIfNotPresent,
						Ports: []coreV1.ContainerPort{{
							ContainerPort: int32(containerPort),
						}},
						Env:       d.createEnv(),
						Resources: d.createResources(),
					}},
				},
			},
		}}
	createDeployment, err := k8sService.CreateDeployment(context.TODO(), d.k8sNameSpace, deployment)
	if err != nil {
		logs.GetLogger().Error(err)
		return
	}
	d.DeployName = createDeployment.GetName()
	updateJobStatus(d.jobUuid, models.JobPullImage)
	logs.GetLogger().Infof("Created deployment: %s", createDeployment.GetName())

	if _, err := d.deployK8sResource(int32(containerPort)); err != nil {
		logs.GetLogger().Error(err)
		return
	}
	updateJobStatus(d.jobUuid, models.JobDeployToK8s, "https://"+d.hostName)

	d.watchContainerRunningTime()
	return
}

func (d *Deploy) YamlToK8s() {
	containerResources, err := yaml.HandlerYaml(d.yamlPath)
	if err != nil {
		logs.GetLogger().Error(err)
		return
	}

	deleteJob(d.k8sNameSpace, d.spaceUuid)

	if err := d.deployNamespace(); err != nil {
		logs.GetLogger().Error(err)
		return
	}

	k8sService := NewK8sService()
	for _, cr := range containerResources {
		for i, envVar := range cr.Env {
			if strings.Contains(envVar.Name, "NEXTAUTH_URL") {
				cr.Env[i].Value = "https://" + d.hostName
				break
			}
		}

		var volumeMount []coreV1.VolumeMount
		var volumes []coreV1.Volume
		if cr.VolumeMounts.Path != "" {
			fileNameWithoutExt := filepath.Base(cr.VolumeMounts.Name[:len(cr.VolumeMounts.Name)-len(filepath.Ext(cr.VolumeMounts.Name))])
			configMap, err := k8sService.CreateConfigMap(context.TODO(), d.k8sNameSpace, d.spaceUuid, filepath.Dir(d.yamlPath), cr.VolumeMounts.Name)
			if err != nil {
				logs.GetLogger().Error(err)
				return
			}
			configName := configMap.GetName()
			volumes = []coreV1.Volume{
				{
					Name: d.spaceUuid + "-" + fileNameWithoutExt,
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
					Name:      d.spaceUuid + "-" + fileNameWithoutExt,
					MountPath: cr.VolumeMounts.Path,
				},
			}
		}

		var containers []coreV1.Container
		for _, depend := range cr.Depends {
			var handler = new(coreV1.ExecAction)
			handler.Command = depend.ReadyCmd
			containers = append(containers, coreV1.Container{
				Name:            d.spaceUuid + "-" + depend.Name,
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
				Value: d.walletAddress,
			},
			{
				Name:  "space_uuid",
				Value: d.spaceUuid,
			},
			{
				Name:  "result_url",
				Value: d.hostName,
			},
			{
				Name:  "job_uuid",
				Value: d.jobUuid,
			},
		}...)

		containers = append(containers, coreV1.Container{
			Name:            d.spaceUuid + "-" + cr.Name,
			Image:           cr.ImageName,
			Command:         cr.Command,
			Args:            cr.Args,
			Env:             cr.Env,
			Ports:           cr.Ports,
			ImagePullPolicy: coreV1.PullIfNotPresent,
			Resources:       d.createResources(),
			VolumeMounts:    volumeMount,
		})

		deployment := &appV1.Deployment{
			TypeMeta: metaV1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
			ObjectMeta: metaV1.ObjectMeta{
				Name:      constants.K8S_DEPLOY_NAME_PREFIX + d.spaceUuid,
				Namespace: d.k8sNameSpace,
			},

			Spec: appV1.DeploymentSpec{
				Selector: &metaV1.LabelSelector{
					MatchLabels: map[string]string{"lad_app": d.spaceUuid},
				},
				Template: coreV1.PodTemplateSpec{
					ObjectMeta: metaV1.ObjectMeta{
						Labels:    map[string]string{"lad_app": d.spaceUuid},
						Namespace: d.k8sNameSpace,
					},
					Spec: coreV1.PodSpec{
						NodeSelector: generateLabel(d.hardwareResource.Gpu.Unit),
						Containers:   containers,
						Volumes:      volumes,
					},
				},
			}}

		createDeployment, err := k8sService.CreateDeployment(context.TODO(), d.k8sNameSpace, deployment)
		if err != nil {
			logs.GetLogger().Error(err)
			return
		}
		d.DeployName = createDeployment.GetName()
		updateJobStatus(d.jobUuid, models.JobPullImage)
		logs.GetLogger().Infof("Created deployment: %s", createDeployment.GetObjectMeta().GetName())

		serviceHost, err := d.deployK8sResource(cr.Ports[0].ContainerPort)
		if err != nil {
			logs.GetLogger().Error(err)
			return
		}

		updateJobStatus(d.jobUuid, models.JobDeployToK8s, "https://"+d.hostName)

		if len(cr.Models) > 0 {
			for _, res := range cr.Models {
				go func(res yaml.ModelResource) {
					downloadModelUrl(d.k8sNameSpace, d.spaceUuid, serviceHost, []string{"wget", res.Url, "-O", filepath.Join(res.Dir, res.Name)})
				}(res)
			}
		}
		d.watchContainerRunningTime()
	}
}

func (d *Deploy) ModelInferenceToK8s() error {
	var modelSetting struct {
		ModelId string `json:"model_id"`
	}
	modelData, _ := os.ReadFile(d.modelsSettingFile)
	err := json.Unmarshal(modelData, &modelSetting)
	if err != nil {
		logs.GetLogger().Errorf("convert model_id out to json failed, error: %+v", err)
		return err
	}

	cpPath, _ := os.LookupEnv("CP_PATH")
	basePath := filepath.Join(cpPath, "inference-model")

	modelInfoOut, err := util.RunPythonScript(filepath.Join(basePath, "/scripts/hf_client.py"), "model_info", modelSetting.ModelId)
	if err != nil {
		logs.GetLogger().Errorf("exec model_info cmd failed, error: %+v", err)
		return err
	}

	var modelInfo struct {
		ModelId   string `json:"model_id"`
		Task      string `json:"task"`
		Framework string `json:"framework"`
	}
	err = json.Unmarshal([]byte(modelInfoOut), &modelInfo)
	if err != nil {
		logs.GetLogger().Errorf("convert model_info out to json failed, error: %+v", err)
		return err
	}

	deleteJob(d.k8sNameSpace, d.spaceUuid)
	imageName := "lagrange/" + modelInfo.Framework + ":v1.0"

	logFile := filepath.Join(d.SpacePath, BuildFileName)
	if _, err = os.Create(logFile); err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)
	util.StreamPythonScriptOutput(&wg, filepath.Join(basePath, "build_docker.py"), basePath, modelInfo.Framework, imageName, logFile)
	wg.Wait()

	modelEnvs := []coreV1.EnvVar{
		{
			Name:  "TASK",
			Value: modelInfo.Task,
		},
		{
			Name:  "MODEL_ID",
			Value: modelInfo.ModelId,
		},
	}

	d.image = imageName

	if err := d.deployNamespace(); err != nil {
		logs.GetLogger().Error(err)
		return err
	}

	k8sService := NewK8sService()
	deployment := &appV1.Deployment{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      constants.K8S_DEPLOY_NAME_PREFIX + d.spaceUuid,
			Namespace: d.k8sNameSpace,
		},
		Spec: appV1.DeploymentSpec{
			Selector: &metaV1.LabelSelector{
				MatchLabels: map[string]string{"lad_app": d.spaceUuid},
			},

			Template: coreV1.PodTemplateSpec{
				ObjectMeta: metaV1.ObjectMeta{
					Labels:    map[string]string{"lad_app": d.spaceUuid},
					Namespace: d.k8sNameSpace,
				},

				Spec: coreV1.PodSpec{
					NodeSelector: generateLabel(d.hardwareResource.Gpu.Unit),
					Containers: []coreV1.Container{{
						Name:            constants.K8S_CONTAINER_NAME_PREFIX + d.spaceUuid,
						Image:           d.image,
						ImagePullPolicy: coreV1.PullIfNotPresent,
						Ports: []coreV1.ContainerPort{{
							ContainerPort: int32(80),
						}},
						Env: d.createEnv(modelEnvs...),
						//Resources: d.createResources(),
					}},
				},
			},
		}}
	createDeployment, err := k8sService.CreateDeployment(context.TODO(), d.k8sNameSpace, deployment)
	if err != nil {
		logs.GetLogger().Error(err)
		return err
	}
	d.DeployName = createDeployment.GetName()
	updateJobStatus(d.jobUuid, models.JobPullImage)
	logs.GetLogger().Infof("Created deployment: %s", createDeployment.GetObjectMeta().GetName())

	if _, err := d.deployK8sResource(int32(80)); err != nil {
		logs.GetLogger().Error(err)
		return err
	}
	updateJobStatus(d.jobUuid, models.JobDeployToK8s)
	d.watchContainerRunningTime()
	return nil
}

func (d *Deploy) deployNamespace() error {
	k8sService := NewK8sService()
	// create namespace
	if _, err := k8sService.GetNameSpace(context.TODO(), d.k8sNameSpace, metaV1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			namespace := &coreV1.Namespace{
				ObjectMeta: metaV1.ObjectMeta{
					Name: d.k8sNameSpace,
					Labels: map[string]string{
						"lab-ns": strings.ToLower(d.walletAddress),
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

func (d *Deploy) createEnv(envs ...coreV1.EnvVar) []coreV1.EnvVar {
	defaultEnv := []coreV1.EnvVar{
		{
			Name:  "space_uuid",
			Value: d.spaceUuid,
		},
		{
			Name:  "space_name",
			Value: d.spaceName,
		},
		{
			Name:  "result_url",
			Value: d.hostName,
		},
		{
			Name:  "job_uuid",
			Value: d.jobUuid,
		},
	}

	defaultEnv = append(defaultEnv, envs...)
	return defaultEnv
}

func (d *Deploy) createResources() coreV1.ResourceRequirements {

	memQuantity, err := resource.ParseQuantity(fmt.Sprintf("%d%s", d.hardwareResource.Memory.Quantity, d.hardwareResource.Memory.Unit))
	if err != nil {
		logs.GetLogger().Error("get memory failed, error: %+v", err)
		return coreV1.ResourceRequirements{}
	}

	storageQuantity, err := resource.ParseQuantity(fmt.Sprintf("%d%s", d.hardwareResource.Storage.Quantity, d.hardwareResource.Storage.Unit))
	if err != nil {
		logs.GetLogger().Error("get storage failed, error: %+v", err)
		return coreV1.ResourceRequirements{}
	}

	return coreV1.ResourceRequirements{
		Limits: coreV1.ResourceList{
			coreV1.ResourceCPU:              *resource.NewQuantity(d.hardwareResource.Cpu.Quantity, resource.DecimalSI),
			coreV1.ResourceMemory:           memQuantity,
			coreV1.ResourceEphemeralStorage: storageQuantity,
			"nvidia.com/gpu":                resource.MustParse(fmt.Sprintf("%d", d.hardwareResource.Gpu.Quantity)),
		},
		Requests: coreV1.ResourceList{
			coreV1.ResourceCPU:              *resource.NewQuantity(d.hardwareResource.Cpu.Quantity, resource.DecimalSI),
			coreV1.ResourceMemory:           memQuantity,
			coreV1.ResourceEphemeralStorage: storageQuantity,
			"nvidia.com/gpu":                resource.MustParse(fmt.Sprintf("%d", d.hardwareResource.Gpu.Quantity)),
		},
	}
}

func (d *Deploy) deployK8sResource(containerPort int32) (string, error) {
	k8sService := NewK8sService()

	createService, err := k8sService.CreateService(context.TODO(), d.k8sNameSpace, d.spaceUuid, containerPort)
	if err != nil {
		return "", fmt.Errorf("failed creata service, error: %w", err)
	}
	logs.GetLogger().Infof("Created service successfully: %s", createService.GetObjectMeta().GetName())

	serviceHost := fmt.Sprintf("http://%s:%d", createService.Spec.ClusterIP, createService.Spec.Ports[0].Port)

	createIngress, err := k8sService.CreateIngress(context.TODO(), d.k8sNameSpace, d.spaceUuid, d.hostName, containerPort)
	if err != nil {
		return "", fmt.Errorf("failed creata ingress, error: %w", err)
	}
	logs.GetLogger().Infof("Created Ingress successfully: %s", createIngress.GetObjectMeta().GetName())
	return serviceHost, nil
}

func (d *Deploy) watchContainerRunningTime() {
	conn := redisPool.Get()
	_, err := conn.Do("SET", d.spaceUuid, "wait-delete", "EX", d.duration)
	if err != nil {
		logs.GetLogger().Errorf("Failed set redis key and expire time, key: %s, error: %+v", d.jobUuid, err)
		return
	}

	key := constants.REDIS_FULL_PREFIX + d.spaceUuid
	conn.Do("DEL", redis.Args{}.AddFlat(key)...)

	fullArgs := []interface{}{key}
	fields := map[string]string{
		"wallet_address": d.walletAddress,
		"space_name":     d.spaceName,
		"expire_time":    strconv.Itoa(int(time.Now().Unix() + d.duration)),
		"space_uuid":     d.spaceUuid,
		"job_uuid":       d.jobUuid,
		"task_type":      d.TaskType,
		"deploy_name":    d.DeployName,
		"hardware":       d.hardwareDesc,
		"url":            fmt.Sprintf("https://%s", d.hostName),
		"task_uuid":      d.taskUuid,
	}

	for key, val := range fields {
		fullArgs = append(fullArgs, key, val)
	}
	_, _ = conn.Do("HSET", fullArgs...)

}

func getHardwareDetail(description string) (string, models.Resource) {
	var taskType string
	var hardwareResource models.Resource
	confSplits := strings.Split(description, "·")
	if strings.Contains(confSplits[0], "CPU") {
		hardwareResource.Gpu.Quantity = 0
		hardwareResource.Gpu.Unit = ""
		taskType = "CPU"
	} else {
		taskType = "GPU"
		hardwareResource.Gpu.Quantity = 1
		oldName := strings.TrimSpace(confSplits[0])
		hardwareResource.Gpu.Unit = strings.ReplaceAll(oldName, "Nvidia", "NVIDIA")
	}

	cpuSplits := strings.Split(confSplits[1], " ")
	cores, _ := strconv.ParseInt(cpuSplits[1], 10, 64)
	hardwareResource.Cpu.Quantity = cores
	hardwareResource.Cpu.Unit = cpuSplits[2]

	memSplits := strings.Split(confSplits[2], " ")
	mem, _ := strconv.ParseInt(memSplits[1], 10, 64)
	hardwareResource.Memory.Quantity = mem
	hardwareResource.Memory.Unit = strings.ReplaceAll(memSplits[2], "B", "")

	hardwareResource.Storage.Quantity = 30
	hardwareResource.Storage.Unit = "Gi"
	return taskType, hardwareResource
}
