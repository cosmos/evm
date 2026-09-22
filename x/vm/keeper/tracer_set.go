package keeper

import (
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"

	"github.com/cosmos/evm/x/vm/statedb"
	vmtracer "github.com/cosmos/evm/x/vm/tracer"
	"github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SetApplicationTracerFactories configures application-provided tracers for
// every top-level EVM execution. Cosmos EVM installs no application tracer
// unless the embedding application calls this method.
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
		var applicationTracer vmtracer.Tracer
		ctx, applicationTracer = factory(ctx, execution)
		manager.InstallApplication(applicationTracer)
	}
	return ctx, manager.Hooks()
}
