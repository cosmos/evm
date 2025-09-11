package config

import (
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"
)

const (
	// DefaultGasPrice is the default gas price used for testing transactions
	DefaultGasPrice = 20

	// DefaultChainIDPrefix defines the default chain ID prefix for tests
	DefaultChainIDPrefix = "cosmos"
	// DefaultChainID defines the default chain ID for tests
	DefaultChainID = "cosmos-1"
	// DefaultEvmChainID defines the default EVM chain ID for tests
	DefaultEvmChainID = 9001
	// DefaultDisplayDenom defines the default display denom for use in tests
	DefaultDisplayDenom = "atom"
	// DefaultDecimals defines the default decimals used for creating denoms in tests
	DefaultDecimals = evmtypes.EighteenDecimals

	// DefaultWevmosContractMainnet is the default WEVMOS contract address for mainnet
	DefaultWevmosContractMainnet = "0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"
	// DefaultWevmosContractTestnet is the WEVMOS contract address for testnet
	DefaultWevmosContractTestnet = "0xcc491f589b45d4a3c679016195b3fb87d7848210"

	// ExampleEvmAddressAlice is an example EVM address
	EvmAddressAlice = "0x1e0DE5DB1a39F99cBc67B00fA3415181b3509e42"
	// EvmAddressBob is an example EVM address
	EvmAddressBob = "0x0AFc8e15F0A74E98d0AEC6C67389D2231384D4B2"
)

// Common test configurations for reuse
var (
	// DefaultChainConfig provides a standard 18-decimal cosmos/atom test configuration
	DefaultChainConfig = CreateChainConfig(DefaultChainID, DefaultEvmChainID, nil, DefaultDisplayDenom, DefaultDecimals, DefaultDecimals)
	// TwoDecimalsChainConfig provides a 2-decimal test configuration
	TwoDecimalsChainConfig = CreateChainConfig("ostwo-1", 9002, nil, "test2", evmtypes.TwoDecimals, evmtypes.EighteenDecimals)
	// SixDecimalsChainConfig provides a 6-decimal test configuration
	SixDecimalsChainConfig = CreateChainConfig("ossix-1", 9006, nil, "test6", evmtypes.SixDecimals, evmtypes.EighteenDecimals)
	// TwelveDecimalsChainConfig provides a 12-decimal test configuration
	TwelveDecimalsChainConfig = CreateChainConfig("ostwelve-1", 9012, nil, "test12", evmtypes.TwelveDecimals, evmtypes.EighteenDecimals)

	// DefaultTokenPairs defines a slice containing a pair for the native denom of the default chain
	DefaultTokenPairs = []erc20types.TokenPair{
		{
			Erc20Address:  DefaultWevmosContractMainnet,
			Denom:         evmtypes.CreateDenomStr(DefaultDecimals, DefaultDisplayDenom),
			Enabled:       true,
			ContractOwner: erc20types.OWNER_MODULE,
		},
	}

	// DefaultAllowances defines a slice containing an allowance for the native denom of the default chain
	DefaultAllowances = []erc20types.Allowance{
		{
			Erc20Address: DefaultWevmosContractMainnet,
			Owner:        EvmAddressAlice,
			Spender:      EvmAddressBob,
			Value:        math.NewInt(100),
		},
	}
)
