package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v2"
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
