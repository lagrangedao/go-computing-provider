package initializer

import (
	"fmt"
	"github.com/lagrangedao/go-computing-provider/internal/computing"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/filswan/go-swan-lib/logs"
	"github.com/lagrangedao/go-computing-provider/conf"
	"github.com/lagrangedao/go-computing-provider/constants"
)

func sendHeartbeat(nodeId string) {
	// Replace the following URL with your Flask application's heartbeat endpoint URL
	heartbeatURL := conf.GetConfig().LAG.ServerUrl + "/cp/heartbeat"
	payload := strings.NewReader(fmt.Sprintf(`{
    "node_id": "%s",
    "status": "Active"
}`, nodeId))

	client := &http.Client{}
	req, err := http.NewRequest("POST", heartbeatURL, payload)
	if err != nil {
		logs.GetLogger().Errorf("Error creating request: %v", err)
		return
	}
	// Set the API token in the request header (replace "your_api_token" with the actual token)
	req.Header.Set("Authorization", "Bearer "+conf.GetConfig().LAG.AccessToken)
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		logs.GetLogger().Errorf("Error sending heartbeat, retrying to connect to the LAD server: %v", err)
		computing.Reconnect(nodeId)
	} else {
		_, err := ioutil.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			logs.GetLogger().Warningln("Retrying to connect to the LAD server")
			computing.Reconnect(nodeId)
		}
		if err != nil {
			fmt.Println(err)
			return
		}
	}
}

func sendHeartbeats(nodeId string) {
	ticker := time.NewTicker(15 * time.Second)
	for range ticker.C {
		sendHeartbeat(nodeId)
	}
}
func ProjectInit(cpRepoPath string) {
	if err := conf.InitConfig(cpRepoPath); err != nil {
		logs.GetLogger().Fatal(err)
	}
	nodeID := computing.InitComputingProvider(cpRepoPath)
	// Start sending heartbeats
	go sendHeartbeats(nodeID)

	go computing.NewScheduleTask().Run()

	computing.RunSyncTask(nodeID)
	celeryService := computing.NewCeleryService()
	celeryService.RegisterTask(constants.TASK_DEPLOY, computing.DeploySpaceTask)
	celeryService.Start()

}
