package yaml

import (
	"gopkg.in/errgo.v2/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"strings"
)

type DeployYamlV2 struct {
	Version    string                `yaml:"version"`
	Services   map[string]Service    `yaml:"services"`
	Profiles   Profiles              `yaml:"profiles"`
	Deployment map[string]Deployment `yaml:"deployment"`
}

func (dy *DeployYamlV2) checkRequired() error {
	if len(dy.Services) <= 0 {
		return errors.New("at least one service must be defined")
	}
	return nil
}

func (dy *DeployYamlV2) ServiceToK8sResource() ([]ContainerResource, error) {
	if err := dy.checkRequired(); err != nil {
		return nil, err
	}
	var containers []ContainerResource
	for name, deployment := range dy.Deployment {
		container := new(ContainerResource)
		if service, ok := dy.Services[name]; ok {
			container.ImageName = service.Image
			if len(service.Command) > 0 {
				container.Command = service.Command
			}
			if len(service.Args) > 0 {
				container.Args = service.Args
			}
			if len(service.Env) > 0 {
				var envVars []corev1.EnvVar
				for _, env := range service.Env {
					envSplit := strings.Split(strings.TrimSpace(env), "=")
					envVars = append(envVars, corev1.EnvVar{
						Name:  envSplit[0],
						Value: envSplit[1],
					})
				}
				container.Env = envVars
			}
			if len(service.Expose) > 0 {
				var ports []corev1.ContainerPort
				for _, expose := range service.Expose {
					ports = append(ports, corev1.ContainerPort{
						ContainerPort: int32(expose.Port),
						Protocol:      corev1.ProtocolTCP,
					})
				}
				container.Ports = ports
			}
		}

		var resourceList = make(corev1.ResourceList)
		if cpRs, ok := dy.Profiles.Compute[deployment.Akash.Profile]; ok {
			if cpRs.Resources.Cpu.Units != "" {
				resourceList[corev1.ResourceCPU] = resource.MustParse(cpRs.Resources.Cpu.Units)
			}
			if cpRs.Resources.Memory.Size != "" {
				resourceList[corev1.ResourceMemory] = resource.MustParse(cpRs.Resources.Memory.Size)
			}
			if cpRs.Resources.Storage.Size != "" {
				resourceList[corev1.ResourceStorage] = resource.MustParse(cpRs.Resources.Storage.Size)
			}
		}

		container.ResourceLimit = resourceList
		container.Count = deployment.Akash.Count
		containers = append(containers, *container)
	}
	return containers, nil
}

type Service struct {
	Image   string   `yaml:"image"`
	Command []string `yaml:"command"`
	Args    []string `yaml:"args"`
	Env     []string `yaml:"env"`
	Expose  []Expose `yaml:"expose"`
}

type Expose struct {
	Port int `yaml:"port"`
	To   []struct {
		Global bool `yaml:"global"`
	} `yaml:"to"`
	As int `yaml:"as"`
}

type Profiles struct {
	Compute map[string]Compute `yaml:"compute"`
}

type Compute struct {
	Resources struct {
		Cpu struct {
			Units string `yaml:"units"`
		} `yaml:"cpu"`
		Memory struct {
			Size string `yaml:"size"`
		} `yaml:"memory"`
		Storage struct {
			Size string `yaml:"size"`
		} `yaml:"storage"`
	} `yaml:"resources"`
}

type Deployment struct {
	Akash struct {
		Profile string `yaml:"profile"`
		Count   int    `yaml:"count"`
	} `yaml:"akash"`
}
