package initializer

import (
	"fmt"
	"github.com/filswan/go-swan-lib/logs"
	"github.com/joho/godotenv"
	"go-computing-provider/computing"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

func LoadEnv() {
	err := godotenv.Load(".env")
	if err != nil {
		logs.GetLogger().Error(err)
	}

	logs.GetLogger().Info("name: ", os.Getenv("MCS_BUCKET"))
}

func sendHeartbeat(nodeId string) {
	// Replace the following URL with your Flask application's heartbeat endpoint URL
	heartbeatURL := os.Getenv("LAGRANGE_HOST") + "/cp/heartbeat"
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
	req.Header.Set("Authorization", "Bearer "+os.Getenv("LAGRANGE_API_TOKEN"))
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		logs.GetLogger().Errorf("Error sending heartbeat: %v", err)
	} else {
		body, err := ioutil.ReadAll(resp.Body)
		logs.GetLogger().Infof("Heartbeat sent. Status code: %d\n %s", resp.StatusCode, string(body))
		if err != nil {
			fmt.Println(err)
			return
		}
	}
}

func sendHeartbeats(nodeId string) {
	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		sendHeartbeat(nodeId)
	}
}
func ProjectInit() {
	LoadEnv()
	nodeID := computing.InitComputingProvider()
	// Start sending heartbeats
	go sendHeartbeats(nodeID)

}
