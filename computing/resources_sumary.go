package computing

import (
	"context"
	"fmt"
	"github.com/lagrangedao/go-computing-provider/models"
	corev1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strconv"
)

const (
	Nvidia_Gpu_Product string = "nvidia.com/gpu.product"
	Nvidia_Gpu_Memory  string = "nvidia.com/gpu.memory"
	Nvidia_Gpu_Count   string = "nvidia.com/gpu.count"

	Nvidia_Gpu_Num string = "nvidia.com/gpu"
	Cpu_Model      string = "feature.node.kubernetes.io/cpu-model.vendor_id"
)

func allActivePods(clientSet *kubernetes.Clientset) ([]corev1.Pod, error) {
	allPods, err := clientSet.CoreV1().Pods("").List(context.TODO(), metaV1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, err
	}
	return allPods.Items, nil
}

func getNodeResource(allPods []corev1.Pod, node *corev1.Node) (*models.NodeResource, error) {
	var (
		allocatedCPU int64
		allocatedMem int64
	)

	var nodeResource = new(models.NodeResource)
	nodeResource.MachineId = node.Status.NodeInfo.MachineID

	for _, pod := range getPodsFromNode(allPods, node) {
		allocatedCPU += cpuInPod(&pod)
		allocatedMem += memInPod(&pod)
	}

	nodeResource.Cpu.Total = strconv.FormatInt(node.Status.Capacity.Cpu().Value(), 10)
	nodeResource.Cpu.Used = strconv.FormatInt(allocatedCPU, 10)
	nodeResource.Cpu.Free = strconv.FormatInt(node.Status.Capacity.Cpu().Value()-allocatedCPU, 10)

	nodeResource.Memory.Total = fmt.Sprintf("%.3f GiB", float64(node.Status.Capacity.Memory().Value()/1024/1024))
	nodeResource.Memory.Used = fmt.Sprintf("%.3f GiB", float64(allocatedMem/1024/1024))
	freeMemory := node.Status.Capacity.Memory().Value() - allocatedMem
	nodeResource.Memory.Free = fmt.Sprintf("%.3f GiB", float64(freeMemory/1024/1024))
	return nodeResource, nil
}

func getPodsFromNode(allPods []corev1.Pod, node *corev1.Node) (pods []corev1.Pod) {
	for _, pod := range allPods {
		if pod.Spec.NodeName == node.Name {
			pods = append(pods, pod)
		}
	}
	return pods
}

func gpuInPod(pod *corev1.Pod) (gpuCount int64) {
	containers := pod.Spec.Containers
	for _, container := range containers {
		val, ok := container.Resources.Limits[corev1.ResourceName(Nvidia_Gpu_Num)]
		if !ok {
			continue
		}
		gpuCount += val.Value()
	}
	return gpuCount
}

func cpuInPod(pod *corev1.Pod) (cpuCount int64) {
	containers := pod.Spec.Containers
	for _, container := range containers {
		val, ok := container.Resources.Requests[corev1.ResourceCPU]
		if !ok {
			continue
		}
		cpuCount += val.Value()
	}
	return cpuCount
}

func memInPod(pod *corev1.Pod) (memCount int64) {
	containers := pod.Spec.Containers
	for _, container := range containers {
		val, ok := container.Resources.Requests[corev1.ResourceMemory]
		if !ok {
			continue
		}
		memCount += val.Value()
	}
	return memCount
}

func GetNodeRole(node *corev1.Node) string {
	if _, ok := node.Labels[""]; ok {
		return "master"
	}
	return "node"
}
