package types

import (
	"fmt"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Failed returns if the contract execution failed in vm errors
func (egr EstimateGasResponse) Failed() bool {
	return len(egr.VmError) > 0
}

// Apply applies the state overrides to the given context's KVStore.
// This allows overriding specific keys in the Cosmos SDK state during EVM calls.
func (s *StoreStateDiff) Apply(ctx sdk.Context, storeKeys map[string]*storetypes.KVStoreKey) error {
	if s == nil {
		return nil
	}
	storeKey, exists := storeKeys[s.Name]
	if !exists {
		return fmt.Errorf("store key %s not found", s.Name)
	}
	kvStore := ctx.KVStore(storeKey)
	for _, entry := range s.Entries {
		if entry.Delete {
			kvStore.Delete(entry.Key)
		} else {
			kvStore.Set(entry.Key, entry.Value)
		}
	}
	return nil
}
