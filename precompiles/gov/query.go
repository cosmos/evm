package gov

import (
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
	args GetVotesCall,
) (*GetVotesReturn, error) {
	queryVotesReq, err := ParseVotesArgs(args)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Votes(ctx, queryVotesReq)
	if err != nil {
		return nil, err
	}

	var out GetVotesReturn
	if err := out.FromResponse(res); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetVote implements the query logic for getting votes for a proposal.
func (p *Precompile) GetVote(
	ctx sdk.Context,
	args GetVoteCall,
) (*GetVoteReturn, error) {
	queryVotesReq, err := ParseVoteArgs(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Vote(ctx, queryVotesReq)
	if err != nil {
		return nil, err
	}

	var out GetVoteReturn
	if err := out.FromResponse(res); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDeposit implements the query logic for getting a deposit for a proposal.
func (p *Precompile) GetDeposit(
	ctx sdk.Context,
	args GetDepositCall,
) (*GetDepositReturn, error) {
	queryDepositReq, err := ParseDepositArgs(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Deposit(ctx, queryDepositReq)
	if err != nil {
		return nil, err
	}

	var out GetDepositReturn
	if err := out.FromResponse(res); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDeposits implements the query logic for getting all deposits for a proposal.
func (p *Precompile) GetDeposits(
	ctx sdk.Context,
	args GetDepositsCall,
) (*GetDepositsReturn, error) {
	queryDepositsReq, err := ParseDepositsArgs(args)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Deposits(ctx, queryDepositsReq)
	if err != nil {
		return nil, err
	}

	var out GetDepositsReturn
	if err := out.FromResponse(res); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTallyResult implements the query logic for getting the tally result of a proposal.
func (p *Precompile) GetTallyResult(
	ctx sdk.Context,
	args GetTallyResultCall,
) (*GetTallyResultReturn, error) {
	queryTallyResultReq, err := ParseTallyResultArgs(args)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.TallyResult(ctx, queryTallyResultReq)
	if err != nil {
		return nil, err
	}

	var out GetTallyResultReturn
	out.FromResponse(res)
	return &out, nil
}

// GetProposal implements the query logic for getting a proposal
func (p *Precompile) GetProposal(
	ctx sdk.Context,
	args GetProposalCall,
) (*GetProposalReturn, error) {
	queryProposalReq, err := ParseProposalArgs(args)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Proposal(ctx, queryProposalReq)
	if err != nil {
		return nil, err
	}

	var out GetProposalReturn
	if err := out.FromResponse(res); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetProposals implements the query logic for getting proposals
func (p *Precompile) GetProposals(
	ctx sdk.Context,
	args GetProposalsCall,
) (*GetProposalsReturn, error) {
	queryProposalsReq, err := ParseProposalsArgs(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Proposals(ctx, queryProposalsReq)
	if err != nil {
		return nil, err
	}

	var output GetProposalsReturn
	if _, err := output.FromResponse(res); err != nil {
		return nil, err
	}

	return &output, nil
}

// GetParams implements the query logic for getting governance parameters
func (p *Precompile) GetParams(
	ctx sdk.Context,
	args GetParamsCall,
) (*GetParamsReturn, error) {
	queryParamsReq, err := BuildQueryParamsRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Params(ctx, queryParamsReq)
	if err != nil {
		return nil, err
	}

	var out GetParamsReturn
	out.FromResponse(res)
	return &out, nil
}

// GetConstitution implements the query logic for getting the constitution
func (p *Precompile) GetConstitution(
	ctx sdk.Context,
	args GetConstitutionCall,
) (*GetConstitutionReturn, error) {
	req, err := BuildQueryConstitutionRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.govQuerier.Constitution(ctx, req)
	if err != nil {
		return nil, err
	}

	return &GetConstitutionReturn{Constitution: res.Constitution}, nil
}
