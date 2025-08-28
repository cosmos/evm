package config

import (
	"github.com/cosmos/evm/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type ChainConfig struct {
	ChainInfo ChainInfo
	CoinInfo  evmtypes.EvmCoinInfo
}

type ChainInfo struct {
	ChainID    string
	EVMChainID uint64
}

// CreateChainConfig allows creating a test chain with custom parameters
func CreateChainConfig(chainID string, evmChainID uint64, displayDenom string, decimals evmtypes.Decimals) ChainConfig {
	// extended denom is always 18-decimals (atto) denom
	denom := evmtypes.CreateDenomStr(decimals, displayDenom)
	extendedDenom := evmtypes.CreateDenomStr(evmtypes.EighteenDecimals, displayDenom)

	return ChainConfig{
		ChainInfo: ChainInfo{
			ChainID:    chainID,
			EVMChainID: evmChainID,
		},
		CoinInfo: evmtypes.EvmCoinInfo{
			Denom:         denom,
			ExtendedDenom: extendedDenom,
			DisplayDenom:  displayDenom,
			Decimals:      decimals,
		},
	}
}

// SetBech32Prefixes sets the global prefixes to be used when serializing addresses and public keys to Bech32 strings.
func SetBech32Prefixes(config *sdk.Config) {
	config.SetBech32PrefixForAccount(DefaultBech32PrefixAccAddr, DefaultBech32PrefixAccPub)
	config.SetBech32PrefixForValidator(DefaultBech32PrefixValAddr, DefaultBech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(DefaultBech32PrefixConsAddr, DefaultBech32PrefixConsPub)
}

// SetBip44CoinType sets the global coin type to be used in hierarchical deterministic wallets.
func SetBip44CoinType(config *sdk.Config) {
	config.SetCoinType(types.Bip44CoinType)
	config.SetPurpose(sdk.Purpose)                  // Shared
	config.SetFullFundraiserPath(types.BIP44HDPath) //nolint: staticcheck
}
