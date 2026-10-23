package keeper

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/core"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/vm/statedb"
	vmtracer "github.com/cosmos/evm/x/vm/tracer"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestPrepareCallEVMTracingReusesContextManager(t *testing.T) {
	k := Keeper{}
	ctx := sdk.Context{}.WithContext(context.Background()).WithEventManager(sdk.NewEventManager())
	stateDB := statedb.New(ctx, nil, statedb.NewEmptyTxConfig())
	manager := vmtracer.New(nil)
	ctx = vmtracer.WithManager(ctx, manager)
	factoryCalls := 0
	k.SetApplicationTracerFactories(func(factoryCtx sdk.Context, _ vmtracer.ExecutionInfo) (sdk.Context, vmtracer.Tracer) {
		factoryCalls++
		return factoryCtx, nil
	})

	actualCtx, hooks := k.prepareCallEVMTracing(ctx, stateDB, core.Message{}, statedb.NewEmptyTxConfig(), true)

	actualManager, ok := vmtracer.FromContext(actualCtx)
	require.True(t, ok)
	require.Same(t, manager, actualManager)
	require.Same(t, manager.NestedHooks(), hooks)
	stateDBManager, ok := vmtracer.FromContext(stateDB.GetContext())
	require.True(t, ok)
	require.Same(t, manager, stateDBManager)
	require.Zero(t, factoryCalls)
}

func TestPrepareCallEVMTracingPrefersStateDBManager(t *testing.T) {
	type contextKey string
	const callValueKey contextKey = "call-value"
	callValue := &struct{}{}

	k := Keeper{}
	stateDBManager := vmtracer.New(nil)
	stateDBCtx := vmtracer.WithManager(
		sdk.Context{}.WithContext(context.Background()).WithEventManager(sdk.NewEventManager()),
		stateDBManager,
	)
	stateDB := statedb.New(stateDBCtx, nil, statedb.NewEmptyTxConfig())

	callManager := vmtracer.New(nil)
	callCtx := vmtracer.WithManager(
		sdk.Context{}.WithContext(context.Background()).WithValue(callValueKey, callValue),
		callManager,
	)
	factoryCalls := 0
	k.SetApplicationTracerFactories(func(factoryCtx sdk.Context, _ vmtracer.ExecutionInfo) (sdk.Context, vmtracer.Tracer) {
		factoryCalls++
		return factoryCtx, nil
	})

	actualCtx, hooks := k.prepareCallEVMTracing(callCtx, stateDB, core.Message{}, statedb.NewEmptyTxConfig(), true)

	actualManager, ok := vmtracer.FromContext(actualCtx)
	require.True(t, ok)
	require.Same(t, stateDBManager, actualManager)
	require.NotSame(t, callManager, actualManager)
	require.Same(t, stateDBManager.NestedHooks(), hooks)
	require.Same(t, callValue, actualCtx.Value(callValueKey))
	require.Zero(t, factoryCalls)
}

func TestPrepareCallEVMTracingReusesManagerWithoutCommit(t *testing.T) {
	k := Keeper{}
	manager := vmtracer.New(nil)
	ctx := vmtracer.WithManager(
		sdk.Context{}.WithContext(context.Background()).WithEventManager(sdk.NewEventManager()),
		manager,
	)
	stateDB := statedb.New(ctx, nil, statedb.NewEmptyTxConfig())
	factoryCalls := 0
	k.SetApplicationTracerFactories(func(factoryCtx sdk.Context, _ vmtracer.ExecutionInfo) (sdk.Context, vmtracer.Tracer) {
		factoryCalls++
		return factoryCtx, nil
	})

	_, hooks := k.prepareCallEVMTracing(ctx, stateDB, core.Message{}, statedb.NewEmptyTxConfig(), false)

	require.Same(t, manager.NestedHooks(), hooks)
	require.Zero(t, factoryCalls)
}

func TestPrepareCallEVMTracingCreatesManagerWithoutCommit(t *testing.T) {
	k := Keeper{}
	ctx := sdk.Context{}.WithContext(context.Background()).WithEventManager(sdk.NewEventManager())
	stateDB := statedb.New(ctx, nil, statedb.NewEmptyTxConfig())
	factoryCalls := 0
	k.SetApplicationTracerFactories(func(factoryCtx sdk.Context, execution vmtracer.ExecutionInfo) (sdk.Context, vmtracer.Tracer) {
		factoryCalls++
		require.False(t, execution.Commit)
		return factoryCtx, nil
	})

	actualCtx, hooks := k.prepareCallEVMTracing(ctx, stateDB, core.Message{}, statedb.NewEmptyTxConfig(), false)

	require.NotNil(t, hooks)
	require.Equal(t, 1, factoryCalls)
	manager, ok := vmtracer.FromContext(actualCtx)
	require.True(t, ok)
	stateDBManager, ok := vmtracer.FromContext(stateDB.GetContext())
	require.True(t, ok)
	require.Same(t, manager, stateDBManager)
}
