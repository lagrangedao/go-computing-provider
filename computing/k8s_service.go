package computing

import (
	"context"
	"flag"
	"fmt"
	"github.com/lagrangedao/go-computing-provider/constants"
	"net"
	"path/filepath"
	"sync"

	appV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"

	"github.com/filswan/go-mcs-sdk/mcs/api/common/logs"
	networkingv1 "k8s.io/api/networking/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var k8sOnce sync.Once
var clientset *kubernetes.Clientset

type K8sService struct {
	k8sClient *kubernetes.Clientset
}

func NewK8sService() *K8sService {
	k8sOnce.Do(func() {
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
				logs.GetLogger().Errorf("Failed create k8s config, error: %v", err)
				return
			}
		}
		clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			logs.GetLogger().Errorf("Failed create k8s clientset, error: %v", err)
			return
		}
	})

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
			Name:      deploy.DeployName,
			Namespace: nameSpace,
		},
		Spec: appV1.DeploymentSpec{
			Selector: &metaV1.LabelSelector{
				MatchLabels: deploy.Label,
			},

			Template: coreV1.PodTemplateSpec{
				ObjectMeta: metaV1.ObjectMeta{
					Labels:    deploy.Label,
					Namespace: nameSpace,
				},

				Spec: coreV1.PodSpec{
					Containers: []coreV1.Container{{
						Name:            deploy.ContainerName,
						Image:           deploy.ImageName,
						ImagePullPolicy: coreV1.PullIfNotPresent,
						Ports: []coreV1.ContainerPort{{
							ContainerPort: deploy.ContainerPort,
						}},
						Resources: coreV1.ResourceRequirements{
							Limits: coreV1.ResourceList{
								//coreV1.ResourceCPU:    *resource.NewQuantity(deploy.Res.Cpu.Quantity, resource.DecimalSI),
								//coreV1.ResourceMemory: resource.MustParse(deploy.Res.Memory.Description),
								//coreV1.ResourceName("nvidia.com/gpu"): *resource.NewQuantity(deploy.Res.Gpu.Quantity, resource.DecimalSI),
							},
							Requests: coreV1.ResourceList{
								//coreV1.ResourceCPU:    *resource.NewQuantity(deploy.Res.Cpu.Quantity, resource.DecimalSI),
								//coreV1.ResourceMemory: resource.MustParse(deploy.Res.Memory.Description),
								//coreV1.ResourceName("nvidia.com/gpu"): *resource.NewQuantity(deploy.Res.Gpu.Quantity, resource.DecimalSI),
							},
						},
					}},
				},
			},
		}}

	return s.k8sClient.AppsV1().Deployments(nameSpace).Create(ctx, deployment, metaV1.CreateOptions{})
}

func (s *K8sService) DeleteDeployment(ctx context.Context, namespace, deploymentName string) error {
	return s.k8sClient.AppsV1().Deployments(namespace).Delete(ctx, deploymentName, metaV1.DeleteOptions{})
}

func (s *K8sService) DeletePod(ctx context.Context, namespace, spaceName string) error {
	return s.k8sClient.CoreV1().Pods(namespace).DeleteCollection(ctx, *metaV1.NewDeleteOptions(0), metaV1.ListOptions{
		LabelSelector: fmt.Sprintf("lad_app=%s", spaceName),
	})
}

func (s *K8sService) DeleteDeployRs(ctx context.Context, namespace, spaceName string) error {
	return s.k8sClient.AppsV1().ReplicaSets(namespace).DeleteCollection(ctx, *metaV1.NewDeleteOptions(0), metaV1.ListOptions{
		LabelSelector: fmt.Sprintf("lad_app=%s", spaceName),
	})
}

func (s *K8sService) GetDeploymentImages(ctx context.Context, namespace, deploymentName string) ([]string, error) {
	deployment, err := s.k8sClient.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metaV1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var imageIds []string
	for _, container := range deployment.Spec.Template.Spec.Containers {
		imageIds = append(imageIds, container.Image)
	}
	return imageIds, nil
}

func (s *K8sService) GetServiceByName(ctx context.Context, namespace, serviceName string, opts metaV1.GetOptions) (result *coreV1.Service, err error) {
	return s.k8sClient.CoreV1().Services(namespace).Get(ctx, serviceName, opts)
}

func (s *K8sService) CreateService(ctx context.Context, nameSpace, spaceName string, containerPort int32) (result *coreV1.Service, err error) {
	service := &coreV1.Service{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      constants.K8S_SERVICE_NAME_PREFIX + spaceName,
			Namespace: nameSpace,
		},
		Spec: coreV1.ServiceSpec{
			Ports: []coreV1.ServicePort{
				{
					Name: "http",
					Port: containerPort,
				},
			},
			Selector: map[string]string{
				"lad_app": spaceName,
			},
		},
	}
	return s.k8sClient.CoreV1().Services(nameSpace).Create(ctx, service, metaV1.CreateOptions{})
}

func (s *K8sService) DeleteService(ctx context.Context, namespace, serviceName string) error {
	return s.k8sClient.CoreV1().Services(namespace).Delete(ctx, serviceName, metaV1.DeleteOptions{})
}

func (s *K8sService) CreateIngress(ctx context.Context, k8sNameSpace, spaceName, hostName string, port int32) (*networkingv1.Ingress, error) {
	var ingressClassName = "nginx"
	ingress := &networkingv1.Ingress{
		ObjectMeta: metaV1.ObjectMeta{
			Name: constants.K8S_INGRESS_NAME_PREFIX + spaceName,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class":           "nginx",
				"nginx.ingress.kubernetes.io/use-regex": "true",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &ingressClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: hostName,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/*",
									PathType: func() *networkingv1.PathType { t := networkingv1.PathTypePrefix; return &t }(),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: constants.K8S_SERVICE_NAME_PREFIX + spaceName,
											Port: networkingv1.ServiceBackendPort{
												Number: port,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return s.k8sClient.NetworkingV1().Ingresses(k8sNameSpace).Create(ctx, ingress, metaV1.CreateOptions{})
}

func (s *K8sService) DeleteIngress(ctx context.Context, nameSpace, ingressName string) error {
	return s.k8sClient.NetworkingV1().Ingresses(nameSpace).Delete(ctx, ingressName, metaV1.DeleteOptions{})
}

func (s *K8sService) GetNodeList() (ip string, err error) {
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

func (s *K8sService) GetPods(namespace, spaceName string) (bool, error) {
	listOption := metaV1.ListOptions{}
	if spaceName != "" {
		listOption = metaV1.ListOptions{
			LabelSelector: fmt.Sprintf("lad_app=%s", spaceName),
		}
	}
	podList, err := s.k8sClient.CoreV1().Pods(namespace).List(context.TODO(), listOption)
	if err != nil {
		logs.GetLogger().Error(err)
		return false, err
	}
	if podList != nil && len(podList.Items) > 0 {
		return true, nil
	}
	return false, nil
}

func (s *K8sService) CreateNetworkPolicy(ctx context.Context, namespace string) (*networkingv1.NetworkPolicy, error) {
	networkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      namespace + "-" + generateString(4),
			Namespace: namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metaV1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "ingress-nginx",
								},
							},
						},
					},
				},
			},
		},
	}

	return s.k8sClient.NetworkingV1().NetworkPolicies(namespace).Create(ctx, networkPolicy, metaV1.CreateOptions{})
}

func (s *K8sService) CreateNameSpace(ctx context.Context, nameSpace *coreV1.Namespace, opts metaV1.CreateOptions) (result *coreV1.Namespace, err error) {
	return s.k8sClient.CoreV1().Namespaces().Create(ctx, nameSpace, opts)
}

func (s *K8sService) GetNameSpace(ctx context.Context, nameSpace string, opts metaV1.GetOptions) (result *coreV1.Namespace, err error) {
	return s.k8sClient.CoreV1().Namespaces().Get(ctx, nameSpace, opts)
}

func (s *K8sService) DeleteNameSpace(ctx context.Context, nameSpace string) error {
	return s.k8sClient.CoreV1().Namespaces().Delete(ctx, nameSpace, metaV1.DeleteOptions{})
}

func (s *K8sService) ListUsedImage(ctx context.Context, nameSpace string) ([]string, error) {
	list, err := s.k8sClient.CoreV1().Pods(nameSpace).List(ctx, metaV1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var usedImages []string
	for _, item := range list.Items {
		for _, status := range item.Status.ContainerStatuses {
			usedImages = append(usedImages, status.Image)
		}
	}
	return usedImages, nil
}

func (s *K8sService) ListNamespace(ctx context.Context) ([]string, error) {
	list, err := s.k8sClient.CoreV1().Namespaces().List(ctx, metaV1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var namespaces []string
	for _, item := range list.Items {
		namespaces = append(namespaces, item.Name)
	}
	return namespaces, nil
}
