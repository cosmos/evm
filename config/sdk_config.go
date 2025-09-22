package config

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/types"
)

func SetupSdkConfig() {
	config := sdk.GetConfig()
	SetBech32Prefixes(config)
	config.Seal()
}

// SetBech32Prefixes sets the global prefixes to be used when serializing addresses and public keys to Bech32 strings.
func SetBech32Prefixes(config *sdk.Config) {
	config.SetBech32PrefixForAccount(sdk.Bech32PrefixAccAddr, sdk.Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(sdk.Bech32PrefixValAddr, sdk.Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(sdk.Bech32PrefixConsAddr, sdk.Bech32PrefixConsPub)
}

// SetBip44CoinType sets the global coin type to be used in hierarchical deterministic wallets.
func SetBip44CoinType(config *sdk.Config) {
	config.SetCoinType(types.Bip44CoinType)
	config.SetPurpose(sdk.Purpose)                  // Shared
	config.SetFullFundraiserPath(types.BIP44HDPath) //nolint: staticcheck
}
