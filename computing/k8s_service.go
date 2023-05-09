package computing

import (
	"flag"
	"github.com/filswan/go-swan-lib/logs"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
)

type K8sService struct {
	K8sClient *kubernetes.Clientset
}

func NewK8sService() *K8sService {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logs.GetLogger().Errorf("Failed create k8s clientset, error: %v", err)
		return nil
	}
	return &K8sService{
		K8sClient: clientset,
	}
}
