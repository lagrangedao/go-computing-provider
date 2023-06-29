package computing

import (
	corev1 "k8s.io/api/core/v1"
)

//func GetNodeResource(allPods []corev1.Pod, node *corev1.Node) (resource map[string]model.ResourceStatus, err error) {
//	var (
//		allocatedGPU int64
//		allocatedCPU int64
//		allocatedMem int64
//		capacityGPU  int64
//	)
//	resource = map[string]model.ResourceStatus{}
//	val, ok := node.Status.Capacity[common.NvidiaGPUResource]
//	if !ok {
//		capacityGPU = 0
//	} else {
//		capacityGPU = val.Value()
//	}
//	for _, pod := range getPodsFromNode(allPods, node) {
//		allocatedGPU += gpuInPod(&pod)
//		allocatedCPU += cpuInPod(&pod)
//		allocatedMem += memInPod(&pod)
//	}
//	resource[common.ResourceGPU] = model.ResourceStatus{
//		Request:  allocatedGPU,
//		Capacity: capacityGPU,
//	}
//	resource[common.ResourceCPU] = model.ResourceStatus{
//		Request:  allocatedCPU,
//		Capacity: node.Status.Capacity.Cpu().Value(),
//	}
//	resource[common.ResourceMemory] = model.ResourceStatus{
//		Request:  allocatedMem,
//		Capacity: node.Status.Capacity.Memory().Value(),
//	}
//	return resource, nil
//}
//
//func getPodsFromNode(allPods []corev1.Pod, node *corev1.Node) (pods []corev1.Pod) {
//	for _, pod := range allPods {
//		if pod.Spec.NodeName == node.Name {
//			pods = append(pods, pod)
//		}
//	}
//	return pods
//}
//
//func AllActivePods(clientSet *kubernetes.Clientset) ([]corev1.Pod, error) {
//	allPods, err := clientSet.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
//		FieldSelector: "status.phase=Running",
//	})
//	if err != nil {
//		log.Infof("get pods err, %s", err)
//		return nil, err
//	}
//	log.Infof("Running pod count %d", len(allPods.Items))
//	return allPods.Items, nil
//}
//
//func gpuInPod(pod *corev1.Pod) (gpuCount int64) {
//	containers := pod.Spec.Containers
//	for _, container := range containers {
//		val, ok := container.Resources.Limits[common.NvidiaGPUResource]
//		if !ok {
//			continue
//		}
//		gpuCount += val.Value()
//	}
//	return gpuCount
//}
//
//func cpuInPod(pod *corev1.Pod) (cpuCount int64) {
//	containers := pod.Spec.Containers
//	for _, container := range containers {
//		val, ok := container.Resources.Requests[common.ResourceCPU]
//		if !ok {
//			continue
//		}
//		cpuCount += val.Value()
//	}
//	return cpuCount
//}
//
//func memInPod(pod *corev1.Pod) (memCount int64) {
//	containers := pod.Spec.Containers
//	for _, container := range containers {
//		val, ok := container.Resources.Requests[common.ResourceMemory]
//		if !ok {
//			continue
//		}
//		memCount += val.Value()
//	}
//	return memCount
//}

func GetNodeRole(node *corev1.Node) string {
	if _, ok := node.Labels[""]; ok {
		return "master"
	}
	return "node"
}
