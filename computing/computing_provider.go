package computing

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/filswan/go-swan-lib/logs"
	"go-computing-provider/models"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

func InitComputingProvider() {
	nodeID, peerID, address := generateNodeID()

	logs.GetLogger().Infof("Node ID :%s Peer ID:%s address:%s",
		nodeID,
		peerID, address,
	)
}
func generateNodeID() (string, string, string) {
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
