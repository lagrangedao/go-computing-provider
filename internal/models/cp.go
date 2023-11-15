package models

type BidStatus string

const (
	BidDisabledStatus    BidStatus = "bidding_disabled"
	BidEnabledStatus     BidStatus = "bidding_enabled"
	BidGpuDisabledStatus BidStatus = "bidding_gpu_disabled"

	ActiveStatus   string = "Active"
	InactiveStatus string = "Inactive"
)

type ComputingProvider struct {
	Name          string `json:"name"`
	NodeId        string `json:"node_id"`
	MultiAddress  string `json:"multi_address"`
	Autobid       int    `json:"autobid"`
	WalletAddress int    `json:"wallet_address"`
	Status        string `json:"status"`
}

type JobData struct {
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Duration int    `json:"duration"`
	//Hardware      string `json:"hardware"`
	JobSourceURI  string `json:"job_source_uri"`
	JobResultURI  string `json:"job_result_uri"`
	StorageSource string `json:"storage_source"`
	TaskUUID      string `json:"task_uuid"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	BuildLog      string `json:"build_log"`
	ContainerLog  string `json:"container_log"`
}

type Job struct {
	Uuid   string
	Status JobStatus
	Url    string
	Count  int
}

type JobStatus string

const (
	JobDownloadSource JobStatus = "downloadSource" // download file form job_resource_uri
	JobUploadResult   JobStatus = "uploadResult"   // upload task result to mcs
	JobBuildImage     JobStatus = "buildImage"     // build images
	JobPushImage      JobStatus = "pushImage"      // push image to registry
	JobPullImage      JobStatus = "pullImage"      // download file form job_resource_uri
	JobDeployToK8s    JobStatus = "deployToK8s"    // deploy image to k8s
)

type DeleteJobReq struct {
	CreatorWallet string `json:"creator_wallet"`
	SpaceName     string `json:"space_name"`
}

type SpaceJSON struct {
	Data struct {
		Files []SpaceFile `json:"files"`
		Owner struct {
			PublicAddress string `json:"public_address"`
		} `json:"owner"`
		Space struct {
			Uuid        string `json:"uuid"`
			Name        string `json:"name"`
			ActiveOrder struct {
				Config SpaceHardware `json:"config"`
			} `json:"activeOrder"`
		} `json:"space"`
	} `json:"data"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

type SpaceFile struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type SpaceHardware struct {
	Description  string `json:"description"`
	HardwareType string `json:"hardware_type"`
	Memory       int    `json:"memory"`
	Name         string `json:"name"`
	Vcpu         int    `json:"vcpu"`
}

type Resource struct {
	Cpu     Specification
	Memory  Specification
	Gpu     Specification
	Storage Specification
}

type Specification struct {
	Quantity int64
	Unit     string
}

type CacheSpaceDetail struct {
	WalletAddress string
	SpaceName     string
	SpaceUuid     string
	ExpireTime    int64
	JobUuid       string
	TaskType      string
	DeployName    string
	Hardware      string
	Url           string
}
