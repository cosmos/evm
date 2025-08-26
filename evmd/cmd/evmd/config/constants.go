package config

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

const (
	// ExampleChainDenom is the denomination of the Cosmos EVM example chain's base coin.
	ExampleChainDenom = "aatom"

	// ExampleDisplayDenom is the display denomination of the Cosmos EVM example chain's base coin.
	ExampleDisplayDenom = "atom"

	// Epix Chain Constants
	// EpixChainDenom is the denomination of the Epix chain's base coin (18 decimals).
	EpixChainDenom = "aepix"

	// EpixDisplayDenom is the display denomination of the Epix chain's base coin.
	EpixDisplayDenom = "epix"

	// EpixMainnetChainID is the chain ID for the Epix mainnet.
	EpixMainnetChainID = 1916

	// EpixTestnetChainID is the chain ID for the Epix testnet.
	EpixTestnetChainID = 1917

	// EighteenDecimalsChainID is the chain ID for the 18 decimals chain.
	EighteenDecimalsChainID = 9001

	// SixDecimalsChainID is the chain ID for the 6 decimals chain.
	SixDecimalsChainID = 9002

	// TwelveDecimalsChainID is the chain ID for the 12 decimals chain.
	TwelveDecimalsChainID = 9003

	// TwoDecimalsChainID is the chain ID for the 2 decimals chain.
	TwoDecimalsChainID = 9004

	CosmosChainID = 262144

	// TestChainID1 is test chain IDs for IBC E2E test
	TestChainID1 = 9005
	// TestChainID2 is test chain IDs for IBC E2E test
	TestChainID2 = 9006

	// WEVMOSContractMainnet is the WEVMOS contract address for mainnet
	WEVMOSContractMainnet = "0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"

	// WEPIXContractMainnet is the WEPIX contract address for mainnet
	WEPIXContractMainnet = "0x211781849EF6de72acbf1469Ce3808E74D7ce158"

	// WEPIXContractTestnet is the WEPIX contract address for testnet (same as mainnet due to deterministic generation)
	WEPIXContractTestnet = "0x211781849EF6de72acbf1469Ce3808E74D7ce158"
)

// GetWEPIXAddress returns the WEPIX contract address for the given chain ID
func GetWEPIXAddress(chainID int64) (common.Address, error) {
	switch chainID {
	case EpixMainnetChainID:
		return common.HexToAddress(WEPIXContractMainnet), nil
	case EpixTestnetChainID:
		return common.HexToAddress(WEPIXContractTestnet), nil
	default:
		return common.Address{}, fmt.Errorf("unsupported chain ID for WEPIX: %d", chainID)
	}
}
