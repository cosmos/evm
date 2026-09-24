package statedb_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/vm/statedb"
	vmtracer "github.com/cosmos/evm/x/vm/tracer"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/testutil"
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
}
