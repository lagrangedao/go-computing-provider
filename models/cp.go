package models

type ComputingProvider struct {
	Name          string `json:"name"`
	NodeId        string `json:"node_id"`
	MultiAddress  string `json:"multi_address"`
	Autobid       int    `json:"autobid"`
	WalletAddress int    `json:"wallet_address"`
}

type JobData struct {
	UUID          string `json:"uuid"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	Duration      int    `json:"duration"`
	Hardware      string `json:"hardware"`
	JobSourceURI  string `json:"job_source_uri"`
	JobResultURI  string `json:"job_result_uri"`
	StorageSource string `json:"storage_source"`
	TaskUUID      string `json:"task_uuid"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}
