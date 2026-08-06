package evmd

import (
	evmmempool "github.com/cosmos/evm/mempool"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ baseapp.ProposalTxVerifier = &SnapshotVerifiedTxVerifier{}

// SnapshotVerifiedTxVerifier re-runs ante over a proposal candidate only when
// the mempool cannot show it was already validated at the current head —
// under backlog the cosmos pool serves a snapshot carried from an earlier
// height, and only those carried entries may since have become invalid.
type SnapshotVerifiedTxVerifier struct {
	*baseapp.BaseApp
	mempool *evmmempool.Mempool
}

func NewSnapshotVerifiedTxVerifier(b *baseapp.BaseApp, mempool *evmmempool.Mempool) *SnapshotVerifiedTxVerifier {
	return &SnapshotVerifiedTxVerifier{BaseApp: b, mempool: mempool}
}

// PrepareProposalVerifyTx encodes txs validated at the head height and defers
// to BaseApp's full ante verification for stale or unknown ones.
func (txv *SnapshotVerifiedTxVerifier) PrepareProposalVerifyTx(tx sdk.Tx) ([]byte, error) {
	if txv.mempool.ProposalTxValidatedAtHead(tx) {
		return txv.TxEncode(tx)
	}
	return txv.BaseApp.PrepareProposalVerifyTx(tx)
}
