package keeper

import (
	"context"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/vm/statedb"
	vmtracer "github.com/cosmos/evm/x/vm/tracer"
)

func TestPrepareCallEVMTracingReusesContextManager(t *testing.T) {
	k := Keeper{}
	ctx := sdk.Context{}.WithContext(context.Background()).WithEventManager(sdk.NewEventManager())
	stateDB := statedb.New(ctx, nil, statedb.NewEmptyTxConfig())
	manager := vmtracer.New(nil)
	ctx = vmtracer.WithManager(ctx, manager)
	manager.Hooks().OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	defer manager.Hooks().OnExit(0, nil, 0, nil, false)
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
	stateDBManager.Hooks().OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	defer stateDBManager.Hooks().OnExit(0, nil, 0, nil, false)
	stateDBCtx := vmtracer.WithManager(
		sdk.Context{}.WithContext(context.Background()).WithEventManager(sdk.NewEventManager()),
		stateDBManager,
	)
	stateDB := statedb.New(stateDBCtx, nil, statedb.NewEmptyTxConfig())

	callManager := vmtracer.New(nil)
	callManager.Hooks().OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	defer callManager.Hooks().OnExit(0, nil, 0, nil, false)
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

func TestPrepareCallEVMTracingReplacesInactiveManagers(t *testing.T) {
	k := Keeper{}
	ctx := sdk.Context{}.WithContext(context.Background()).WithEventManager(sdk.NewEventManager())
	staleManager := vmtracer.New(nil)
	ctx = vmtracer.WithManager(ctx, staleManager)
	stateDB := statedb.New(ctx, nil, statedb.NewEmptyTxConfig())

	var factoryManager *vmtracer.Manager
	factoryCalls := 0
	k.SetApplicationTracerFactories(func(factoryCtx sdk.Context, _ vmtracer.ExecutionInfo) (sdk.Context, vmtracer.Tracer) {
		factoryCalls++
		factoryManager, _ = vmtracer.FromContext(factoryCtx)
		return factoryCtx, nil
	})

	actualCtx, hooks := k.prepareCallEVMTracing(ctx, stateDB, core.Message{}, statedb.NewEmptyTxConfig(), true)

	actualManager, ok := vmtracer.FromContext(actualCtx)
	require.True(t, ok)
	require.NotSame(t, staleManager, actualManager)
	require.Same(t, factoryManager, actualManager)
	require.Same(t, actualManager.Hooks(), hooks)
	stateDBManager, ok := vmtracer.FromContext(stateDB.GetContext())
	require.True(t, ok)
	require.Same(t, actualManager, stateDBManager)
	require.Equal(t, 1, factoryCalls)
}

func TestPrepareCallEVMTracingUsesActiveContextManagerOverInactiveStateDBManager(t *testing.T) {
	k := Keeper{}
	ctx := sdk.Context{}.WithContext(context.Background()).WithEventManager(sdk.NewEventManager())
	stateDB := statedb.New(vmtracer.WithManager(ctx, vmtracer.New(nil)), nil, statedb.NewEmptyTxConfig())
	activeManager := vmtracer.New(nil)
	activeManager.Hooks().OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	defer activeManager.Hooks().OnExit(0, nil, 0, nil, false)
	ctx = vmtracer.WithManager(ctx, activeManager)

	actualCtx, hooks := k.prepareCallEVMTracing(ctx, stateDB, core.Message{}, statedb.NewEmptyTxConfig(), true)

	actualManager, ok := vmtracer.FromContext(actualCtx)
	require.True(t, ok)
	require.Same(t, activeManager, actualManager)
	require.Same(t, activeManager.NestedHooks(), hooks)
	stateDBManager, ok := vmtracer.FromContext(stateDB.GetContext())
	require.True(t, ok)
	require.Same(t, activeManager, stateDBManager)
}
