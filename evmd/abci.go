package evmd

import (
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

const (
	PrepareProposalDuration = "prepare_proposal_duration"
	ProcessProposalDuration = "process_proposal_duration"
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

type ExtProposalHandler struct {
	baseapp.DefaultProposalHandler
	verifier baseapp.ProposalTxVerifier
	selector baseapp.TxSelector
}

func NewExtProposalHandler(mp mempool.Mempool, txVerifier baseapp.ProposalTxVerifier) *ExtProposalHandler {
	return &ExtProposalHandler{
		DefaultProposalHandler: *baseapp.NewDefaultProposalHandler(mp, txVerifier),
		verifier:               txVerifier,
		selector:               baseapp.NewDefaultTxSelector(),
	}
}

func (h *ExtProposalHandler) PrepareProposalHandler() sdk.PrepareProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		defer telemetry.MeasureSince(time.Now(), PrepareProposalDuration)
		return h.DefaultProposalHandler.PrepareProposalHandler()(ctx, req)
	}
}

func (h *ExtProposalHandler) ProcessProposalHandler() sdk.ProcessProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		defer telemetry.MeasureSince(time.Now(), ProcessProposalDuration)
		return h.DefaultProposalHandler.ProcessProposalHandler()(ctx, req)
	}
}
