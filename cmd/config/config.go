package config

import (
	"github.com/cosmos/evm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SetBip44CoinType sets the global coin type to be used in hierarchical deterministic wallets.
func SetBip44CoinType(config *sdk.Config) {
	config.SetCoinType(types.Bip44CoinType)
	config.SetPurpose(sdk.Purpose)                  // Shared
	config.SetFullFundraiserPath(types.BIP44HDPath) //nolint: staticcheck
}
