package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

const (
	GethVersion = "1.15.10"

	Dev0PrivateKey = "88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305" // dev0
	Dev1PrivateKey = "741de4f8988ea941d3ff0287911ca4074e62b7d45c991a51186455366f10b544" // dev1
	Dev2PrivateKey = "3b7955d25189c99a7468192fcbc6429205c158834053ebe3f78f4512ab432db9" // dev2
	Dev3PrivateKey = "8a36c69d940a92fcea94b36d0f2928c7a0ee19a90073eda769693298dfa9603b" // dev3
)

type Config struct {
	RpcEndpoint string `yaml:"rpc_endpoint"`
	RichPrivKey string `yaml:"rich_privkey"`
	// Timeout is the timeout for the RPC (e.g. 5s, 1m)
	Timeout string `yaml:"timeout"`
}

func (c *Config) Validate() error {
	if c.RpcEndpoint == "" {
		return fmt.Errorf("rpc_endpoint must be set")
	}
	if c.RichPrivKey == "" {
		return fmt.Errorf("rich_privkey must be set")
	}
	if _, err := time.ParseDuration(c.Timeout); err != nil {
		return fmt.Errorf("invalid timeout: %v", err)
	}
	return nil
}

func MustLoadConfig(filename string) *Config {
	var config Config
	file, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}
	err = yaml.Unmarshal(file, &config)
	if err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}

	if err = config.Validate(); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}
	return &config
}
