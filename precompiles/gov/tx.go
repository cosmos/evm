package gov

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// SubmitProposalMethod defines the ABI method name for the gov SubmitProposal transaction.
	SubmitProposalMethod = "submitProposal"
	// DepositMethod defines the ABI method name for the gov Deposit transaction.
	DepositMethod = "deposit"
	// DepositProposalMethod defines the ABI method name for the gov DepositProposal transaction.
	CancelProposalMethod = "cancelProposal"
	// VoteMethod defines the ABI method name for the gov Vote transaction.
	VoteMethod = "vote"
	// VoteWeightedMethod defines the ABI method name for the gov VoteWeighted transaction.
	VoteWeightedMethod = "voteWeighted"
)

// SubmitProposal defines a method to submit a proposal.
func (p *Precompile) SubmitProposal(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, proposerHexAddr, err := NewMsgSubmitProposal(args, p.codec, p.addrCdc)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != proposerHexAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, proposerHexAddr)
	}

	res, err := p.govMsgServer.SubmitProposal(ctx, msg)
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, SubmitProposalMethod, err.Error())
	}

	if err = p.EmitSubmitProposalEvent(ctx, stateDB, proposerHexAddr, res.ProposalId); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, SubmitProposalMethod, err.Error())
	}

	return method.Outputs.Pack(res.ProposalId)
}

// Deposit defines a method to add a deposit on a specific proposal.
func (p *Precompile) Deposit(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, depositorHexAddr, err := NewMsgDeposit(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != depositorHexAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, depositorHexAddr)
	}

	if _, err = p.govMsgServer.Deposit(ctx, msg); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, DepositMethod, err.Error())
	}

	if err = p.EmitDepositEvent(ctx, stateDB, depositorHexAddr, msg.ProposalId, msg.Amount); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, DepositMethod, err.Error())
	}

	return method.Outputs.Pack(true)
}

// CancelProposal defines a method to cancel a proposal.
func (p *Precompile) CancelProposal(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, proposerHexAddr, err := NewMsgCancelProposal(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != proposerHexAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, proposerHexAddr)
	}

	if _, err = p.govMsgServer.CancelProposal(ctx, msg); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, CancelProposalMethod, err.Error())
	}

	if err = p.EmitCancelProposalEvent(ctx, stateDB, proposerHexAddr, msg.ProposalId); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, CancelProposalMethod, err.Error())
	}

	return method.Outputs.Pack(true)
}

// Vote defines a method to add a vote on a specific proposal.
func (p Precompile) Vote(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, voterHexAddr, err := NewMsgVote(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != voterHexAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, voterHexAddr)
	}

	if _, err = p.govMsgServer.Vote(ctx, msg); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, VoteMethod, err.Error())
	}

	if err = p.EmitVoteEvent(ctx, stateDB, voterHexAddr, msg.ProposalId, int32(msg.Option)); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, VoteMethod, err.Error())
	}

	return method.Outputs.Pack(true)
}

// VoteWeighted defines a method to add a vote on a specific proposal.
func (p *Precompile) VoteWeighted(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, voterHexAddr, options, err := NewMsgVoteWeighted(method, args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != voterHexAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, voterHexAddr)
	}

	if _, err = p.govMsgServer.VoteWeighted(ctx, msg); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, VoteWeightedMethod, err.Error())
	}

	if err = p.EmitVoteWeightedEvent(ctx, stateDB, voterHexAddr, msg.ProposalId, options); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, VoteWeightedMethod, err.Error())
	}

	return method.Outputs.Pack(true)
}
