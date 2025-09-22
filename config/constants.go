package config

import (
	serverflags "github.com/cosmos/evm/server/config"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

const ()

const (
	// DefaultGasPrice is the default gas price used for setting gas-related calculations
	DefaultGasPrice = 20
	// DefaultChainID is the default chain ID
	DefaultChainID = "cosmos"
	// DefaultEvmChainID defines the default EVM chain ID
	DefaultEvmChainID = serverflags.DefaultEVMChainID
	// DefaultDisplayDenom defines the display denom for the default EVM coin info
	DefaultDisplayDenom = "atom"
	// DefaultDecimals defines the decimals for the default EVM coin info
	DefaultDecimals = evmtypes.EighteenDecimals
	// DefaultExtendedDecimals defines the extended decimals for the default EVM coin info
	DefaultExtendedDecimals = evmtypes.EighteenDecimals
	// DefaultWevmosContractMainnet is the default WEVMOS contract address for mainnet
	DefaultWevmosContractMainnet = "0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"
)

var DefaultEvmCoinInfo = evmtypes.EvmCoinInfo{
	DisplayDenom:     DefaultDisplayDenom,
	Decimals:         DefaultDecimals,
	ExtendedDecimals: DefaultExtendedDecimals,
}
