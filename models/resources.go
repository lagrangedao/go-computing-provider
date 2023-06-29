package models

type NodeResource struct {
	Cpu          CpuModel
	MemoryTotal  int64
	StorageTotal int64
	Gpu          GpuModel
}

type GpuModel struct {
	TypeName string
	Count    int
	Memory   int
}

type CpuModel struct {
	TypeName string
	Count    int64
}

type MemoryModel struct {
	Total int64
}
