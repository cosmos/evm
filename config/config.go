package config

import (
	"github.com/cosmos/evm/config/eips"
	"github.com/cosmos/evm/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultChainConfig provides a standard 18-decimal cosmos/atom test configuration
var (
	DefaultChainConfig = NewChainConfig(
		DefaultChainID,
		DefaultEvmChainID,
		eips.CosmosEVMActivators,
		nil,
		nil,
		evmtypes.EvmCoinInfo{
			DisplayDenom:     DisplayDenom,
			Decimals:         Decimals,
			ExtendedDecimals: ExtendedDecimals,
		},
		false,
	)
)

// ChainsCoinInfo is a map of the chain id and its corresponding EvmCoinInfo
// that allows initializing the app with different coin info based on the
// chain id
var ChainsCoinInfo = map[uint64]evmtypes.EvmCoinInfo{
	EighteenDecimalsChainID: {
		DisplayDenom:     ExampleDisplayDenom,
		Decimals:         evmtypes.EighteenDecimals,
		ExtendedDecimals: evmtypes.EighteenDecimals,
	},
	// SixDecimalsChainID provides a chain ID which is being set up with 6 decimals
	SixDecimalsChainID: {
		DisplayDenom:     "test",
		Decimals:         evmtypes.SixDecimals,
		ExtendedDecimals: evmtypes.EighteenDecimals,
	},
	// EVMChainID provides a chain ID used for internal testing
	DefaultEvmChainID: {
		DisplayDenom:     "test",
		Decimals:         evmtypes.EighteenDecimals,
		ExtendedDecimals: evmtypes.EighteenDecimals,
	},
}

const (
	// Bech32Prefix defines the Bech32 prefix used for accounts on the exemplary Cosmos EVM blockchain.
	Bech32Prefix = "cosmos"
	// Bech32PrefixAccAddr defines the Bech32 prefix of an account's address.
	Bech32PrefixAccAddr = Bech32Prefix
	// Bech32PrefixAccPub defines the Bech32 prefix of an account's public key.
	Bech32PrefixAccPub = Bech32Prefix + sdk.PrefixPublic
	// Bech32PrefixValAddr defines the Bech32 prefix of a validator's operator address.
	Bech32PrefixValAddr = Bech32Prefix + sdk.PrefixValidator + sdk.PrefixOperator
	// Bech32PrefixValPub defines the Bech32 prefix of a validator's operator public key.
	Bech32PrefixValPub = Bech32Prefix + sdk.PrefixValidator + sdk.PrefixOperator + sdk.PrefixPublic
	// Bech32PrefixConsAddr defines the Bech32 prefix of a consensus node address.
	Bech32PrefixConsAddr = Bech32Prefix + sdk.PrefixValidator + sdk.PrefixConsensus
	// Bech32PrefixConsPub defines the Bech32 prefix of a consensus node public key.
	Bech32PrefixConsPub = Bech32Prefix + sdk.PrefixValidator + sdk.PrefixConsensus + sdk.PrefixPublic
	// DisplayDenom defines the denomination displayed to users in client applications.
	DisplayDenom = "atom"
	// BaseDenom defines to the default denomination used in the Cosmos EVM example chain.
	BaseDenom = "aatom"
	// Decimals defines the precision of the base denomination.
	Decimals         = evmtypes.EighteenDecimals
	ExtendedDecimals = evmtypes.EighteenDecimals
	// DefaultEvmChainID defines the EIP-155 replay-protection chain id for the current ethereum chain config.
	DefaultEvmChainID = 262144
	// ExampleChainDenom is the denomination of the Cosmos EVM example chain's base coin.
	ExampleChainDenom = "aatom"
	// ExampleDisplayDenom is the display denomination of the Cosmos EVM example chain's base coin.
	ExampleDisplayDenom = "atom"
	// EighteenDecimalsChainID is the chain ID for the 18 decimals chain.
	EighteenDecimalsChainID = 9001
	// SixDecimalsChainID is the chain ID for the 6 decimals chain.
	SixDecimalsChainID = 9002
	// TwelveDecimalsChainID is the chain ID for the 12 decimals chain.
	TwelveDecimalsChainID = 9003
	// TwoDecimalsChainID is the chain ID for the 2 decimals chain.
	TwoDecimalsChainID = 9004
	DefaultChainID     = "cosmos"
	CosmosChainID      = 262144
	// TestChainID1 is test chain IDs for IBC E2E test
	TestChainID1 = 9005
	// TestChainID2 is test chain IDs for IBC E2E test
	TestChainID2 = 9006
	// WEVMOSContractMainnet is the WEVMOS contract address for mainnet
	WEVMOSContractMainnet = "0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"
)

// SetBech32Prefixes sets the global prefixes to be used when serializing addresses and public keys to Bech32 strings.
func SetBech32Prefixes(config *sdk.Config) {
	config.SetBech32PrefixForAccount(Bech32PrefixAccAddr, Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(Bech32PrefixValAddr, Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(Bech32PrefixConsAddr, Bech32PrefixConsPub)
}

// SetBip44CoinType sets the global coin type to be used in hierarchical deterministic wallets.
func SetBip44CoinType(config *sdk.Config) {
	config.SetCoinType(types.Bip44CoinType)
	config.SetPurpose(sdk.Purpose)                  // Shared
	config.SetFullFundraiserPath(types.BIP44HDPath) //nolint: staticcheck
}

// NewTestChainConfig creates a fresh chain config for testing purposes
// This avoids the issue of shared EvmConfig instances being sealed
func NewTestChainConfig(evmChainID uint64) ChainConfig {
	coinInfo := evmtypes.EvmCoinInfo{
		DisplayDenom:     DisplayDenom,
		Decimals:         Decimals,
		ExtendedDecimals: ExtendedDecimals,
	}

	return NewChainConfig(
		DefaultChainID,
		evmChainID,
		eips.CosmosEVMActivators,
		nil,
		nil,
		coinInfo,
		true, // reset = true for testing
	)
}
