package evmd

import (
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ baseapp.ProposalTxVerifier = &NoCheckProposalTxVerifier{}

type NoCheckProposalTxVerifier struct {
	*baseapp.BaseApp
}

func NewNoCheckProposalTxVerifier(b *baseapp.BaseApp) *NoCheckProposalTxVerifier {
	return &NoCheckProposalTxVerifier{BaseApp: b}
}

// PrepareProposalVerifyTx overrides the typical tx verification done in
// BaseApp's PrepareProposalHandler. The default PrepareProposalVerifyTx
// implementation encodes the tx to bytes, then calls runTx in 'checktx' mode,
// executing all antehandlers.
//
// We now override the implementation to only verify that the tx can be encoded
// to bytes, since we will guarantee that all txs selected are valid elsewhere.
func (txv *NoCheckProposalTxVerifier) PrepareProposalVerifyTx(tx sdk.Tx) ([]byte, error) {
	return txv.TxEncode(tx)
}
