package gov

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/utils"

	"cosmossdk.io/core/address"
	sdkerrors "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

// WeightedVoteOptions defines a slice of WeightedVoteOption.
type WeightedVoteOptions []WeightedVoteOption

// NewMsgSubmitProposal constructs a MsgSubmitProposal.
func NewMsgSubmitProposal(args SubmitProposalCall, cdc codec.Codec, addrCdc address.Codec) (*govv1.MsgSubmitProposal, common.Address, error) {
	emptyAddr := common.Address{}
	// -------------------------------------------------------------------------
	// 1. Argument sanity
	// -------------------------------------------------------------------------
	if args.Proposer == emptyAddr {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidProposer, args.Proposer)
	}

	// 1-a  JSON blob
	if len(args.JsonProposal) == 0 {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidProposalJSON, "jsonBlob arg")
	}

	// 1-b  Deposit
	coins := args.Deposit

	// -------------------------------------------------------------------------
	// 2. Convert coins and build proposal
	// -------------------------------------------------------------------------
	amt, err := cmn.NewSdkCoinsFromCoins(coins)
	if err != nil {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidDeposits, "deposit arg")
	}

	// 1. Decode the envelope
	var prop struct {
		Messages  []json.RawMessage `json:"messages"`
		Metadata  string            `json:"metadata"`
		Title     string            `json:"title"`
		Summary   string            `json:"summary"`
		Expedited bool              `json:"expedited"`
	}
	if err := json.Unmarshal(args.JsonProposal, &prop); err != nil {
		return nil, emptyAddr, sdkerrors.Wrap(err, "invalid proposal JSON")
	}

	// 2. Decode each message
	msgs := make([]sdk.Msg, len(prop.Messages))
	for i, m := range prop.Messages {
		var msg sdk.Msg
		if err := cdc.UnmarshalInterfaceJSON(m, &msg); err != nil {
			return nil, emptyAddr, sdkerrors.Wrapf(err, "message %d", i)
		}
		msgs[i] = msg
	}

	// 3. Pack into Any
	anys := make([]*codectypes.Any, len(msgs))
	for i, m := range msgs {
		anyVal, err := codectypes.NewAnyWithValue(m)
		if err != nil {
			return nil, common.Address{}, err
		}
		anys[i] = anyVal
	}
	// 3. Build & dispatch MsgSubmitProposal
	proposerAddr, err := addrCdc.BytesToString(args.Proposer.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode proposer address: %w", err)
	}
	smsg := &govv1.MsgSubmitProposal{
		Messages:       anys,
		InitialDeposit: amt,
		Proposer:       proposerAddr,
		Metadata:       prop.Metadata,
		Title:          prop.Title,
		Summary:        prop.Summary,
		Expedited:      prop.Expedited,
	}

	return smsg, args.Proposer, nil
}

// NewMsgDeposit constructs a MsgDeposit.
func NewMsgDeposit(args DepositCall, addrCdc address.Codec) (*govv1.MsgDeposit, common.Address, error) {
	emptyAddr := common.Address{}
	if args.Depositor == emptyAddr {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidDepositor, args.Depositor)
	}

	amt, err := cmn.NewSdkCoinsFromCoins(args.Amount)
	if err != nil {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidDeposits, "deposit arg")
	}

	depositorAddr, err := addrCdc.BytesToString(args.Depositor.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode depositor address: %w", err)
	}
	msg := &govv1.MsgDeposit{
		ProposalId: args.ProposalId,
		Amount:     amt,
		Depositor:  depositorAddr,
	}

	return msg, args.Depositor, nil
}

// NewMsgCancelProposal constructs a MsgCancelProposal.
func NewMsgCancelProposal(args CancelProposalCall, addrCdc address.Codec) (*govv1.MsgCancelProposal, common.Address, error) {
	emptyAddr := common.Address{}
	if args.Proposer == emptyAddr {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidProposer, args.Proposer)
	}

	proposerAddr, err := addrCdc.BytesToString(args.Proposer.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode proposer address: %w", err)
	}
	return govv1.NewMsgCancelProposal(
		args.ProposalId,
		proposerAddr,
	), args.Proposer, nil
}

// NewMsgVote creates a new MsgVote instance.
func NewMsgVote(args VoteCall, addrCdc address.Codec) (*govv1.MsgVote, common.Address, error) {
	if args.Voter == (common.Address{}) {
		return nil, common.Address{}, fmt.Errorf(ErrInvalidVoter, args.Voter)
	}

	voterAddr, err := addrCdc.BytesToString(args.Voter.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode voter address: %w", err)
	}
	msg := &govv1.MsgVote{
		ProposalId: args.ProposalId,
		Voter:      voterAddr,
		Option:     govv1.VoteOption(args.Option),
		Metadata:   args.Metadata,
	}

	return msg, args.Voter, nil
}

// NewMsgVoteWeighted creates a new MsgVoteWeighted instance.
func NewMsgVoteWeighted(args VoteWeightedCall, addrCdc address.Codec) (*govv1.MsgVoteWeighted, common.Address, WeightedVoteOptions, error) {
	if args.Voter == (common.Address{}) {
		return nil, common.Address{}, WeightedVoteOptions{}, fmt.Errorf(ErrInvalidVoter, args.Voter)
	}

	weightedOptions := make([]*govv1.WeightedVoteOption, len(args.Options))
	for i, option := range args.Options {
		weightedOptions[i] = &govv1.WeightedVoteOption{
			Option: govv1.VoteOption(option.Option),
			Weight: option.Weight,
		}
	}

	voterAddr, err := addrCdc.BytesToString(args.Voter.Bytes())
	if err != nil {
		return nil, common.Address{}, WeightedVoteOptions{}, fmt.Errorf("failed to decode voter address: %w", err)
	}
	msg := &govv1.MsgVoteWeighted{
		ProposalId: args.ProposalId,
		Voter:      voterAddr,
		Options:    weightedOptions,
		Metadata:   args.Metadata,
	}

	return msg, args.Voter, args.Options, nil
}

// ParseVotesArgs parses the arguments for the Votes query.
func ParseVotesArgs(args GetVotesCall) (*govv1.QueryVotesRequest, error) {
	return &govv1.QueryVotesRequest{
		ProposalId: args.ProposalId,
		Pagination: args.Pagination.ToPageRequest(),
	}, nil
}

func (vo *GetVotesReturn) FromResponse(res *govv1.QueryVotesResponse) error {
	vo.Votes = make([]WeightedVote, len(res.Votes))
	for i, v := range res.Votes {
		hexAddr, err := utils.HexAddressFromBech32String(v.Voter)
		if err != nil {
			return err
		}
		options := make([]WeightedVoteOption, len(v.Options))
		for j, opt := range v.Options {
			options[j] = WeightedVoteOption{
				Option: uint8(opt.Option), //nolint:gosec // G115
				Weight: opt.Weight,
			}
		}
		vo.Votes[i] = WeightedVote{
			ProposalId: v.ProposalId,
			Voter:      hexAddr,
			Options:    options,
			Metadata:   v.Metadata,
		}
	}
	vo.PageResponse = cmn.FromPageResponse(res.Pagination)
	return nil
}

// ParseVoteArgs parses the arguments for the Votes query.
func ParseVoteArgs(args GetVoteCall, addrCdc address.Codec) (*govv1.QueryVoteRequest, error) {
	voterAddr, err := addrCdc.BytesToString(args.Voter.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode voter address: %w", err)
	}
	return &govv1.QueryVoteRequest{
		ProposalId: args.ProposalId,
		Voter:      voterAddr,
	}, nil
}

func (vo *GetVoteReturn) FromResponse(res *govv1.QueryVoteResponse) error {
	hexVoter, err := utils.HexAddressFromBech32String(res.Vote.Voter)
	if err != nil {
		return err
	}
	vo.Vote.Voter = hexVoter
	vo.Vote.Metadata = res.Vote.Metadata
	vo.Vote.ProposalId = res.Vote.ProposalId

	options := make([]WeightedVoteOption, len(res.Vote.Options))
	for j, opt := range res.Vote.Options {
		options[j] = WeightedVoteOption{
			Option: uint8(opt.Option), //nolint:gosec // G115
			Weight: opt.Weight,
		}
	}
	vo.Vote.Options = options
	return nil
}

// ParseDepositArgs parses the arguments for the Deposit query.
func ParseDepositArgs(args GetDepositCall, addrCdc address.Codec) (*govv1.QueryDepositRequest, error) {
	depositorAddr, err := addrCdc.BytesToString(args.Depositor.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode depositor address: %w", err)
	}
	return &govv1.QueryDepositRequest{
		ProposalId: args.ProposalId,
		Depositor:  depositorAddr,
	}, nil
}

// ParseDepositsArgs parses the arguments for the Deposits query.
func ParseDepositsArgs(args GetDepositsCall) (*govv1.QueryDepositsRequest, error) {
	return &govv1.QueryDepositsRequest{
		ProposalId: args.ProposalId,
		Pagination: args.Pagination.ToPageRequest(),
	}, nil
}

// ParseTallyResultArgs parses the arguments for the TallyResult query.
func ParseTallyResultArgs(args GetTallyResultCall) (*govv1.QueryTallyResultRequest, error) {
	return &govv1.QueryTallyResultRequest{
		ProposalId: args.ProposalId,
	}, nil
}

func (do *GetDepositReturn) FromResponse(res *govv1.QueryDepositResponse) error {
	hexDepositor, err := utils.HexAddressFromBech32String(res.Deposit.Depositor)
	if err != nil {
		return err
	}
	coins := make([]cmn.Coin, len(res.Deposit.Amount))
	for i, c := range res.Deposit.Amount {
		coins[i] = cmn.Coin{
			Denom:  c.Denom,
			Amount: c.Amount.BigInt(),
		}
	}
	do.Deposit = DepositData{
		ProposalId: res.Deposit.ProposalId,
		Depositor:  hexDepositor,
		Amount:     coins,
	}
	return nil
}

func (do *GetDepositsReturn) FromResponse(res *govv1.QueryDepositsResponse) error {
	do.Deposits = make([]DepositData, len(res.Deposits))
	for i, d := range res.Deposits {
		hexDepositor, err := utils.HexAddressFromBech32String(d.Depositor)
		if err != nil {
			return err
		}
		coins := make([]cmn.Coin, len(d.Amount))
		for j, c := range d.Amount {
			coins[j] = cmn.Coin{
				Denom:  c.Denom,
				Amount: c.Amount.BigInt(),
			}
		}
		do.Deposits[i] = DepositData{
			ProposalId: d.ProposalId,
			Depositor:  hexDepositor,
			Amount:     coins,
		}
	}
	do.PageResponse = cmn.FromPageResponse(res.Pagination)
	return nil
}

func (tro *GetTallyResultReturn) FromResponse(res *govv1.QueryTallyResultResponse) {
	tro.TallyResult = TallyResultData{
		Yes:        res.Tally.YesCount,
		Abstain:    res.Tally.AbstainCount,
		No:         res.Tally.NoCount,
		NoWithVeto: res.Tally.NoWithVetoCount,
	}
}

// ParseProposalArgs parses the arguments for the Proposal query
func ParseProposalArgs(args GetProposalCall) (*govv1.QueryProposalRequest, error) {
	return &govv1.QueryProposalRequest{
		ProposalId: args.ProposalId,
	}, nil
}

// ParseProposalsArgs parses the arguments for the Proposals query
func ParseProposalsArgs(args GetProposalsCall, addrCdc address.Codec) (*govv1.QueryProposalsRequest, error) {
	voter := ""
	if args.Voter != (common.Address{}) {
		var err error
		voter, err = addrCdc.BytesToString(args.Voter.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to decode voter address: %w", err)
		}
	}

	depositor := ""
	if args.Depositor != (common.Address{}) {
		var err error
		depositor, err = addrCdc.BytesToString(args.Depositor.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to decode depositor address: %w", err)
		}
	}

	return &govv1.QueryProposalsRequest{
		ProposalStatus: govv1.ProposalStatus(args.ProposalStatus), //nolint:gosec // G115
		Voter:          voter,
		Depositor:      depositor,
		Pagination:     args.Pagination.ToPageRequest(),
	}, nil
}

func (po *GetProposalReturn) FromResponse(res *govv1.QueryProposalResponse) error {
	msgs := make([]string, len(res.Proposal.Messages))
	for i, msg := range res.Proposal.Messages {
		msgs[i] = msg.TypeUrl
	}

	coins := make([]cmn.Coin, len(res.Proposal.TotalDeposit))
	for i, c := range res.Proposal.TotalDeposit {
		coins[i] = cmn.Coin{
			Denom:  c.Denom,
			Amount: c.Amount.BigInt(),
		}
	}

	proposer, err := utils.HexAddressFromBech32String(res.Proposal.Proposer)
	if err != nil {
		return err
	}

	po.Proposal = ProposalData{
		Id:       res.Proposal.Id,
		Messages: msgs,
		Status:   uint32(res.Proposal.Status), //nolint:gosec // G115
		FinalTallyResult: TallyResultData{
			Yes:        res.Proposal.FinalTallyResult.YesCount,
			Abstain:    res.Proposal.FinalTallyResult.AbstainCount,
			No:         res.Proposal.FinalTallyResult.NoCount,
			NoWithVeto: res.Proposal.FinalTallyResult.NoWithVetoCount,
		},
		SubmitTime:     uint64(res.Proposal.SubmitTime.Unix()),     //nolint:gosec // G115
		DepositEndTime: uint64(res.Proposal.DepositEndTime.Unix()), //nolint:gosec // G115
		TotalDeposit:   coins,
		Metadata:       res.Proposal.Metadata,
		Title:          res.Proposal.Title,
		Summary:        res.Proposal.Summary,
		Proposer:       proposer,
	}
	// The following fields are nil when proposal is in deposit period
	if res.Proposal.VotingStartTime != nil {
		po.Proposal.VotingStartTime = uint64(res.Proposal.VotingStartTime.Unix()) //nolint:gosec // G115
	}
	if res.Proposal.VotingEndTime != nil {
		po.Proposal.VotingEndTime = uint64(res.Proposal.VotingEndTime.Unix()) //nolint:gosec // G115
	}
	return nil
}

func (po *GetProposalsReturn) FromResponse(res *govv1.QueryProposalsResponse) (*GetProposalsReturn, error) {
	po.Proposals = make([]ProposalData, len(res.Proposals))
	for i, p := range res.Proposals {
		msgs := make([]string, len(p.Messages))
		for j, msg := range p.Messages {
			msgs[j] = msg.TypeUrl
		}

		coins := make([]cmn.Coin, len(p.TotalDeposit))
		for j, c := range p.TotalDeposit {
			coins[j] = cmn.Coin{
				Denom:  c.Denom,
				Amount: c.Amount.BigInt(),
			}
		}

		proposer, err := utils.HexAddressFromBech32String(p.Proposer)
		if err != nil {
			return nil, err
		}

		proposalData := ProposalData{
			Id:       p.Id,
			Messages: msgs,
			Status:   uint32(p.Status), //nolint:gosec // G115
			FinalTallyResult: TallyResultData{
				Yes:        p.FinalTallyResult.YesCount,
				Abstain:    p.FinalTallyResult.AbstainCount,
				No:         p.FinalTallyResult.NoCount,
				NoWithVeto: p.FinalTallyResult.NoWithVetoCount,
			},
			SubmitTime:     uint64(p.SubmitTime.Unix()),     //nolint:gosec // G115
			DepositEndTime: uint64(p.DepositEndTime.Unix()), //nolint:gosec // G115
			TotalDeposit:   coins,
			Metadata:       p.Metadata,
			Title:          p.Title,
			Summary:        p.Summary,
			Proposer:       proposer,
		}

		// The following fields are nil when proposal is in deposit period
		if p.VotingStartTime != nil {
			proposalData.VotingStartTime = uint64(p.VotingStartTime.Unix()) //nolint:gosec // G115
		}
		if p.VotingEndTime != nil {
			proposalData.VotingEndTime = uint64(p.VotingEndTime.Unix()) //nolint:gosec // G115
		}

		po.Proposals[i] = proposalData
	}

	po.PageResponse = cmn.FromPageResponse(res.Pagination)
	return po, nil
}

// FromResponse populates the GetParamsReturn from a query response
func (o *GetParamsReturn) FromResponse(res *govv1.QueryParamsResponse) {
	o.Params.VotingPeriod = res.Params.VotingPeriod.Nanoseconds()
	o.Params.MinDeposit = cmn.NewCoinsResponse(res.Params.MinDeposit)
	o.Params.MaxDepositPeriod = res.Params.MaxDepositPeriod.Nanoseconds()
	o.Params.Quorum = res.Params.Quorum
	o.Params.Threshold = res.Params.Threshold
	o.Params.VetoThreshold = res.Params.VetoThreshold
	o.Params.MinInitialDepositRatio = res.Params.MinInitialDepositRatio
	o.Params.ProposalCancelRatio = res.Params.ProposalCancelRatio
	o.Params.ProposalCancelDest = res.Params.ProposalCancelDest
	o.Params.ExpeditedVotingPeriod = res.Params.ExpeditedVotingPeriod.Nanoseconds()
	o.Params.ExpeditedThreshold = res.Params.ExpeditedThreshold
	o.Params.ExpeditedMinDeposit = cmn.NewCoinsResponse(res.Params.ExpeditedMinDeposit)
	o.Params.BurnVoteQuorum = res.Params.BurnVoteQuorum
	o.Params.BurnProposalDepositPrevote = res.Params.BurnProposalDepositPrevote
	o.Params.BurnVoteVeto = res.Params.BurnVoteVeto
	o.Params.MinDepositRatio = res.Params.MinDepositRatio
}

// BuildQueryParamsRequest returns the structure for the governance parameters query.
func BuildQueryParamsRequest(args GetParamsCall) (*govv1.QueryParamsRequest, error) {
	return &govv1.QueryParamsRequest{
		ParamsType: "",
	}, nil
}

// BuildQueryConstitutionRequest validates the args (none expected).
func BuildQueryConstitutionRequest(args GetConstitutionCall) (*govv1.QueryConstitutionRequest, error) {
	return &govv1.QueryConstitutionRequest{}, nil
}
