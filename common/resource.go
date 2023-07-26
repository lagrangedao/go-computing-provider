package common

var HardwareResource = map[string]Resource{
	"C1ae.small": {
		Cpu: Specification{
			Quantity: 2,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 16,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 0,
			Unit:     "",
		},
	},
	"C1ae.medium": {
		Cpu: Specification{
			Quantity: 8,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 32,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 0,
			Unit:     "",
		},
	},
	"M1ae.small": {
		Cpu: Specification{
			Quantity: 4,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 16,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 2060",
		},
	},
	"M1ae.medium": {
		Cpu: Specification{
			Quantity: 8,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 32,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 2060",
		},
	},
	"M1ae.large": {
		Cpu: Specification{
			Quantity: 8,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 32,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 2080 Ti",
		},
	},
	"M2ae.small": {
		Cpu: Specification{
			Quantity: 4,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 16,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 3060 Ti",
		},
	},
	"M2ae.medium": {
		Cpu: Specification{
			Quantity: 8,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 32,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 3060 Ti",
		},
	},
	"M2ae.large": {
		Cpu: Specification{
			Quantity: 4,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 16,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 3070",
		},
	},
	"M2ae.xlarge": {
		Cpu: Specification{
			Quantity: 8,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 32,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 3070 Ti",
		},
	},
	"G1ae.small": {
		Cpu: Specification{
			Quantity: 4,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 16,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 3080",
		},
	},
	"G1ae.medium": {
		Cpu: Specification{
			Quantity: 8,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 32,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 3080",
		},
	},
	"G1ae.large": {
		Cpu: Specification{
			Quantity: 4,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 16,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 3080 Ti",
		},
	},
	"G1ae.xlarge": {
		Cpu: Specification{
			Quantity: 8,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 32,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 3080 Ti",
		},
	},
	"G2ae.small": {
		Cpu: Specification{
			Quantity: 4,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 16,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia T4",
		},
	},
	"G2ae.medium": {
		Cpu: Specification{
			Quantity: 8,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 32,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia T4",
		},
	},
	"G2ae.large": {
		Cpu: Specification{
			Quantity: 4,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 16,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia A10G",
		},
	},
	"G2ae.xlarge": {
		Cpu: Specification{
			Quantity: 8,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 32,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia A10G",
		},
	},
	"Hpc1ae.small": {
		Cpu: Specification{
			Quantity: 4,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 16,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 3090",
		},
	},
	"Hpc1ae.medium": {
		Cpu: Specification{
			Quantity: 8,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 32,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 3090",
		},
	},
	"Hpc1ae.large": {
		Cpu: Specification{
			Quantity: 4,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 16,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 3090 Ti",
		},
	},
	"Hpc1ae.xlarge": {
		Cpu: Specification{
			Quantity: 8,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 32,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 3090 Ti",
		},
	},
	"Hpc2ae.small": {
		Cpu: Specification{
			Quantity: 4,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 16,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 4090",
		},
	},
	"Hpc2ae.medium": {
		Cpu: Specification{
			Quantity: 8,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 32,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 4090",
		},
	},
	"Hpc2ae.large": {
		Cpu: Specification{
			Quantity: 4,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 16,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 4090 Ti",
		},
	},
	"Hpc2ae.xlarge": {
		Cpu: Specification{
			Quantity: 8,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 32,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia 4090 Ti",
		},
	},
	"P1ae.small": {
		Cpu: Specification{
			Quantity: 12,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 128,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia A100",
		},
	},
	"P1ae.medium": {
		Cpu: Specification{
			Quantity: 24,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 256,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia A100",
		},
	},
	"P1ae.large": {
		Cpu: Specification{
			Quantity: 12,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 128,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia H100",
		},
	},
	"P1ae.xlarge": {
		Cpu: Specification{
			Quantity: 24,
			Unit:     "vCPU",
		},
		Memory: Specification{
			Quantity: 256,
			Unit:     "GiB",
		},
		Gpu: Specification{
			Quantity: 1,
			Unit:     "Nvidia H100",
		},
	},
}

type Resource struct {
	Cpu    Specification
	Memory Specification
	Gpu    Specification
}

type Specification struct {
	Quantity int64
	Unit     string
}
