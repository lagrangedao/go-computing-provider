package computing

import (
	"context"
	"flag"
	"fmt"
	"github.com/lagrangedao/go-computing-provider/constants"
	"github.com/lagrangedao/go-computing-provider/models"
	"os"
	"path/filepath"
	"strings"
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

type K8sService struct {
	k8sClient *kubernetes.Clientset
	Version   string
}

func NewK8sService() *K8sService {
	var clientSet *kubernetes.Clientset
	var version string
	k8sOnce.Do(func() {
		config, err := rest.InClusterConfig()
		if err != nil {
			var kubeConfig *string
			if home := homedir.HomeDir(); home != "" {
				kubeConfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
			} else {
				kubeConfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
			}
			flag.Parse()
			config, err = clientcmd.BuildConfigFromFlags("", *kubeConfig)
			if err != nil {
				logs.GetLogger().Errorf("Failed create k8s config, error: %v", err)
				return
			}
		}
		clientSet, err = kubernetes.NewForConfig(config)
		if err != nil {
			logs.GetLogger().Errorf("Failed create k8s clientset, error: %v", err)
			return
		}

		versionInfo, err := clientSet.Discovery().ServerVersion()
		if err != nil {
			logs.GetLogger().Errorf("Failed get k8s version, error: %v", err)
			return
		}
		version = versionInfo.String()
	})

	return &K8sService{
		k8sClient: clientSet,
		Version:   version,
	}
}

func (s *K8sService) CreateDeployment(ctx context.Context, nameSpace string, deploy *appV1.Deployment) (result *appV1.Deployment, err error) {
	return s.k8sClient.AppsV1().Deployments(nameSpace).Create(ctx, deploy, metaV1.CreateOptions{})
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

func (s *K8sService) CreateConfigMap(ctx context.Context, k8sNameSpace, spaceName, basePath, configName string) (*coreV1.ConfigMap, error) {
	configFilePath := filepath.Join(basePath, configName)

	fileNameWithoutExt := filepath.Base(configName[:len(configName)-len(filepath.Ext(configName))])

	iniData, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}

	configMap := &coreV1.ConfigMap{
		ObjectMeta: metaV1.ObjectMeta{
			Name: spaceName + "-" + fileNameWithoutExt + "-" + generateString(4),
		},
		Data: map[string]string{
			configName: string(iniData),
		},
	}
	return s.k8sClient.CoreV1().ConfigMaps(k8sNameSpace).Create(ctx, configMap, metaV1.CreateOptions{})
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

func (s *K8sService) StatisticalSources(ctx context.Context) ([]*models.NodeResource, error) {
	activePods, err := allActivePods(s.k8sClient)
	if err != nil {
		return nil, err
	}
	var nodeList []*models.NodeResource

	nodes, err := s.k8sClient.CoreV1().Nodes().List(ctx, metaV1.ListOptions{})
	if err != nil {
		logs.GetLogger().Error(err)
		return nil, err
	}
	for _, node := range nodes.Items {
		nodeResource, err := getNodeResource(activePods, &node)
		if err != nil {
			logs.GetLogger().Error(err)
		}
		nodeList = append(nodeList, nodeResource)
	}
	return nodeList, nil
}

func generateLabel(name string) map[string]string {
	var labels = make(map[string]string)
	if strings.Contains(name, "NVIDIA") {
		labels["nvidia.com/gpu.product"] = name
	}
	return labels
}

func IsKubernetesVersionGreaterThan(version string, targetVersion string) bool {
	v1, err := parseKubernetesVersion(version)
	if err != nil {
		return false
	}

	v2, err := parseKubernetesVersion(targetVersion)
	if err != nil {
		return false
	}

	if v1.major > v2.major {
		return true
	} else if v1.major == v2.major && v1.minor > v2.minor {
		return true
	} else if v1.major == v2.major && v1.minor == v2.minor && v1.patch > v2.patch {
		return true
	}

	return false
}

type kubernetesVersion struct {
	major int
	minor int
	patch int
}

func parseKubernetesVersion(version string) (*kubernetesVersion, error) {
	v := &kubernetesVersion{}

	parts := strings.Split(strings.ReplaceAll(version, "v", ""), ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid version format")
	}

	_, err := fmt.Sscanf(parts[0], "%d", &v.major)
	if err != nil {
		return nil, fmt.Errorf("failed to parse major version")
	}

	_, err = fmt.Sscanf(parts[1], "%d", &v.minor)
	if err != nil {
		return nil, fmt.Errorf("failed to parse minor version")
	}

	_, err = fmt.Sscanf(parts[2], "%d", &v.patch)
	if err != nil {
		return nil, fmt.Errorf("failed to parse patch version")
	}

	return v, nil
}
