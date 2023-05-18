package common

var HardwareResource = map[string]Resource{
	"0": {
		Cpu: Specification{
			Description: "2 vCPU",
			Quantity:    2,
		},
		Memory: Specification{
			Description: "16GiB",
			Quantity:    16,
		},
	},
	"1": {
		Cpu: Specification{
			Description: "8 vCPU",
			Quantity:    8,
		},
		Memory: Specification{
			Description: "32GiB",
			Quantity:    32,
		},
	},
	"2": {
		Cpu: Specification{
			Description: "4 vCPU",
			Quantity:    4,
		},
		Memory: Specification{
			Description: "15GiB",
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
			Description: "30GiB",
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
			Description: "15GiB",
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
			Description: "46GiB",
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
			Description: "142GiB",
			Quantity:    142,
		},
		Gpu: Specification{
			Description: "Nvidia A100 40GB",
			Quantity:    1,
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
