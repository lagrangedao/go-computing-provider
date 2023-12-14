package conf

import (
	"github.com/BurntSushi/toml"
	"path/filepath"
)

const (
	TokenContract      = "swan_token"
	CollateralContract = "swan_collateral"
	DefaultRpc         = "swan"
	BaseRpc            = "goerli"
)

type ChainConfig struct {
	RPC struct {
		GoerliUrl   string `toml:"GOERLI_URL"`
		SwanTestnet string `toml:"SWAN_TESTNET"`
		SwanMainnet string `toml:"SWAN_MAINNET"`
	} `toml:"RPC"`
	CONTRACT struct {
		SwanToken  string `toml:"SWAN_CONTRACT"`
		Collateral string `toml:"SWAN_COLLATERAL_CONTRACT"`
	} `toml:"CONTRACT"`
}

func GetContractAddressByName(name string) (string, error) {
	chain, err := loadConfig()
	if err != nil {
		return "", err
	}
	var rpc string
	switch name {
	case TokenContract:
		rpc = chain.CONTRACT.SwanToken
		break
	case CollateralContract:
		rpc = chain.CONTRACT.Collateral
		break
	}
	return rpc, nil
}

func GetRpcByName(rpcName string) (string, error) {
	chain, err := loadConfig()
	if err != nil {
		return "", err
	}
	var rpc string
	switch rpcName {
	case BaseRpc:
		rpc = chain.RPC.GoerliUrl
		break
	case DefaultRpc:
		rpc = chain.RPC.SwanTestnet
		break
	}
	return rpc, nil
}

func loadConfig() (*ChainConfig, error) {
	var chainConfig ChainConfig
	configFilePath := filepath.Join("", "config.toml")
	if _, err := toml.DecodeFile(configFilePath, &chainConfig); err != nil {
		return nil, err
	}
	return &chainConfig, nil
}
