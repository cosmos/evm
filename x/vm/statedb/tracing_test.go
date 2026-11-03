package statedb_test

import (
	"testing"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/testutil"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/vm/statedb"
	vmtracer "github.com/cosmos/evm/x/vm/tracer"
	vmglobal "github.com/cosmos/evm/x/vm/tracer/application"
)

func TestAttachTracerManager(t *testing.T) {
	type contextKey string
	const baseValueKey contextKey = "base-value"
	baseValue := &struct{}{}

	key := storetypes.NewKVStoreKey("execution-context")
	tkey := storetypes.NewTransientStoreKey("execution-context-transient")
	ctx := sdktestutil.DefaultContext(key, tkey).WithValue(baseValueKey, baseValue)
	manager := vmtracer.New(nil)

	db := statedb.New(ctx, NewMockKeeper(), emptyTxConfig)
	baseStore := db.GetContext().MultiStore()
	baseEvents := db.GetContext().EventManager()
	baseGasMeter := db.GetContext().GasMeter()

	cacheCtx, err := db.GetCacheContext()
	require.NoError(t, err)
	cacheStore := cacheCtx.MultiStore()
	cacheEvents := cacheCtx.EventManager()
	cacheGasMeter := cacheCtx.GasMeter()

	db.AttachTracerManager(manager)

	actualManager, ok := vmtracer.FromContext(db.GetContext())
	require.True(t, ok)
	require.Same(t, manager, actualManager)
	require.Same(t, baseValue, db.GetContext().Value(baseValueKey))
	require.Same(t, baseStore, db.GetContext().MultiStore())
	require.Same(t, baseEvents, db.GetContext().EventManager())
	require.Same(t, baseGasMeter, db.GetContext().GasMeter())

	cacheCtx, err = db.GetCacheContext()
	require.NoError(t, err)
	actualManager, ok = vmtracer.FromContext(cacheCtx)
	require.True(t, ok)
	require.Same(t, manager, actualManager)
	require.Same(t, baseValue, cacheCtx.Value(baseValueKey))
	require.Same(t, cacheStore, cacheCtx.MultiStore())
	require.Same(t, cacheEvents, cacheCtx.EventManager())
	require.Same(t, cacheGasMeter, cacheCtx.GasMeter())

	db.AttachTracerManager(nil)
	_, ok = vmtracer.FromContext(db.GetContext())
	require.False(t, ok)
	require.Same(t, baseValue, db.GetContext().Value(baseValueKey))
	cacheCtx, err = db.GetCacheContext()
	require.NoError(t, err)
	_, ok = vmtracer.FromContext(cacheCtx)
	require.False(t, ok)
	require.Same(t, cacheStore, cacheCtx.MultiStore())
}

func TestStateDBObservationsPreserveRevertedFacts(t *testing.T) {
	ctx := newTestCtx()
	db := statedb.New(ctx, NewMockKeeper(), emptyTxConfig)
	manager := vmtracer.New(nil)
	collector := vmglobal.NewCollector()
	manager.InstallApplication(collector)
	manager.SetCodeReader(db.GetCode)
	db.AttachTracerManager(manager)
	manager.Hooks().OnEnter(0, byte(vm.CALL), address, address2, nil, 100, nil)
	snapshot := db.Snapshot()
	db.AddBalance(address, uint256.NewInt(10), tracing.BalanceChangeTransfer)
	db.SubBalance(address, uint256.NewInt(3), tracing.BalanceChangeTransfer)
	db.SetBalance(address, uint256.NewInt(2), tracing.BalanceChangeUnspecified)
	log := &ethtypes.Log{
		Address: address,
		Data:    []byte{7},
	}
	db.AddLog(log)
	db.RevertToSnapshot(snapshot)
	manager.Hooks().OnExit(0, nil, 0, vm.ErrExecutionReverted, true)
	require.True(t, db.GetBalance(address).IsZero())
	require.Empty(t, db.Logs())
	facts := collector.Observations()
	require.Len(t, facts, 6)
	require.Equal(t, vmtracer.BalanceChange, facts[1].Kind)
	require.Equal(t, int64(0), facts[1].PreviousBalance.Int64())
	require.Equal(t, int64(10), facts[1].CurrentBalance.Int64())
	require.Equal(t, int64(7), facts[2].CurrentBalance.Int64())
	require.Equal(t, int64(2), facts[3].CurrentBalance.Int64())
	require.Equal(t, vmtracer.Log, facts[4].Kind)
	require.Equal(t, uint64(1), facts[4].CallID)
	require.True(t, facts[4].Status.Reverted)
	log.Data[0] = 99
	require.Equal(t, byte(7), facts[4].Log.Data[0])
	require.NoError(t, manager.ExecutionError())
}
