package mempool

import (
	"context"

	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ baseapp.TxSelector = &NoCopyProposalTxSelector{}

// NoCopyProposalTxSelector is the same as the baseapps defaultTxSelector,
// however this does not return a copy of the tx bytes via SelectedTxs, and
// instead directly returns the tx bytes.
type NoCopyProposalTxSelector struct {
	totalTxBytes uint64
	totalTxGas   uint64
	selectedTxs  [][]byte
}

func NewNoCopyProposalTxSelector() *NoCopyProposalTxSelector {
	return &NoCopyProposalTxSelector{}
}

func (ts *NoCopyProposalTxSelector) SelectedTxs(_ context.Context) [][]byte {
	return ts.selectedTxs
}

func (ts *NoCopyProposalTxSelector) Clear() {
	ts.totalTxBytes = 0
	ts.totalTxGas = 0
	ts.selectedTxs = nil
}

func (ts *NoCopyProposalTxSelector) SelectTxForProposal(_ context.Context, maxTxBytes, maxBlockGas uint64, memTx sdk.Tx, txBz []byte) bool {
	txSize := uint64(cmttypes.ComputeProtoSizeForTxs([]cmttypes.Tx{txBz}))

	var txGasLimit uint64
	if memTx != nil {
		if gasTx, ok := memTx.(baseapp.GasTx); ok {
			txGasLimit = gasTx.GetGas()
		}
	}

	// only add the transaction to the proposal if we have enough capacity
	if (txSize + ts.totalTxBytes) <= maxTxBytes {
		// If there is a max block gas limit, add the tx only if the limit has
		// not been met.
		if maxBlockGas > 0 {
			if (txGasLimit + ts.totalTxGas) <= maxBlockGas {
				ts.totalTxGas += txGasLimit
				ts.totalTxBytes += txSize
				ts.selectedTxs = append(ts.selectedTxs, txBz)
			}
		} else {
			ts.totalTxBytes += txSize
			ts.selectedTxs = append(ts.selectedTxs, txBz)
		}
	}

	// check if we've reached capacity; if so, we cannot select any more transactions
	return ts.totalTxBytes >= maxTxBytes || (maxBlockGas > 0 && (ts.totalTxGas >= maxBlockGas))
}
