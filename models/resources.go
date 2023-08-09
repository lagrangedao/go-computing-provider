package models

type ClusterResource struct {
	NodeId      string          `json:"node_id"`
	Region      string          `json:"region"`
	ClusterInfo []*NodeResource `json:"cluster_info"`
}

type NodeResource struct {
	MachineId string `json:"machine_id"`
	Model     string `json:"model"`
	Cpu       Common `json:"cpu"`
	Vcpu      Common `json:"vcpu"`
	Memory    Common `json:"memory"`
	Gpu       Gpu    `json:"gpu"`
	Storage   Common `json:"storage"`
}

type Gpu struct {
	DriverVersion string      `json:"driver_version"`
	CudaVersion   string      `json:"cuda_version"`
	AttachedGpus  int         `json:"attached_gpus"`
	Details       []GpuDetail `json:"details"`
}

type GpuDetail struct {
	ProductName     string    `json:"product_name"`
	Status          GpuStatus `json:"status"`
	FbMemoryUsage   Common    `json:"fb_memory_usage"`
	Bar1MemoryUsage Common    `json:"bar1_memory_usage"`
}

type Common struct {
	Total string `json:"total"`
	Used  string `json:"used"`
	Free  string `json:"free"`
}

type ResourceStatus struct {
	Request  int64
	Capacity int64
}

type GpuStatus string

const (
	Occupied  GpuStatus = "occupied"
	Available GpuStatus = "available"

	DefaultGpuSize string = "0 MiB"
)
