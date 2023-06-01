package common

//var HardwareResource = map[string]Resource{
//	"0": {
//		Cpu: Specification{
//			Description: "2 vCPU",
//			Quantity:    2,
//		},
//		Memory: Specification{
//			Description: "16Gi",
//			Quantity:    16,
//		},
//	},
//	"1": {
//		Cpu: Specification{
//			Description: "8 vCPU",
//			Quantity:    8,
//		},
//		Memory: Specification{
//			Description: "32Gi",
//			Quantity:    32,
//		},
//	},
//	"2": {
//		Cpu: Specification{
//			Description: "4 vCPU",
//			Quantity:    4,
//		},
//		Memory: Specification{
//			Description: "15Gi",
//			Quantity:    15,
//		},
//		Gpu: Specification{
//			Description: "Nvidia T4",
//			Quantity:    1,
//		},
//	},
//	"3": {
//		Cpu: Specification{
//			Description: "8 vCPU",
//			Quantity:    8,
//		},
//		Memory: Specification{
//			Description: "30Gi",
//			Quantity:    30,
//		},
//		Gpu: Specification{
//			Description: "Nvidia T4",
//			Quantity:    1,
//		},
//	},
//	"4": {
//		Cpu: Specification{
//			Description: "4 vCPU",
//			Quantity:    4,
//		},
//		Memory: Specification{
//			Description: "15Gi",
//			Quantity:    15,
//		},
//		Gpu: Specification{
//			Description: "Nvidia A10G",
//			Quantity:    1,
//		},
//	},
//	"5": {
//		Cpu: Specification{
//			Description: "12 vCPU",
//			Quantity:    12,
//		},
//		Memory: Specification{
//			Description: "46Gi",
//			Quantity:    46,
//		},
//		Gpu: Specification{
//			Description: "Nvidia A10G",
//			Quantity:    1,
//		},
//	},
//	"6": {
//		Cpu: Specification{
//			Description: "12 vCPU",
//			Quantity:    12,
//		},
//		Memory: Specification{
//			Description: "142Gi",
//			Quantity:    142,
//		},
//		Gpu: Specification{
//			Description: "Nvidia A100 40GB",
//			Quantity:    1,
//		},
//	},
//	"7": {
//		Cpu: Specification{
//			Description: "0 vCPU",
//			Quantity:    0,
//		},
//		Memory: Specification{
//			Description: "0Gi",
//			Quantity:    0,
//		},
//		Gpu: Specification{
//			Description: "Nvidia A100 40GB",
//			Quantity:    0,
//		},
//	},
//}

var HardwareResource = map[string]Resource{
	"0": {
		Cpu: Specification{
			Description: "2 vCPU",
			Quantity:    2,
		},
		Memory: Specification{
			Description: "8Gi",
			Quantity:    8,
		},
	},
	"1": {
		Cpu: Specification{
			Description: "8 vCPU",
			Quantity:    4,
		},
		Memory: Specification{
			Description: "4Gi",
			Quantity:    4,
		},
	},
	"2": {
		Cpu: Specification{
			Description: "4 vCPU",
			Quantity:    4,
		},
		Memory: Specification{
			Description: "15Gi",
			Quantity:    15,
		},
		Gpu: Specification{
			Description: "Nvidia T4",
			Quantity:    1,
		},
	},
	"3": {
		Cpu: Specification{
			Description: "8 vCPU",
			Quantity:    8,
		},
		Memory: Specification{
			Description: "30Gi",
			Quantity:    30,
		},
		Gpu: Specification{
			Description: "Nvidia T4",
			Quantity:    1,
		},
	},
	"4": {
		Cpu: Specification{
			Description: "4 vCPU",
			Quantity:    4,
		},
		Memory: Specification{
			Description: "15Gi",
			Quantity:    15,
		},
		Gpu: Specification{
			Description: "Nvidia A10G",
			Quantity:    1,
		},
	},
	"5": {
		Cpu: Specification{
			Description: "12 vCPU",
			Quantity:    12,
		},
		Memory: Specification{
			Description: "46Gi",
			Quantity:    46,
		},
		Gpu: Specification{
			Description: "Nvidia A10G",
			Quantity:    1,
		},
	},
	"6": {
		Cpu: Specification{
			Description: "12 vCPU",
			Quantity:    12,
		},
		Memory: Specification{
			Description: "142Gi",
			Quantity:    142,
		},
		Gpu: Specification{
			Description: "Nvidia A100 40GB",
			Quantity:    1,
		},
	},
	"7": {
		Cpu: Specification{
			Description: "0 vCPU",
			Quantity:    0,
		},
		Memory: Specification{
			Description: "0Gi",
			Quantity:    0,
		},
		Gpu: Specification{
			Description: "Nvidia A100 40GB",
			Quantity:    0,
		},
	},
}

type Resource struct {
	Cpu    Specification
	Memory Specification
	Gpu    Specification
}

type Specification struct {
	Description string
	Quantity    int64
}
