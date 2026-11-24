package backend

import (
	"context"

	"github.com/cosmos/evm/mempool/txpool"
	rpctypes "github.com/cosmos/evm/rpc/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// NoOpMempool stands in for the app-side EVM mempool when the node runs
// without one (mempool.max-txs = -1): the txpool namespace reports an empty
// pool and tx submission is rejected with ErrMempoolDisabled.
type NoOpMempool struct {
	sdkmempool.NoOpMempool
}

var _ Mempool = NoOpMempool{}

// Insert rejects the tx with ErrMempoolDisabled.
func (NoOpMempool) Insert(context.Context, sdk.Tx) error {
	return rpctypes.ErrMempoolDisabled
}

// GetTxPool returns nil: there is no underlying EVM txpool.
func (NoOpMempool) GetTxPool() *txpool.TxPool {
	return nil
}
