package constants

const StatusActive = "Active"
const StatusOffline = "Offline"

// bidding status
const BiddingCreated string = "created"
const BiddingAccepting string = "accepting_bids"
const BiddingProcessing string = "processing"
const BiddingSubmitted string = "submitted"
const BiddingCompleted string = "completed"
const BiddingCancelled string = "cancelled"

const TASK_DEPLOY string = "worker.deploy"

const K8S_NAMESPACE_NAME_PREFIX = "ns-"
const K8S_CONTAINER_NAME_PREFIX = "worker-"
const K8S_SERVICE_NAME_PREFIX = "svc-"
const K8S_DEPLOY_NAME_PREFIX = "deploy-"
const REDIS_PORT_PREFIX = "port:"
const REDIS_FULL_PREFIX = "FULL:"

const MIN_PORT = 32750
const MAX_PORT = 32755
