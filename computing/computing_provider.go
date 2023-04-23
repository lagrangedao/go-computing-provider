package computing

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/filswan/go-swan-lib/logs"
	"go-computing-provider/models"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func updateProviderInfo(nodeID string, peerID string, address string) {
	// Replace the following URL with your Flask application's /cp endpoint URL
	updateURL := "http://localhost:5002/cp"

	data := url.Values{
		"name":          {"Provider Local"},
		"node_id":       {nodeID},
		"multi_address": {"/ip4/127.0.0.1/tcp/8085"},
		"autobid":       {"1"},
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", updateURL, strings.NewReader(data.Encode()))
	if err != nil {
		logs.GetLogger().Errorf("Error creating request: %v", err)
		return
	}

	// Set the content type and API token in the request header
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("LAGRANGE_API_TOKEN"))

	resp, err := client.Do(req)
	if err != nil {
		logs.GetLogger().Errorf("Error updating provider info: %v", err)
	} else {
		logs.GetLogger().Infof("Provider info sent. Status code: %d\n", resp.StatusCode)
		if resp.StatusCode == 400 {
			logs.GetLogger().Info(resp.Body)
		}

		err := resp.Body.Close()
		if err != nil {
			logs.GetLogger().Errorf(err.Error())
			return
		}
	}
}
func InitComputingProvider() string {
	nodeID, peerID, address := generateNodeID()

	logs.GetLogger().Infof("Node ID :%s Peer ID:%s address:%s",
		nodeID,
		peerID, address)
	updateProviderInfo(nodeID, peerID, address)
	return nodeID
}
func generateNodeID() (string, string, string) {
	privateKeyPath := ".swan_node/private_key"
	var privateKeyBytes []byte

	if _, err := os.Stat(privateKeyPath); err == nil {
		privateKeyBytes, err = os.ReadFile(privateKeyPath)
		if err != nil {
			log.Fatalf("Error reading private key: %v", err)
		}
		log.Printf("Found key in %s", privateKeyPath)
	} else {
		log.Printf("Created key in %s", privateKeyPath)
		privateKeyBytes = make([]byte, 32)
		_, err := rand.Read(privateKeyBytes)
		if err != nil {
			log.Fatalf("Error generating random key: %v", err)
		}

		err = os.MkdirAll(filepath.Dir(privateKeyPath), os.ModePerm)
		if err != nil {
			log.Fatalf("Error creating directory for private key: %v", err)
		}

		err = os.WriteFile(privateKeyPath, privateKeyBytes, 0644)
		if err != nil {
			log.Fatalf("Error writing private key: %v", err)
		}
	}

	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		log.Fatalf("Error converting private key bytes: %v", err)
	}
	nodeID := hex.EncodeToString(crypto.FromECDSAPub(&privateKey.PublicKey))
	peerID := hashPublicKey(&privateKey.PublicKey)
	address := crypto.PubkeyToAddress(privateKey.PublicKey).String()
	return nodeID, peerID, address
}

func hashPublicKey(publicKey *ecdsa.PublicKey) string {
	publicKeyBytes := crypto.FromECDSAPub(publicKey)
	hash := sha256.Sum256(publicKeyBytes)
	return hex.EncodeToString(hash[:])
}
func GetServiceProviderInfo() *models.HostInfo {
	info := new(models.HostInfo)
	//info.SwanProviderVersion = common.GetVersion()
	info.OperatingSystem = runtime.GOOS
	info.Architecture = runtime.GOARCH
	info.CPUCores = runtime.NumCPU()
	return info
}
