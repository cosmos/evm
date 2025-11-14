package gov

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/gov"
	"github.com/cosmos/evm/precompiles/testutil"
	testconstants "github.com/cosmos/evm/testutil/constants"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

var (
	_, _, addr = testdata.KeyTestPubAddr()
	// gov account authority address
	govAcct = authtypes.NewModuleAddress(govtypes.ModuleName)
	// TestProposalMsgs are msgs used on a proposal.
	TestProposalMsgs = []sdk.Msg{
		banktypes.NewMsgSend(govAcct, addr, sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, math.NewInt(1000)))),
	}
)

func (s *PrecompileTestSuite) TestGetVotes() {
	var ctx sdk.Context
	testCases := []struct {
		name        string
		malleate    func() []gov.WeightedVote
		args        *gov.GetVotesCall
		expPass     bool
		errContains string
		expTotal    uint64
	}{
		{
			name: "valid query",
			malleate: func() []gov.WeightedVote {
				proposalID := uint64(1)
				voter := s.keyring.GetAccAddr(0)
				voteOption := &govv1.WeightedVoteOption{
					Option: govv1.OptionYes,
					Weight: "1.0",
				}

				err := s.network.App.GetGovKeeper().AddVote(
					s.network.GetContext(),
					proposalID,
					voter,
					[]*govv1.WeightedVoteOption{voteOption},
					"",
				)
				s.Require().NoError(err)

				return []gov.WeightedVote{{
					ProposalId: proposalID,
					Voter:      s.keyring.GetAddr(0),
					Options: []gov.WeightedVoteOption{
						{
							Option: uint8(voteOption.Option), //nolint:gosec // G115 -- integer overflow is not happening here
							Weight: voteOption.Weight,
						},
					},
				}}
			},
			args:     &gov.GetVotesCall{ProposalId: uint64(1), Pagination: cmn.PageRequest{Limit: 10, CountTotal: true}},
			expPass:  true,
			expTotal: 1,
		},
		{
			name:        "invalid proposal ID",
			args:        &gov.GetVotesCall{ProposalId: uint64(0), Pagination: cmn.PageRequest{Limit: 10, CountTotal: true}},
			expPass:     false,
			errContains: "proposal id can not be 0",
		},
		{
			name:        "fail - invalid number of args",
			args:        &gov.GetVotesCall{},
			errContains: fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 0),
		},
		{
			name: "fail - internal error from response",
			malleate: func() []gov.WeightedVote {
				proposalID := uint64(1)
				voter := sdk.AccAddress{}
				voteOption := &govv1.WeightedVoteOption{
					Option: govv1.OptionYes,
					Weight: "1.0",
				}
				err := s.network.App.GetGovKeeper().AddVote(
					s.network.GetContext(),
					proposalID,
					voter,
					[]*govv1.WeightedVoteOption{voteOption},
					"",
				)
				s.Require().NoError(err)
				return []gov.WeightedVote{{
					ProposalId: proposalID,
					Voter:      common.Address{},
					Options: []gov.WeightedVoteOption{
						{
							Option: uint8(voteOption.Option), //nolint:gosec // G115 -- integer overflow is not happening here
							Weight: voteOption.Weight,
						},
					},
				}}
			},
			args:        &gov.GetVotesCall{ProposalId: uint64(1), Pagination: cmn.PageRequest{Limit: 10, CountTotal: true}},
			expPass:     false,
			errContains: "empty address string is not allowed",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()

			var votes []gov.WeightedVote
			if tc.malleate != nil {
				votes = tc.malleate()
			}

			out, err := s.precompile.GetVotes(ctx, *tc.args)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(votes, out.Votes)
				s.Require().Equal(tc.expTotal, out.PageResponse.Total)
			} else {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.errContains)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestGetVote() {
	var voter sdk.AccAddress
	var voterAddr common.Address

	testCases := []struct {
		name          string
		malleate      func() *gov.GetVoteCall
		expPass       bool
		expPropNumber uint64
		expVoter      common.Address
		errContains   string
	}{
		{
			name: "valid query",
			malleate: func() *gov.GetVoteCall {
				err := s.network.App.GetGovKeeper().AddVote(s.network.GetContext(), 1, voter, []*govv1.WeightedVoteOption{{Option: govv1.OptionYes, Weight: "1.0"}}, "")
				s.Require().NoError(err)

				return &gov.GetVoteCall{ProposalId: 1, Voter: voterAddr}
			},
			expPropNumber: uint64(1),
			expVoter:      common.BytesToAddress(voter.Bytes()),
			expPass:       true,
		},
		{
			name:    "invalid proposal ID",
			expPass: false,
			malleate: func() *gov.GetVoteCall {
				err := s.network.App.GetGovKeeper().AddVote(s.network.GetContext(), 1, voter, []*govv1.WeightedVoteOption{{Option: govv1.OptionYes, Weight: "1.0"}}, "")
				s.Require().NoError(err)

				return &gov.GetVoteCall{ProposalId: 10, Voter: voterAddr}
			},
			errContains: "not found for proposal",
		},
		{
			name: "non-existent vote",
			malleate: func() *gov.GetVoteCall {
				return &gov.GetVoteCall{ProposalId: 1, Voter: voterAddr}
			},
			expPass:     false,
			errContains: "not found for proposal",
		},
		{
			name: "invalid number of args",
			malleate: func() *gov.GetVoteCall {
				return &gov.GetVoteCall{}
			},
			errContains: fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 0),
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			voter = s.keyring.GetAccAddr(0)
			voterAddr = s.keyring.GetAddr(0)
			gas := uint64(200_000)

			var args gov.GetVoteCall
			if tc.malleate != nil {
				args = *tc.malleate()
			}

			_, ctx := testutil.NewPrecompileContract(s.T(), s.network.GetContext(), voterAddr, s.precompile.Address(), gas)

			out, err := s.precompile.GetVote(ctx, args)

			expVote := gov.WeightedVote{
				ProposalId: tc.expPropNumber,
				Voter:      voterAddr,
				Options:    []gov.WeightedVoteOption{{Option: uint8(govv1.OptionYes), Weight: "1.0"}},
				Metadata:   "",
			}

			if tc.expPass {
				s.Require().NoError(err)

				s.Require().NoError(err)
				s.Require().Equal(expVote.ProposalId, out.Vote.ProposalId)
				s.Require().Equal(expVote.Voter, out.Vote.Voter)
				s.Require().Equal(expVote.Options, out.Vote.Options)
				s.Require().Equal(expVote.Metadata, out.Vote.Metadata)
			} else {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.errContains)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestGetDeposit() {
	var depositor sdk.AccAddress
	testCases := []struct {
		name          string
		malleate      func()
		propNumber    uint64
		expPass       bool
		expPropNumber uint64
		gas           uint64
		errContains   string
	}{
		{
			name:          "valid query",
			malleate:      func() {},
			propNumber:    uint64(1),
			expPropNumber: uint64(1),
			expPass:       true,
			gas:           200_000,
		},
		{
			name:        "invalid proposal ID",
			propNumber:  uint64(10),
			expPass:     false,
			gas:         200_000,
			malleate:    func() {},
			errContains: "not found",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			depositor = s.keyring.GetAccAddr(0)

			tc.malleate()

			_, ctx := testutil.NewPrecompileContract(s.T(), s.network.GetContext(), s.keyring.GetAddr(0), s.precompile.Address(), tc.gas)

			args := gov.GetDepositCall{
				ProposalId: tc.propNumber,
				Depositor:  common.BytesToAddress(depositor.Bytes()),
			}
			out, err := s.precompile.GetDeposit(ctx, args)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(tc.expPropNumber, out.Deposit.ProposalId)
				s.Require().Equal(common.BytesToAddress(depositor.Bytes()), out.Deposit.Depositor)
				s.Require().Equal([]cmn.Coin{{Denom: "aatom", Amount: big.NewInt(100)}}, out.Deposit.Amount)
			} else {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestGetDeposits() {
	testCases := []struct {
		name     string
		malleate func() []gov.DepositData
		args     *gov.GetDepositsCall
		expPass  bool
		expTotal uint64
		gas      uint64
	}{
		{
			name: "valid query",
			malleate: func() []gov.DepositData {
				return []gov.DepositData{
					{ProposalId: 1, Depositor: s.keyring.GetAddr(0), Amount: []cmn.Coin{{Denom: s.network.GetBaseDenom(), Amount: big.NewInt(100)}}},
				}
			},
			args:     &gov.GetDepositsCall{ProposalId: 1, Pagination: cmn.PageRequest{Limit: 10, CountTotal: true}},
			expPass:  true,
			expTotal: 1,
			gas:      200_000,
		},
		{
			name:    "invalid proposal ID",
			args:    &gov.GetDepositsCall{ProposalId: 0, Pagination: cmn.PageRequest{Limit: 10, CountTotal: true}},
			expPass: false,
			gas:     200_000,
			malleate: func() []gov.DepositData {
				return []gov.DepositData{}
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx := s.network.GetContext()

			deposits := tc.malleate()

			out, err := s.precompile.GetDeposits(ctx, *tc.args)
			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(deposits, out.Deposits)
				s.Require().Equal(tc.expTotal, out.PageResponse.Total)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestGetTallyResult() {
	testCases := []struct {
		name        string
		malleate    func() (gov.TallyResultData, uint64)
		expPass     bool
		gas         uint64
		errContains string
	}{
		{
			name: "valid query",
			malleate: func() (gov.TallyResultData, uint64) {
				proposal, err := s.network.App.GetGovKeeper().SubmitProposal(s.network.GetContext(), TestProposalMsgs, "", "Proposal", "testing proposal", s.keyring.GetAccAddr(0), false)
				s.Require().NoError(err)
				votingStarted, err := s.network.App.GetGovKeeper().AddDeposit(s.network.GetContext(), proposal.Id, s.keyring.GetAccAddr(0), sdk.NewCoins(sdk.NewCoin(s.network.GetBaseDenom(), math.NewInt(100))))
				s.Require().NoError(err)
				s.Require().True(votingStarted)
				err = s.network.App.GetGovKeeper().AddVote(s.network.GetContext(), proposal.Id, s.keyring.GetAccAddr(0), govv1.NewNonSplitVoteOption(govv1.OptionYes), "")
				s.Require().NoError(err)
				return gov.TallyResultData{
					Yes:        "3000000000000000000",
					Abstain:    "0",
					No:         "0",
					NoWithVeto: "0",
				}, proposal.Id
			},
			expPass: true,
			gas:     200_000,
		},
		{
			name:        "invalid proposal ID",
			expPass:     false,
			gas:         200_000,
			malleate:    func() (gov.TallyResultData, uint64) { return gov.TallyResultData{}, 10 },
			errContains: "proposal 10 doesn't exist",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			expTally, propID := tc.malleate()

			_, ctx := testutil.NewPrecompileContract(s.T(), s.network.GetContext(), s.keyring.GetAddr(0), s.precompile.Address(), tc.gas)

			args := gov.GetTallyResultCall{
				ProposalId: propID,
			}
			out, err := s.precompile.GetTallyResult(ctx, args)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(expTally, out.TallyResult)
			} else {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestGetProposal() {
	testCases := []struct {
		name        string
		malleate    func() *gov.GetProposalCall
		postCheck   func(data *gov.ProposalData)
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"fail - invalid proposal ID",
			func() *gov.GetProposalCall {
				return &gov.GetProposalCall{ProposalId: uint64(0)}
			},
			func(_ *gov.ProposalData) {},
			200000,
			true,
			"proposal id can not be 0",
		},
		{
			"fail - proposal doesn't exist",
			func() *gov.GetProposalCall {
				return &gov.GetProposalCall{ProposalId: uint64(10)}
			},
			func(_ *gov.ProposalData) {},
			200000,
			true,
			"proposal 10 doesn't exist",
		},
		{
			"success - get proposal",
			func() *gov.GetProposalCall {
				return &gov.GetProposalCall{ProposalId: uint64(1)}
			},
			func(data *gov.ProposalData) {
				s.Require().Equal(uint64(1), data.Id)
				s.Require().Equal(uint32(govv1.StatusVotingPeriod), data.Status)
				s.Require().Equal(s.keyring.GetAddr(0), data.Proposer)
				s.Require().Equal("test prop", data.Title)
				s.Require().Equal("test prop", data.Summary)
				s.Require().Equal("ipfs://CID", data.Metadata)
				s.Require().Len(data.Messages, 1)
				s.Require().Equal("/cosmos.bank.v1beta1.MsgSend", data.Messages[0])
			},
			200000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			_, ctx := testutil.NewPrecompileContract(s.T(), s.network.GetContext(), s.keyring.GetAddr(0), s.precompile.Address(), tc.gas)

			out, err := s.precompile.GetProposal(ctx, *tc.malleate())

			if tc.expError {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				tc.postCheck(&out.Proposal)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestGetProposals() {
	testCases := []struct {
		name        string
		malleate    func() *gov.GetProposalsCall
		postCheck   func(data []gov.ProposalData, pageRes *cmn.PageResponse)
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"success - get all proposals",
			func() *gov.GetProposalsCall {
				return &gov.GetProposalsCall{
					ProposalStatus: uint32(govv1.StatusNil),
					Pagination: cmn.PageRequest{
						Limit:      10,
						CountTotal: true,
					},
				}
			},
			func(data []gov.ProposalData, pageRes *cmn.PageResponse) {
				s.Require().Len(data, 2)
				s.Require().Equal(uint64(2), pageRes.Total)

				proposal := data[0]
				s.Require().Equal(uint64(1), proposal.Id)
				s.Require().Equal(uint32(govv1.StatusVotingPeriod), proposal.Status)
				s.Require().Equal(s.keyring.GetAddr(0), proposal.Proposer)
				s.Require().Equal("test prop", proposal.Title)
				s.Require().Equal("test prop", proposal.Summary)
				s.Require().Equal("ipfs://CID", proposal.Metadata)
				s.Require().Len(proposal.Messages, 1)
				s.Require().Equal("/cosmos.bank.v1beta1.MsgSend", proposal.Messages[0])
			},
			200000,
			false,
			"",
		},
		{
			"success - filter by status",
			func() *gov.GetProposalsCall {
				return &gov.GetProposalsCall{
					ProposalStatus: uint32(govv1.StatusVotingPeriod),
					Pagination: cmn.PageRequest{
						Limit:      10,
						CountTotal: true,
					},
				}
			},
			func(data []gov.ProposalData, pageRes *cmn.PageResponse) {
				s.Require().Len(data, 2)
				s.Require().Equal(uint64(2), pageRes.Total)
				s.Require().Equal(uint32(govv1.StatusVotingPeriod), data[0].Status)
				s.Require().Equal(uint32(govv1.StatusVotingPeriod), data[1].Status)
			},
			200000,
			false,
			"",
		},
		{
			"success - filter by voter",
			func() *gov.GetProposalsCall {
				// First add a vote
				err := s.network.App.GetGovKeeper().AddVote(s.network.GetContext(), 1, s.keyring.GetAccAddr(0), govv1.NewNonSplitVoteOption(govv1.OptionYes), "")
				s.Require().NoError(err)

				return &gov.GetProposalsCall{
					ProposalStatus: uint32(govv1.StatusVotingPeriod),
					Voter:          s.keyring.GetAddr(0),
					Pagination: cmn.PageRequest{
						Limit:      10,
						CountTotal: true,
					},
				}
			},
			func(data []gov.ProposalData, pageRes *cmn.PageResponse) {
				s.Require().Len(data, 1)
				s.Require().Equal(uint64(1), pageRes.Total)
			},
			200000,
			false,
			"",
		},
		{
			"success - filter by depositor",
			func() *gov.GetProposalsCall {
				return &gov.GetProposalsCall{
					ProposalStatus: uint32(govv1.StatusVotingPeriod),
					Depositor:      s.keyring.GetAddr(0),
					Pagination: cmn.PageRequest{
						Limit:      10,
						CountTotal: true,
					},
				}
			},
			func(data []gov.ProposalData, pageRes *cmn.PageResponse) {
				s.Require().Len(data, 1)
				s.Require().Equal(uint64(1), pageRes.Total)
			},
			200000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			_, ctx := testutil.NewPrecompileContract(s.T(), s.network.GetContext(), s.keyring.GetAddr(0), s.precompile.Address(), tc.gas)

			out, err := s.precompile.GetProposals(ctx, *tc.malleate())

			if tc.expError {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				tc.postCheck(out.Proposals, &out.PageResponse)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestGetParams() {
	testCases := []struct {
		name        string
		malleate    func() *gov.GetParamsCall
		expPass     bool
		errContains string
	}{
		{
			"success - get all params",
			func() *gov.GetParamsCall {
				return &gov.GetParamsCall{}
			},
			true,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			_, err := s.precompile.GetParams(s.network.GetContext(), *tc.malleate())

			if tc.expPass {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			}
		})
	}
}
