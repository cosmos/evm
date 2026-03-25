package ics20

import (
	"testing"

	"github.com/stretchr/testify/require"

	callbackstypes "github.com/cosmos/evm/x/ibc/callbacks/types"

	storetypes "cosmossdk.io/store/types"

	sdktestutil "github.com/cosmos/cosmos-sdk/testutil"
)

func TestTransfer_BlockedDuringSourceCallbackExecution(t *testing.T) {
	storeKey := storetypes.NewKVStoreKey("test")
	tKey := storetypes.NewTransientStoreKey("test_t")
	ctx := sdktestutil.DefaultContext(storeKey, tKey)
	ctx = callbackstypes.WithSourceCallbackExecution(ctx)
	p := &Precompile{}
	_, err := p.Transfer(ctx, nil, nil, nil, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, ErrTransferBlockedInSourceCallback)
}
