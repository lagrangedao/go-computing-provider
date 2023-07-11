package computing

import (
	"context"
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
		allocatedGPU int64
		allocatedCPU int64
		allocatedMem int64
	)

	var nodeResource = new(models.NodeResource)
	nodeResource.MachineId = node.Status.NodeInfo.MachineID

	gpuCount, ok := node.Labels[Nvidia_Gpu_Count]
	if ok {
		gpuCountInt, _ := strconv.Atoi(gpuCount)
		nodeResource.Gpu.TotalNums = gpuCountInt
	}

	var gpuMemoryInt int
	gpuMemory, ok := node.Labels[Nvidia_Gpu_Memory]
	if ok {
		gpuMemoryInt, _ = strconv.Atoi(gpuMemory)
		nodeResource.Gpu.TotalMemory = int64(gpuMemoryInt * nodeResource.Gpu.TotalNums)
	}

	for _, pod := range getPodsFromNode(allPods, node) {
		allocatedGPU += gpuInPod(&pod)
		allocatedCPU += cpuInPod(&pod)
		allocatedMem += memInPod(&pod)
	}

	nodeResource.Cpu.Model = node.Labels[Cpu_Model]
	nodeResource.Cpu.TotalNums = node.Status.Capacity.Cpu().Value()
	nodeResource.Cpu.AvailableNums = nodeResource.Cpu.TotalNums - allocatedCPU

	nodeResource.Memory.TotalMemory = node.Status.Capacity.Memory().Value()
	nodeResource.Memory.AvailableMemory = nodeResource.Memory.TotalMemory - allocatedMem

	nodeResource.Gpu.AvailableNums = nodeResource.Gpu.TotalNums - int(allocatedGPU)
	nodeResource.Gpu.AvailableMemory = int64(nodeResource.Gpu.AvailableNums * gpuMemoryInt)
	for i := 0; i < nodeResource.Gpu.AvailableNums; i++ {
		gpuModel, ok := node.Labels[Nvidia_Gpu_Product]
		if ok {
			nodeResource.Gpu.Details = append(nodeResource.Gpu.Details, models.GpuInfo{
				Model:           gpuModel,
				TotalMemory:     int64(gpuMemoryInt),
				AvailableMemory: int64(gpuMemoryInt),
			})
		}
	}
	for i := 0; i < int(allocatedGPU); i++ {
		gpuModel, ok := node.Labels[Nvidia_Gpu_Product]
		if ok {
			nodeResource.Gpu.Details = append(nodeResource.Gpu.Details, models.GpuInfo{
				Model:           gpuModel,
				TotalMemory:     int64(gpuMemoryInt),
				AvailableMemory: 0,
			})
		}
	}
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
