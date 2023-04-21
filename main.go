package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"github.com/sirupsen/logrus"
	"go-computing-provider/models"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
)

var log = logrus.New()

func generateNodeID() (string, string) {
	privateKeyPath := ".swan_node/private_key"
	var privateKeyBytes []byte

	if _, err := os.Stat(privateKeyPath); err == nil {
		privateKeyBytes, err = ioutil.ReadFile(privateKeyPath)
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

		err = ioutil.WriteFile(privateKeyPath, privateKeyBytes, 0644)
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
	return nodeID, peerID
}

func hashPublicKey(publicKey *ecdsa.PublicKey) string {
	publicKeyBytes := crypto.FromECDSAPub(publicKey)
	hash := sha256.Sum256(publicKeyBytes)
	return hex.EncodeToString(hash[:])
}
func getSwanServiceProviderInfo() *models.HostInfo {
	info := new(models.HostInfo)
	//info.SwanProviderVersion = common.GetVersion()
	info.OperatingSystem = runtime.GOOS
	info.Architecture = runtime.GOARCH
	info.CPUCores = runtime.NumCPU()

	return info
}
func main() {
	log.SetFormatter(&logrus.TextFormatter{})
	log.SetLevel(logrus.InfoLevel)

	hostInfo := getSwanServiceProviderInfo()
	hostInfoJSON, err := json.Marshal(hostInfo)
	if err != nil {
		log.Fatalf("Failed to marshal host info: %v", err)
	}
	log.Info("Swan Service Provider Host Info: ", string(hostInfoJSON))

	gin.SetMode(gin.DebugMode)
	router := gin.Default()

	// Register your routes here
	// ...

	err = router.Run(":8085")
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
