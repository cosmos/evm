package config

import (
	"cosmossdk.io/math"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
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

	// DefaultBech32Prefix defines the default Bech32 address prefix
	DefaultBech32Prefix = "cosmos"
	// DefaultBech32PrefixAccAddr defines the default Bech32 prefix of an account's address.
	DefaultBech32PrefixAccAddr = DefaultBech32Prefix
	// DefaultBech32PrefixAccPub defines the default Bech32 prefix of an account's public key.
	DefaultBech32PrefixAccPub = DefaultBech32Prefix + sdk.PrefixPublic
	// DefaultBech32PrefixValAddr defines the default Bech32 prefix of a validator's operator address.
	DefaultBech32PrefixValAddr = DefaultBech32Prefix + sdk.PrefixValidator + sdk.PrefixOperator
	// DefaultBech32PrefixValPub defines the default Bech32 prefix of a validator's operator public key.
	DefaultBech32PrefixValPub = DefaultBech32Prefix + sdk.PrefixValidator + sdk.PrefixOperator + sdk.PrefixPublic
	// DefaultBech32PrefixConsAddr defines the default Bech32 prefix of a consensus node address.
	DefaultBech32PrefixConsAddr = DefaultBech32Prefix + sdk.PrefixValidator + sdk.PrefixConsensus
	// DefaultBech32PrefixConsPub defines the default Bech32 prefix of a consensus node public key.
	DefaultBech32PrefixConsPub = DefaultBech32Prefix + sdk.PrefixValidator + sdk.PrefixConsensus + sdk.PrefixPublic

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
	DefaultChainConfig = CreateChainConfig(DefaultChainID, DefaultEvmChainID, DefaultDisplayDenom, DefaultDecimals)
	// TwoDecimalsChainConfig provides a 2-decimal test configuration
	TwoDecimalsChainConfig = CreateChainConfig("ostwo-1", 9002, "test2", evmtypes.TwoDecimals)
	// SixDecimalsChainConfig provides a 6-decimal test configuration
	SixDecimalsChainConfig = CreateChainConfig("ossix-1", 9006, "test6", evmtypes.SixDecimals)
	// TwelveDecimalsChainConfig provides a 12-decimal test configuration
	TwelveDecimalsChainConfig = CreateChainConfig("ostwelve-1", 9012, "test12", evmtypes.TwelveDecimals)

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
