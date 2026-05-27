package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/testutil"
)

func TestSourceCallbackExecutionContextMarker(t *testing.T) {
	storeKey := storetypes.NewKVStoreKey("test")
	tKey := storetypes.NewTransientStoreKey("test_t")
	ctx := sdktestutil.DefaultContext(storeKey, tKey)
	require.False(t, IsSourceCallbackExecution(ctx))
	callbackCtx := WithSourceCallbackExecution(ctx)
	require.True(t, IsSourceCallbackExecution(callbackCtx))
	require.False(t, IsSourceCallbackExecution(ctx))
}
