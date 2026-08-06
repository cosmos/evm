package evmd

import (
	"sync/atomic"

	evmmempool "github.com/cosmos/evm/mempool"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ baseapp.ProposalTxVerifier = &SnapshotVerifiedTxVerifier{}

// SnapshotVerifiedTxVerifier re-runs ante over a proposal candidate only when
// the mempool cannot show it was validated at the height the proposal builds
// on. The base is set per proposal from the ABCI request, so a lagging
// recheck pin fails closed into re-verification.
type SnapshotVerifiedTxVerifier struct {
	*baseapp.BaseApp
	mempool *evmmempool.Mempool

	// proposalBase is the last committed height the in-flight proposal builds
	// on (req.Height - 1), set by the prepare-proposal handler before txs are
	// verified. Zero means unknown and re-verifies everything.
	proposalBase atomic.Int64
}

func NewSnapshotVerifiedTxVerifier(b *baseapp.BaseApp, mempool *evmmempool.Mempool) *SnapshotVerifiedTxVerifier {
	return &SnapshotVerifiedTxVerifier{BaseApp: b, mempool: mempool}
}

// SetProposalBase records the height the next proposal builds on.
func (txv *SnapshotVerifiedTxVerifier) SetProposalBase(height int64) {
	txv.proposalBase.Store(height)
}

// PrepareProposalVerifyTx encodes txs validated at the proposal's base height
// and defers to BaseApp's full ante verification for stale or unknown ones.
func (txv *SnapshotVerifiedTxVerifier) PrepareProposalVerifyTx(tx sdk.Tx) ([]byte, error) {
	if base := txv.proposalBase.Load(); base > 0 && txv.mempool.ProposalTxValidatedAt(tx, uint64(base)) {
		return txv.TxEncode(tx)
	}
	return txv.BaseApp.PrepareProposalVerifyTx(tx)
}
