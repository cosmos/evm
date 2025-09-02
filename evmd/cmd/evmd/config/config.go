package config

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

const (
	// WEVMOSContractMainnet is the WEVMOS contract address for mainnet
	WEVMOSContractMainnet = "0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"
)

type ChainConfig struct {
	ChainInfo ChainInfo
	CoinInfo  evmtypes.EvmCoinInfo
}

type ChainInfo struct {
	ChainID    string
	EVMChainID uint64
}

// SetBip44CoinType sets the global coin type to be used in hierarchical deterministic wallets.
func SetBip44CoinType(config *sdk.Config) {
	config.SetCoinType(types.Bip44CoinType)
	config.SetPurpose(sdk.Purpose)                  // Shared
	config.SetFullFundraiserPath(types.BIP44HDPath) //nolint: staticcheck
}
