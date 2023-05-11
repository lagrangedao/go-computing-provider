package computing

import (
	"context"
	"flag"
	"fmt"
	"github.com/circonus-labs/circonus-gometrics/api/config"
	"github.com/filswan/go-swan-lib/logs"
	appV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"net"
	"path/filepath"
	"strconv"
)

type K8sService struct {
	k8sClient *kubernetes.Clientset
}

func NewK8sService() *K8sService {
	config, err := rest.InClusterConfig()
	if err != nil {
		// 如果不在集群内，则尝试使用kubeconfig文件进行认证
		var kubeconfig *string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logs.GetLogger().Errorf("Failed create k8s clientset, error: %v", err)
		return nil
	}
	return &K8sService{
		k8sClient: clientset,
	}
}

func (s *K8sService) CreateDeployment(ctx context.Context, nameSpace string, deploy DeploymentReq) (result *appV1.Deployment, err error) {
	deployment := &appV1.Deployment{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name: deploy.ContainerName,
		},
		Spec: appV1.DeploymentSpec{
			Selector: &metaV1.LabelSelector{
				MatchLabels: deploy.Label,
			},

			Template: coreV1.PodTemplateSpec{
				ObjectMeta: metaV1.ObjectMeta{
					Labels: deploy.Label,
				},

				Spec: coreV1.PodSpec{
					//ImagePullSecrets: coreV1.
					Containers: []coreV1.Container{{
						Name:            deploy.ContainerName,
						Image:           deploy.ImageName,
						ImagePullPolicy: coreV1.PullIfNotPresent,
						Ports: []coreV1.ContainerPort{{
							ContainerPort: deploy.ContainerPort,
						}},
					}},
				},
			},
		}}

	return s.k8sClient.AppsV1().Deployments(nameSpace).Create(ctx, deployment, metaV1.CreateOptions{})
}

func (s *K8sService) CreateService(ctx context.Context, namespace string,
	service *coreV1.Service, opts metaV1.CreateOptions) (result *coreV1.Service, err error) {
	return s.k8sClient.CoreV1().Services(namespace).Create(ctx, service, opts)
}

func (s *K8sService) GetServiceByName(ctx context.Context, namespace string,
	serviceName string, opts metaV1.GetOptions) (result *coreV1.Service, err error) {
	return s.k8sClient.CoreV1().Services(namespace).Get(ctx, serviceName, opts)
}

func (s *K8sService) GetNodeList() (ip string, err error) {
	// 获取所有节点的 IP 地址
	nodes, err := s.k8sClient.CoreV1().Nodes().List(context.Background(), metaV1.ListOptions{})
	if err != nil {
		logs.GetLogger().Error(err)
		return "", err
	}

	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if addr.Type == coreV1.NodeInternalIP {
				ipAddr := net.ParseIP(addr.Address)
				if ipAddr != nil {
					ip = ipAddr.String()
				}
			}
		}
	}
	return ip, nil
}

func (s *K8sService) GetPods(namespace string) {
	podList, err := s.k8sClient.CoreV1().Pods(namespace).List(context.TODO(), metaV1.ListOptions{})
	if err != nil {
		logs.GetLogger().Error(err)
		return
	}
	for _, pod := range podList.Items {
		logs.GetLogger().Infof("name: %s, namespace: %s", pod.Name, pod.Namespace)
	}
}

type DeploymentReq struct {
	ContainerName string
	ImageName     string
	Label         map[string]string
	ContainerPort int32
}

func runContainerToK8s(imageName, dockerfilePath string, spaceName string) string {
	exposedPort, err := ExtractExposedPort(dockerfilePath)
	if err != nil {
		logs.GetLogger().Infof("Failed to extract exposed port: %v", err)
		return ""
	}

	containerName := "computing-worker" + spaceName
	containerPort, err := strconv.ParseInt(exposedPort, 10, 64)
	if err != nil {
		logs.GetLogger().Errorf("Failed to convert exposed port: %v", err)
		return ""
	}

	k8sService := NewK8sService()
	createDeployment, err := k8sService.CreateDeployment(context.TODO(), coreV1.NamespaceDefault, DeploymentReq{
		ContainerName: containerName,
		ImageName:     imageName,
		Label:         map[string]string{"app": spaceName},
		ContainerPort: int32(containerPort),
	})
	if err != nil {
		logs.GetLogger().Error(err)
		return ""
	}
	logs.GetLogger().Infof("Created deployment: %s", createDeployment.GetObjectMeta().GetName())

	service := &coreV1.Service{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name: spaceName,
		},
		Spec: coreV1.ServiceSpec{
			Type: coreV1.ServiceTypeNodePort,
			Ports: []coreV1.ServicePort{
				{
					Port:       int32(containerPort),
					TargetPort: intstr.FromString(exposedPort),
					Protocol:   coreV1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": containerName,
			},
		},
	}
	createService, err := k8sService.CreateService(context.TODO(), coreV1.NamespaceDefault, service, metaV1.CreateOptions{})
	if err != nil {
		logs.GetLogger().Error(err)
		return ""
	}
	logs.GetLogger().Infof("Created service %s", createService.GetObjectMeta().GetName())

	service, err = k8sService.GetServiceByName(context.TODO(), coreV1.NamespaceDefault, spaceName, metaV1.GetOptions{})
	if err != nil {
		logs.GetLogger().Error(err)
		return ""
	}
	port := service.Spec.Ports[0].NodePort
	fmt.Printf("Service is exposed at %s:%d\n", config.Host, port)

	hostIp, err := k8sService.GetNodeList()
	if err != nil {
		logs.GetLogger().Error(err)
		return ""
	}
	url := "http://" + hostIp + ":" + strconv.Itoa(int(port))
	return url
}
