package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// ContractRegistry holds deployed contract addresses for both networks
type ContractRegistry struct {
	EvmdERC20  common.Address `json:"evmd_erc20"`
	GethERC20  common.Address `json:"geth_erc20"`
	Timestamp  int64          `json:"timestamp"`
	DevAccount string         `json:"dev_account"` // Which account deployed the contracts
}

const ContractRegistryFile = "contract_registry.json"

// SaveContractAddresses saves the deployed contract addresses to a registry file
func SaveContractAddresses(evmdAddr, gethAddr common.Address, deployer string) error {
	registry := ContractRegistry{
		EvmdERC20:  evmdAddr,
		GethERC20:  gethAddr,
		Timestamp:  time.Now().Unix(),
		DevAccount: deployer,
	}

	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal contract registry: %w", err)
	}

	err = ioutil.WriteFile(ContractRegistryFile, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write contract registry: %w", err)
	}

	fmt.Printf("✓ Contract addresses saved to %s\n", ContractRegistryFile)
	fmt.Printf("  evmd: %s\n", evmdAddr.Hex())
	fmt.Printf("  geth: %s\n", gethAddr.Hex())

	return nil
}

// LoadContractAddresses loads the deployed contract addresses from the registry file
func LoadContractAddresses() (*ContractRegistry, error) {
	if _, err := os.Stat(ContractRegistryFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("contract registry file %s does not exist - run 'setup' first", ContractRegistryFile)
	}

	data, err := ioutil.ReadFile(ContractRegistryFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read contract registry: %w", err)
	}

	var registry ContractRegistry
	err = json.Unmarshal(data, &registry)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal contract registry: %w", err)
	}

	return &registry, nil
}

// GetContractAddresses is a convenience function to get both addresses
func GetContractAddresses() (evmdAddr, gethAddr common.Address, err error) {
	registry, err := LoadContractAddresses()
	if err != nil {
		return common.Address{}, common.Address{}, err
	}

	return registry.EvmdERC20, registry.GethERC20, nil
}