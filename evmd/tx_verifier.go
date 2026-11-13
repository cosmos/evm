package evmd

import (
	"sync"
	"sync/atomic"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ baseapp.ProposalTxVerifier = &SnapshotVerifiedTxVerifier{}

// ProposalSnapshot reports whether the mempool validated a proposal candidate
// at the height the proposal builds on (see Mempool.ProposalTxValidatedAt).
type ProposalSnapshot interface {
	ProposalTxValidatedAt(tx sdk.Tx, base uint64) bool
}

// SnapshotVerifiedTxVerifier re-runs ante over a proposal candidate only when
// the mempool cannot show it was validated at the proposal's base height, set
// per proposal from the ABCI request so a lagging pin fails closed.
//
// Encoding skips the ante effects (sequence bump, fee deduction) that a later
// stale tx of the same signer needs, so encoded cosmos txs are queued and
// verified ahead of the next stale one. Proposals without stale txs pay
// nothing; a mixed one pays once per encoded tx, bounded by block capacity.
type SnapshotVerifiedTxVerifier struct {
	// ProposalTxVerifier runs full verification and the tx codec; evmd wires the BaseApp.
	baseapp.ProposalTxVerifier
	snapshot ProposalSnapshot
	logger   log.Logger

	// proposalBase is the height the in-flight proposal builds on
	// (req.Height - 1). Zero means unknown and re-verifies everything.
	proposalBase atomic.Int64

	// encoded holds cosmos txs encoded without verification, in selection
	// order, until their ante effects reach the prepare-proposal state.
	mu      sync.Mutex
	encoded []sdk.Tx
}

func NewSnapshotVerifiedTxVerifier(base baseapp.ProposalTxVerifier, snapshot ProposalSnapshot, logger log.Logger) *SnapshotVerifiedTxVerifier {
	return &SnapshotVerifiedTxVerifier{
		ProposalTxVerifier: base,
		snapshot:           snapshot,
		logger:             logger.With(log.ModuleKey, "SnapshotVerifiedTxVerifier"),
	}
}

// SetProposalBase records the height the next proposal builds on. Its queue
// starts empty: the prepare-proposal state is rebuilt per proposal.
func (txv *SnapshotVerifiedTxVerifier) SetProposalBase(height int64) {
	txv.proposalBase.Store(height)
	txv.mu.Lock()
	txv.encoded = nil
	txv.mu.Unlock()
}

// PrepareProposalVerifyTx encodes txs validated at the proposal's base height,
// and fully verifies stale or unknown ones after landing the ante effects of
// the txs encoded before them.
func (txv *SnapshotVerifiedTxVerifier) PrepareProposalVerifyTx(tx sdk.Tx) ([]byte, error) {
	if base := txv.proposalBase.Load(); base > 0 && txv.snapshot.ProposalTxValidatedAt(tx, uint64(base)) {
		bz, err := txv.TxEncode(tx)
		if err == nil && !isEVMTx(tx) {
			txv.mu.Lock()
			txv.encoded = append(txv.encoded, tx)
			txv.mu.Unlock()
		}
		return bz, err
	}
	txv.verifyEncoded()
	return txv.ProposalTxVerifier.PrepareProposalVerifyTx(tx)
}

// verifyEncoded replays the queued txs through full verification so their ante
// effects reach the prepare-proposal state. They passed at base, so a failure
// is only logged: the stale tx behind it then fails on its own and is skipped.
func (txv *SnapshotVerifiedTxVerifier) verifyEncoded() {
	txv.mu.Lock()
	queued := txv.encoded
	txv.encoded = nil
	txv.mu.Unlock()

	for _, tx := range queued {
		if _, err := txv.ProposalTxVerifier.PrepareProposalVerifyTx(tx); err != nil {
			txv.logger.Warn("encoded proposal tx failed verification at the proposal base", "err", err)
		}
	}
}

// isEVMTx reports whether tx carries a single MsgEthereumTx. Such txs are
// never queued: the reserver keeps an address in one pool at a time, so they
// never precede a stale cosmos tx of the same signer.
func isEVMTx(tx sdk.Tx) bool {
	msgs := tx.GetMsgs()
	if len(msgs) != 1 {
		return false
	}
	_, ok := msgs[0].(*evmtypes.MsgEthereumTx)
	return ok
}
