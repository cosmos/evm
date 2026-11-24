package keeper

import (
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"

	"github.com/cosmos/evm/x/vm/statedb"
	vmtracer "github.com/cosmos/evm/x/vm/tracer"
	"github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
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
) (sdk.Context, *tracing.Hooks, error) {
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
) (sdk.Context, *tracing.Hooks, error) {
	if base == nil && len(k.applicationTracerFactories) == 0 {
		// A top-level execution must not inherit a previous execution's manager.
		if _, ok := vmtracer.FromContext(ctx); ok {
			ctx = vmtracer.WithoutManager(ctx)
		}
		return ctx, nil, nil
	}
	manager := vmtracer.New(base)
	ctx = vmtracer.WithManager(ctx, manager)
	execution := vmtracer.ExecutionInfo{
		Message: msg,
		TxHash:  txConfig.TxHash,
		TxIndex: uint64(txConfig.TxIndex),
		Commit:  commit,
	}
	for i, factory := range k.applicationTracerFactories {
		if factory == nil {
			continue
		}
		factoryCtx, executionTracer, err := factory(ctx, execution)
		if err != nil {
			return ctx, nil, errorsmod.Wrapf(err, "failed to initialize application tracer factory at index %d", i)
		}
		ctx = factoryCtx
		manager.InstallApplication(executionTracer)
	}
	return ctx, manager.Hooks(), nil
}
