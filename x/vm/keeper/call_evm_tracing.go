package keeper

import (
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"

	"github.com/cosmos/evm/x/vm/statedb"
	vmtracer "github.com/cosmos/evm/x/vm/tracer"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// prepareCallEVMTracing reuses the lifecycle of an enclosing EVM execution or
// prepares a new lifecycle for a top-level native EVM call.
func (k Keeper) prepareCallEVMTracing(
	ctx sdk.Context,
	stateDB *statedb.StateDB,
	msg core.Message,
	txConfig statedb.TxConfig,
	commit bool,
) (sdk.Context, *tracing.Hooks) {
	if stateDB == nil {
		return ctx, nil
	}
	if manager, ok := vmtracer.FromContext(stateDB.GetContext()); ok {
		ctx = vmtracer.WithManager(ctx, manager)
		return ctx, manager.NestedHooks()
	}

	if manager, ok := vmtracer.FromContext(ctx); ok {
		stateDB.AttachTracerManager(manager)
		return ctx, manager.NestedHooks()
	}

	ctx, hooks := k.prepareTracing(ctx, msg, txConfig, commit)
	manager, _ := vmtracer.FromContext(ctx)
	stateDB.AttachTracerManager(manager)
	return ctx, hooks
}
