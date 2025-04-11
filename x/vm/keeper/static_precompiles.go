package keeper

import (
	"fmt"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/evm/x/vm/types"
)

// WithStaticPrecompiles sets the available static precompiled contracts.
func (k *Keeper) WithStaticPrecompiles(precompiles map[common.Address]vm.PrecompiledContract) *Keeper {
	if k.precompiles != nil {
		panic("available precompiles map already set")
	}

	if len(precompiles) == 0 {
		panic("empty precompiled contract map")
	}

	k.precompiles = precompiles
	return k
}

// GetStaticPrecompileInstance returns the instance of the given static precompile address.
func (k *Keeper) GetStaticPrecompileInstance(params *types.Params, address common.Address) (vm.PrecompiledContract, bool, error) {
	if k.IsAvailableStaticPrecompile(params, address) {
		precompile, found := k.precompiles[address]
		// If the precompile is within params but not found in the precompiles map it means we have memory
		// corruption.
		if !found {
			panic(fmt.Errorf("precompiled contract not stored in memory: %s", address))
		}
		return precompile, true, nil
	}
	return nil, false, nil
}

// IsAvailablePrecompile returns true if the given static precompile address is contained in the
// EVM keeper's available precompiles map.
// This function assumes that the Berlin precompiles cannot be disabled.
func (k Keeper) IsAvailableStaticPrecompile(params *types.Params, address common.Address) bool {
	return slices.Contains(params.ActiveStaticPrecompiles, address.String()) ||
		slices.Contains(vm.PrecompiledAddressesBerlin, address)
}
