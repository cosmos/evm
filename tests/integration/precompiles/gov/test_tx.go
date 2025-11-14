package gov

import (
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/evm/precompiles/gov"
	"github.com/cosmos/evm/precompiles/testutil"
	utiltx "github.com/cosmos/evm/testutil/tx"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *PrecompileTestSuite) TestVote() {
	var ctx sdk.Context
	newVoterAddr := utiltx.GenerateAddress()
	const proposalID uint64 = 1
	const option uint8 = 1
	const metadata = "metadata"

	testCases := []struct {
		name        string
		malleate    func() *gov.VoteCall
		postCheck   func()
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"fail - invalid voter address",
			func() *gov.VoteCall {
				return &gov.VoteCall{
					ProposalId: proposalID,
					Option:     option,
					Metadata:   metadata,
				}
			},
			func() {},
			200000,
			true,
			"invalid voter address",
		},
		{
			"fail - invalid voter address",
			func() *gov.VoteCall {
				return &gov.VoteCall{
					ProposalId: proposalID,
					Option:     option,
					Metadata:   metadata,
				}
			},
			func() {},
			200000,
			true,
			"invalid voter address",
		},
		{
			"fail - using a different voter address",
			func() *gov.VoteCall {
				return &gov.VoteCall{
					Voter:      newVoterAddr,
					ProposalId: proposalID,
					Option:     option,
					Metadata:   metadata,
				}
			},
			func() {},
			200000,
			true,
			"does not match the requester address",
		},
		{
			"fail - invalid vote option",
			func() *gov.VoteCall {
				return &gov.VoteCall{
					Voter:      s.keyring.GetAddr(0),
					ProposalId: proposalID,
					Option:     option + 10,
					Metadata:   metadata,
				}
			},
			func() {},
			200000,
			true,
			"invalid vote option",
		},
		{
			"success - vote proposal success",
			func() *gov.VoteCall {
				return &gov.VoteCall{
					Voter:      s.keyring.GetAddr(0),
					ProposalId: proposalID,
					Option:     option,
					Metadata:   metadata,
				}
			},
			func() {
				proposal, _ := s.network.App.GetGovKeeper().Proposals.Get(ctx, proposalID)
				_, _, tallyResult, err := s.network.App.GetGovKeeper().Tally(ctx, proposal)
				s.Require().NoError(err)
				s.Require().Equal(math.NewInt(3e18).String(), tallyResult.YesCount)
			},
			200000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()

			var contract *vm.Contract
			contract, ctx = testutil.NewPrecompileContract(s.T(), ctx, s.keyring.GetAddr(0), s.precompile.Address(), tc.gas)

			_, err := s.precompile.Vote(ctx, *tc.malleate(), s.network.GetStateDB(), contract)

			if tc.expError {
				s.Require().ErrorContains(err, tc.errContains)
			} else {
				s.Require().NoError(err)
				tc.postCheck()
			}
		})
	}
}

func (s *PrecompileTestSuite) TestVoteWeighted() {
	var ctx sdk.Context
	newVoterAddr := utiltx.GenerateAddress()
	const proposalID uint64 = 1
	const metadata = "metadata"

	testCases := []struct {
		name        string
		malleate    func() *gov.VoteWeightedCall
		postCheck   func()
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"fail - invalid voter address",
			func() *gov.VoteWeightedCall {
				return &gov.VoteWeightedCall{
					ProposalId: proposalID,
					Options:    []gov.WeightedVoteOption{},
					Metadata:   metadata,
				}
			},
			func() {},
			200000,
			true,
			"invalid voter address",
		},
		{
			"fail - using a different voter address",
			func() *gov.VoteWeightedCall {
				return &gov.VoteWeightedCall{
					Voter:      newVoterAddr,
					ProposalId: proposalID,
					Options:    []gov.WeightedVoteOption{},
					Metadata:   metadata,
				}
			},
			func() {},
			200000,
			true,
			"does not match the requester address",
		},
		{
			"fail - invalid vote option",
			func() *gov.VoteWeightedCall {
				return &gov.VoteWeightedCall{
					Voter:      s.keyring.GetAddr(0),
					ProposalId: proposalID,
					Options:    []gov.WeightedVoteOption{{Option: 10, Weight: "1.0"}},
					Metadata:   metadata,
				}
			},
			func() {},
			200000,
			true,
			"invalid vote option",
		},
		{
			"fail - invalid weight sum",
			func() *gov.VoteWeightedCall {
				return &gov.VoteWeightedCall{
					Voter:      s.keyring.GetAddr(0),
					ProposalId: proposalID,
					Options: []gov.WeightedVoteOption{
						{Option: 1, Weight: "0.5"},
						{Option: 2, Weight: "0.6"},
					},
					Metadata: metadata,
				}
			},
			func() {},
			200000,
			true,
			"total weight overflow 1.00",
		},
		{
			"success - vote weighted proposal",
			func() *gov.VoteWeightedCall {
				return &gov.VoteWeightedCall{
					Voter:      s.keyring.GetAddr(0),
					ProposalId: proposalID,
					Options: []gov.WeightedVoteOption{
						{Option: 1, Weight: "0.7"},
						{Option: 2, Weight: "0.3"},
					},
					Metadata: metadata,
				}
			},
			func() {
				proposal, _ := s.network.App.GetGovKeeper().Proposals.Get(ctx, proposalID)
				_, _, tallyResult, err := s.network.App.GetGovKeeper().Tally(ctx, proposal)
				s.Require().NoError(err)
				s.Require().Equal("2100000000000000000", tallyResult.YesCount)
				s.Require().Equal("900000000000000000", tallyResult.AbstainCount)
			},
			200000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()

			var contract *vm.Contract
			contract, ctx = testutil.NewPrecompileContract(s.T(), ctx, s.keyring.GetAddr(0), s.precompile.Address(), tc.gas)

			_, err := s.precompile.VoteWeighted(ctx, *tc.malleate(), s.network.GetStateDB(), contract)

			if tc.expError {
				s.Require().ErrorContains(err, tc.errContains)
			} else {
				s.Require().NoError(err)
				tc.postCheck()
			}
		})
	}
}
