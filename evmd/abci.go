package evmd

import (
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ baseapp.ProposalTxVerifier = ExtProposalVerifier{}

type ExtProposalVerifier struct {
	baseapp.ProposalTxVerifier
	txEncoder sdk.TxEncoder
}

func NewProposalVerifier(wrap baseapp.ProposalTxVerifier, encoder sdk.TxEncoder) *ExtProposalVerifier {
	return &ExtProposalVerifier{
		wrap,
		encoder,
	}
}

func (v ExtProposalVerifier) PrepareProposalVerifyTx(tx sdk.Tx) ([]byte, error) {
	bz, err := v.txEncoder(tx)
	if err != nil {
		return nil, err
	}
	return bz, nil
}
