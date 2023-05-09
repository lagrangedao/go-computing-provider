package test

import (
	"context"
	"github.com/filswan/go-swan-lib/logs"
	"go-computing-provider/computing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestNewK8sService(t *testing.T) {
	service := computing.NewK8sService()
	podList, err := service.K8sClient.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logs.GetLogger().Error(err)
		return
	}
	for _, pod := range podList.Items {
		logs.GetLogger().Infof("name: %s, namespace: %s", pod.Name, pod.Namespace)
	}
}
