package config

import (
	"crypto/ecdsa"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	GethVersion = "1.16.3"

	Dev0PrivateKey = "88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305" // dev0
	Dev1PrivateKey = "741de4f8988ea941d3ff0287911ca4074e62b7d45c991a51186455366f10b544" // dev1
	Dev2PrivateKey = "3b7955d25189c99a7468192fcbc6429205c158834053ebe3f78f4512ab432db9" // dev2
	Dev3PrivateKey = "8a36c69d940a92fcea94b36d0f2928c7a0ee19a90073eda769693298dfa9603b" // dev3

	EvmdHttpEndpoint = "http://localhost:8545"
	EvmdWsEndpoint   = "ws://localhost:8546"
	GethHttpEndpoint = "http://localhost:8547"
	GethWsEndpoint   = "ws://localhost:8548"
)

type Config struct {
	EvmdHttpEndpoint string `yaml:"evmd_http_endpoint"`
	EvmdWsEndpoint   string `yaml:"evmd_ws_endpoint"`
	GethHttpEndpoint string `yaml:"geth_http_endpoint"`
	GethWsEndpoint   string `yaml:"geth_ws_endpoint"`

	RichPrivKey string `yaml:"rich_privkey"`
	// Timeout is the timeout for the RPC (e.g. 5s, 1m)
	Timeout string `yaml:"timeout"`
}

func (c *Config) Validate() error {
	if c.EvmdHttpEndpoint == "" {
		return fmt.Errorf("rpc_endpoint must be set")
	}
	if c.EvmdWsEndpoint == "" {
		return fmt.Errorf("ws_endpoint must be set")
	}
	if c.GethHttpEndpoint == "" {
		return fmt.Errorf("geth_http_endpoint must be set")
	}
	if c.GethWsEndpoint == "" {
		return fmt.Errorf("geth_ws_endpoint must be set")
	}

	if c.RichPrivKey == "" {
		return fmt.Errorf("rich_privkey must be set")
	}
	if _, err := time.ParseDuration(c.Timeout); err != nil {
		return fmt.Errorf("invalid timeout: %v", err)
	}
	return nil
}

func MustLoadConfig() *Config {
	// Use environment variable if set, otherwise default to localhost
	evmdURL := os.Getenv("EVMD_URL")
	if evmdURL == "" {
		evmdURL = EvmdHttpEndpoint
	}

	gethURL := os.Getenv("GETH_URL")
	if gethURL == "" {
		gethURL = GethHttpEndpoint
	}

	// Handle WebSocket URLs - derive from HTTP URLs or use environment variables
	evmdWsURL := os.Getenv("EVMD_WS_URL")
	if evmdWsURL == "" {
		evmdWsURL = EvmdWsEndpoint
	}

	gethWsURL := os.Getenv("GETH_WS_URL")
	if gethWsURL == "" {
		gethWsURL = GethWsEndpoint
	}

	return &Config{
		EvmdHttpEndpoint: evmdURL,
		EvmdWsEndpoint:   evmdWsURL,
		GethHttpEndpoint: gethURL,
		GethWsEndpoint:   gethWsURL,
		RichPrivKey:      Dev0PrivateKey, // Default to dev0's private key
		Timeout:          "10s",
	}
}

// GetDev0PrivateKeyAndAddress returns dev0's private key and address for contract deployment
func GetDev0PrivateKeyAndAddress() (*ecdsa.PrivateKey, common.Address, error) {
	privateKey, err := crypto.HexToECDSA(Dev0PrivateKey)
	if err != nil {
		return nil, common.Address{}, err
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, common.Address{}, fmt.Errorf("error casting public key to ECDSA")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	return privateKey, address, nil
}
