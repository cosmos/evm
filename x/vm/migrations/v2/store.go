package v2

import (
	"slices"

	"github.com/cosmos/evm/x/vm/keeper"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const vestingPrecompileAddress = "0x0000000000000000000000000000000000000803"

// MigrateStore removes the legacy vesting precompile from active EVM params.
func MigrateStore(ctx sdk.Context, k *keeper.Keeper) error {
	params := k.GetParams(ctx)
	activePrecompiles := slices.DeleteFunc(params.ActiveStaticPrecompiles, func(address string) bool {
		return address == vestingPrecompileAddress
	})
	if len(activePrecompiles) == len(params.ActiveStaticPrecompiles) {
		return nil
	}

	params.ActiveStaticPrecompiles = activePrecompiles
	return k.SetParams(ctx, params)
}
