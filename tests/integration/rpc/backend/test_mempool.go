package backend

import (
	"context"
	"math/big"
	"testing"

	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/mempool/txpool"
	txpoolmocks "github.com/cosmos/evm/mempool/txpool/mocks"
	"github.com/cosmos/evm/rpc/backend/mocks"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func NewEvmMempoolMock(t *testing.T) *mocks.Mempool {
	t.Helper()

	blockchain := txpoolmocks.NewBlockChain(t)
	blockchain.On("CurrentBlock").Return(&gethtypes.Header{Number: big.NewInt(1)}).Maybe()
	blockchain.On("StateAt", mock.Anything).Return(nil, nil).Maybe()
	blockchain.On("SubscribeChainHeadEvent", mock.Anything).Return(&mockMempoolSubscription{}).Maybe()
	blockchain.On("Config").Return(&params.ChainConfig{ChainID: big.NewInt(1)}).Maybe()

	// pool.Status() will return TxStatusUnknown
	pool, err := txpool.New(0, blockchain, nil, nil)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	mempool := mocks.NewMempool(t)
	mempool.On("GetTxPool").Return(pool).Maybe()

	return mempool
}

func RegisterMempoolInsert(t *testing.T, mempool *mocks.Mempool, expect sdk.Tx, err error) {
	t.Helper()

	doReturn := func(_ context.Context, actual sdk.Tx) error {
		if err == nil {
			requireTxEqual(t, expect, actual)
		}

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

type mockMempoolSubscription struct{}

func (ms *mockMempoolSubscription) Err() <-chan error { return make(chan error) }
func (ms *mockMempoolSubscription) Unsubscribe()      {}
