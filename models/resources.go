package models

type ClusterResource struct {
	NodeId      string          `json:"node_id"`
	Region      string          `json:"region"`
	ClusterInfo []*NodeResource `json:"cluster_info"`
}

type NodeResource struct {
	MachineId string       `json:"machine_id"`
	Cpu       CpuModel     `json:"cpu"`
	Memory    MemoryModel  `json:"memory"`
	Gpu       GpuModel     `json:"gpu"`
	Storage   StorageModel `json:"storage"`
}

type CpuModel struct {
	Model         string `json:"model"`
	TotalNums     int64  `json:"total_nums"`
	AvailableNums int64  `json:"available_nums"`
}

type GpuModel struct {
	Model           string `json:"model"`
	TotalNums       int    `json:"total_nums"`
	AvailableNums   int    `json:"available_nums"`
	TotalMemory     int64  `json:"total_memory"`
	AvailableMemory int64  `json:"available_memory"`
}

type MemoryModel struct {
	TotalMemory     int64 `json:"total_memory"`
	AvailableMemory int64 `json:"available_memory"`
}

type StorageModel struct {
	Type          string `json:"type"`
	TotalSize     int64  `json:"total_size"`
	AvailableSize int64  `json:"available_size"`
}

type ResourceStatus struct {
	Request  int64
	Capacity int64
}
