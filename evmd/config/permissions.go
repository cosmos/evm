package config

import (
	"maps"
	"sort"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	poatypes "github.com/cosmos/cosmos-sdk/enterprise/poa/x/poa/types"
	cosmosevmutils "github.com/cosmos/evm/utils"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	vmtypes "github.com/cosmos/evm/x/vm/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	corevm "github.com/ethereum/go-ethereum/core/vm"
)

// BlockedAddresses returns all the app's blocked account addresses.
//
// Note, this includes:
//   - module accounts
//   - Ethereum's native precompiled smart contracts
//   - Cosmos EVM' available static precompiled contracts
func BlockedAddresses() map[string]bool {
	blockedAddrs := make(map[string]bool)

	maccPerms := GetMaccPerms()
	accs := make([]string, 0, len(maccPerms))
	for acc := range maccPerms {
		accs = append(accs, acc)
	}
	sort.Strings(accs)

	for _, acc := range accs {
		blockedAddrs[authtypes.NewModuleAddress(acc).String()] = true
	}

	blockedPrecompilesHex := vmtypes.AvailableStaticPrecompiles
	for _, addr := range corevm.PrecompiledAddressesPrague {
		blockedPrecompilesHex = append(blockedPrecompilesHex, addr.Hex())
	}

	for _, precompile := range blockedPrecompilesHex {
		blockedAddrs[cosmosevmutils.Bech32StringFromHexAddress(precompile)] = true
	}

	return blockedAddrs
}

// module account permissions
var maccPerms = map[string][]string{
	authtypes.FeeCollectorName: nil,
	poatypes.ModuleName:        nil,
	transfertypes.ModuleName:   {authtypes.Minter, authtypes.Burner},
	govtypes.ModuleName:        {authtypes.Burner},

	// Cosmos EVM modules
	vmtypes.ModuleName:          {authtypes.Minter, authtypes.Burner},
	feemarkettypes.ModuleName:   nil,
	erc20types.ModuleName:       {authtypes.Minter, authtypes.Burner},
	precisebanktypes.ModuleName: {authtypes.Minter, authtypes.Burner},
}

// GetMaccPerms returns a copy of the module account permissions
func GetMaccPerms() map[string][]string {
	return maps.Clone(maccPerms)
}
