package backend

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/rpc/backend/mocks"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func RegisterMempoolInsert(t *testing.T, mempool *mocks.Mempool, expect sdk.Tx, err error) {
	t.Helper()

	doReturn := func(_ context.Context, actual sdk.Tx) error {
		requireTxEqual(t, expect, actual)

		return err
	}

	mempool.On("Insert", mock.Anything, mock.Anything).Return(doReturn)
}

func requireTxEqual(t *testing.T, expected, actual sdk.Tx) {
	t.Helper()

	require.NotNil(t, actual, "tx mempool.Insert(ctx, tx) is nil")
	require.NotNil(t, expected, "expected tx is nil")

	require.Equal(t, len(expected.GetMsgs()), len(actual.GetMsgs()), "txs have different msgs")

	for i := range len(expected.GetMsgs()) {
		leftStr := expected.GetMsgs()[i].String()
		rightStr := actual.GetMsgs()[i].String()

		require.True(t, leftStr == rightStr, "tx.message %d is different", i)
	}
}
