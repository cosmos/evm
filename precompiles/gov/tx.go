package gov

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
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
	msg, proposerHexAddr, err := NewMsgSubmitProposal(args, p.codec)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != proposerHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), proposerHexAddr.String())
	}

	res, err := govkeeper.NewMsgServerImpl(&p.govKeeper).SubmitProposal(ctx, msg)
	if err != nil {
		return nil, err
	}

	deposit := msg.InitialDeposit
	convertedAmount, err := utils.Uint256FromBigInt(evmtypes.ConvertAmountTo18DecimalsBigInt(deposit.AmountOf(evmtypes.GetEVMCoinDenom()).BigInt()))
	if err != nil {
		return nil, err
	}
	if convertedAmount.Cmp(uint256.NewInt(0)) == 1 {
		p.SetBalanceChangeEntries(cmn.NewBalanceChangeEntry(proposerHexAddr, convertedAmount, cmn.Sub))
	}

	if err = p.EmitSubmitProposalEvent(ctx, stateDB, proposerHexAddr, res.ProposalId); err != nil {
		return nil, err
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
	msg, depositorHexAddr, err := NewMsgDeposit(args)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != depositorHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), depositorHexAddr.String())
	}

	if _, err = govkeeper.NewMsgServerImpl(&p.govKeeper).Deposit(ctx, msg); err != nil {
		return nil, err
	}
	for _, coin := range msg.Amount {
		if coin.Denom != evmtypes.GetEVMCoinDenom() {
			continue
		}
		convertedAmount, err := utils.Uint256FromBigInt(evmtypes.ConvertAmountTo18DecimalsBigInt(coin.Amount.BigInt()))
		if err != nil {
			return nil, err
		}
		if convertedAmount.Cmp(uint256.NewInt(0)) == 1 {
			p.SetBalanceChangeEntries(cmn.NewBalanceChangeEntry(depositorHexAddr, convertedAmount, cmn.Sub))
		}
	}

	if err = p.EmitDepositEvent(ctx, stateDB, depositorHexAddr, msg.ProposalId, msg.Amount); err != nil {
		return nil, err
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
	msg, proposerHexAddr, err := NewMsgCancelProposal(args)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != proposerHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), proposerHexAddr.String())
	}

	// pre-calculate the remaining deposit
	govParams, err := p.govKeeper.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	cancelRate, err := math.LegacyNewDecFromStr(govParams.ProposalCancelRatio)
	if err != nil {
		return nil, err
	}
	deposits, err := p.govKeeper.GetDeposits(ctx, msg.ProposalId)
	if err != nil {
		return nil, err
	}
	var remaninig math.Int
	for _, deposit := range deposits {
		if deposit.Depositor != sdk.AccAddress(proposerHexAddr.Bytes()).String() {
			continue
		}
		for _, coin := range deposit.Amount {
			if coin.Denom == evmtypes.GetEVMCoinDenom() {
				cancelFee := coin.Amount.ToLegacyDec().Mul(cancelRate).TruncateInt()
				remaninig = coin.Amount.Sub(cancelFee)
			}
		}
	}
	if _, err = govkeeper.NewMsgServerImpl(&p.govKeeper).CancelProposal(ctx, msg); err != nil {
		return nil, err
	}

	convertedAmount, err := utils.Uint256FromBigInt(evmtypes.ConvertAmountTo18DecimalsBigInt(remaninig.BigInt()))
	if err != nil {
		return nil, err
	}
	if convertedAmount.Cmp(uint256.NewInt(0)) == 1 {
		p.SetBalanceChangeEntries(cmn.NewBalanceChangeEntry(proposerHexAddr, convertedAmount, cmn.Add))
	}

	if err = p.EmitCancelProposalEvent(ctx, stateDB, proposerHexAddr, msg.ProposalId); err != nil {
		return nil, err
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
	msg, voterHexAddr, err := NewMsgVote(args)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != voterHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), voterHexAddr.String())
	}

	msgSrv := govkeeper.NewMsgServerImpl(&p.govKeeper)
	if _, err = msgSrv.Vote(ctx, msg); err != nil {
		return nil, err
	}

	if err = p.EmitVoteEvent(ctx, stateDB, voterHexAddr, msg.ProposalId, int32(msg.Option)); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// VoteWeighted defines a method to add a vote on a specific proposal.
func (p Precompile) VoteWeighted(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, voterHexAddr, options, err := NewMsgVoteWeighted(method, args)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != voterHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), voterHexAddr.String())
	}

	msgSrv := govkeeper.NewMsgServerImpl(&p.govKeeper)
	if _, err = msgSrv.VoteWeighted(ctx, msg); err != nil {
		return nil, err
	}

	if err = p.EmitVoteWeightedEvent(ctx, stateDB, voterHexAddr, msg.ProposalId, options); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
