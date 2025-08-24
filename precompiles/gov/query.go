package gov

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// GetVotesMethod defines the method name for the votes precompile request.
	GetVotesMethod = "getVotes"
	// GetVoteMethod defines the method name for the vote precompile request.
	GetVoteMethod = "getVote"
	// GetDepositMethod defines the method name for the deposit precompile request.
	GetDepositMethod = "getDeposit"
	// GetDepositsMethod defines the method name for the deposits precompile request.
	GetDepositsMethod = "getDeposits"
	// GetTallyResultMethod defines the method name for the tally result precompile request.
	GetTallyResultMethod = "getTallyResult"
	// GetProposalMethod defines the method name for the proposal precompile request.
	GetProposalMethod = "getProposal"
	// GetProposalsMethod defines the method name for the proposals precompile request.
	GetProposalsMethod = "getProposals"
	// GetParamsMethod defines the method name for the get params precompile request.
	GetParamsMethod = "getParams"
	// GetConstitutionMethod defines the method name for the get constitution precompile request.
	GetConstitutionMethod = "getConstitution"
)

// GetVotes implements the query logic for getting votes for a proposal.
func (p *Precompile) GetVotes(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	queryVotesReq, err := ParseVotesArgs(method, args)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Votes(ctx, queryVotesReq)
	if err != nil {
		return nil, err
	}

	output, err := new(VotesOutput).FromResponse(res)
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack(output.Votes, output.PageResponse)
}

// GetVote implements the query logic for getting votes for a proposal.
func (p *Precompile) GetVote(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	queryVotesReq, err := ParseVoteArgs(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Vote(ctx, queryVotesReq)
	if err != nil {
		return nil, err
	}

	output, err := new(VoteOutput).FromResponse(res)
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack(output.Vote)
}

// GetDeposit implements the query logic for getting a deposit for a proposal.
func (p *Precompile) GetDeposit(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	queryDepositReq, err := ParseDepositArgs(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Deposit(ctx, queryDepositReq)
	if err != nil {
		return nil, err
	}

	output, err := new(DepositOutput).FromResponse(res)
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack(output.Deposit)
}

// GetDeposits implements the query logic for getting all deposits for a proposal.
func (p *Precompile) GetDeposits(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	queryDepositsReq, err := ParseDepositsArgs(method, args)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Deposits(ctx, queryDepositsReq)
	if err != nil {
		return nil, err
	}

	output, err := new(DepositsOutput).FromResponse(res)
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack(output.Deposits, output.PageResponse)
}

// GetTallyResult implements the query logic for getting the tally result of a proposal.
func (p *Precompile) GetTallyResult(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	queryTallyResultReq, err := ParseTallyResultArgs(args)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.TallyResult(ctx, queryTallyResultReq)
	if err != nil {
		return nil, err
	}

	output := new(TallyResultOutput).FromResponse(res)
	return method.Outputs.Pack(output.TallyResult)
}

// GetProposal implements the query logic for getting a proposal
func (p *Precompile) GetProposal(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	queryProposalReq, err := ParseProposalArgs(args)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Proposal(ctx, queryProposalReq)
	if err != nil {
		return nil, err
	}

	output, err := new(ProposalOutput).FromResponse(res)
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack(output.Proposal)
}

// GetProposals implements the query logic for getting proposals
func (p *Precompile) GetProposals(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	queryProposalsReq, err := ParseProposalsArgs(method, args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Proposals(ctx, queryProposalsReq)
	if err != nil {
		return nil, err
	}

	output, err := new(ProposalsOutput).FromResponse(res)
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack(output.Proposals, output.PageResponse)
}

// GetParams implements the query logic for getting governance parameters
func (p *Precompile) GetParams(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	queryParamsReq, err := BuildQueryParamsRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Params(ctx, queryParamsReq)
	if err != nil {
		return nil, err
	}

	output := new(ParamsOutput).FromResponse(res)
	return method.Outputs.Pack(output)
}

// GetConstitution implements the query logic for getting the constitution
func (p *Precompile) GetConstitution(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	req, err := BuildQueryConstitutionRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Constitution(ctx, req)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(res.Constitution)
}
