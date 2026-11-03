package keeper

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"

	"github.com/cosmos/evm/x/vm/statedb"
	vmtracer "github.com/cosmos/evm/x/vm/tracer"
	"github.com/cosmos/evm/x/vm/types"
)

// SetApplicationTracerFactories configures application-provided tracers that are
// installed for every top-level EVM execution. Cosmos EVM does not install a
// application tracer unless the embedding application calls this method.
func (k *Keeper) SetApplicationTracerFactories(factories ...vmtracer.ApplicationTracerFactory) *Keeper {
	k.applicationTracerFactories = append([]vmtracer.ApplicationTracerFactory(nil), factories...)
	return k
}

func (k Keeper) prepareTracing(
	ctx sdk.Context,
	msg core.Message,
	txConfig statedb.TxConfig,
	commit bool,
) (sdk.Context, *tracing.Hooks) {
	var base *tracing.Hooks
	if k.tracer != "" {
		base = k.Tracer(ctx, msg, types.GetEthChainConfig())
	}
	return k.prepareTracingWithBase(ctx, msg, txConfig, commit, base)
}

// prepareTracingWithBase preserves an explicitly selected tracer (such as a
// debug RPC tracer) while installing the application's application hooks.
func (k Keeper) prepareTracingWithBase(
	ctx sdk.Context,
	msg core.Message,
	txConfig statedb.TxConfig,
	commit bool,
	base *tracing.Hooks,
) (sdk.Context, *tracing.Hooks) {
	manager := vmtracer.New(base)
	ctx = vmtracer.WithManager(ctx, manager)
	execution := vmtracer.ExecutionInfo{
		Message: msg,
		TxHash:  txConfig.TxHash,
		TxIndex: uint64(txConfig.TxIndex),
		Commit:  commit,
	}
	for _, factory := range k.applicationTracerFactories {
		if factory == nil {
			continue
		}
		var executionTracer vmtracer.Tracer
		ctx, executionTracer = factory(ctx, execution)
		manager.InstallApplication(executionTracer)
	}
	return ctx, manager.Hooks()
}
