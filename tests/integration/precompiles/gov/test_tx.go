package gov

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/gov"
	"github.com/cosmos/evm/precompiles/testutil"
	utiltx "github.com/cosmos/evm/testutil/tx"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *PrecompileTestSuite) TestVote() {
	var ctx sdk.Context
	method := s.precompile.Methods[gov.VoteMethod]
	newVoterAddr := utiltx.GenerateAddress()
	const proposalID uint64 = 1
	const option uint8 = 1
	const metadata = "metadata"

	testCases := []struct {
		name      string
		malleate  func() []interface{}
		postCheck func()
		gas       uint64
		expError  bool
		wantErrFn func(*PrecompileTestSuite, []interface{}) error
	}{
		{
			"fail - empty input args",
			func() []interface{} {
				return []interface{}{}
			},
			func() {},
			200000,
			true,
			func(*PrecompileTestSuite, []interface{}) error {
				return cmn.NewRevertWithSolidityError(gov.ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(4), big.NewInt(0))
			},
		},
		{
			"fail - invalid voter address",
			func() []interface{} {
				return []interface{}{
					"",
					proposalID,
					option,
					metadata,
				}
			},
			func() {},
			200000,
			true,
			func(*PrecompileTestSuite, []interface{}) error {
				return cmn.NewRevertWithSolidityError(gov.ABI, cmn.SolidityErrInvalidAddress, "")
			},
		},
		{
			"fail - invalid voter address",
			func() []interface{} {
				return []interface{}{
					common.Address{},
					proposalID,
					option,
					metadata,
				}
			},
			func() {},
			200000,
			true,
			func(*PrecompileTestSuite, []interface{}) error {
				return cmn.NewRevertWithSolidityError(gov.ABI, cmn.SolidityErrInvalidAddress, common.Address{}.String())
			},
		},
		{
			"fail - using a different voter address",
			func() []interface{} {
				return []interface{}{
					newVoterAddr,
					proposalID,
					option,
					metadata,
				}
			},
			func() {},
			200000,
			true,
			func(s *PrecompileTestSuite, args []interface{}) error {
				return cmn.NewRevertWithSolidityError(gov.ABI, cmn.SolidityErrRequesterIsNotMsgSender, s.keyring.GetAddr(0), args[0].(common.Address))
			},
		},
		{
			"fail - invalid vote option",
			func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(0),
					proposalID,
					option + 10,
					metadata,
				}
			},
			func() {},
			200000,
			true,
			func(*PrecompileTestSuite, []interface{}) error {
				return cmn.NewRevertWithSolidityError(gov.ABI, cmn.SolidityErrMsgServerFailed, gov.VoteMethod, "11: invalid vote option")
			},
		},
		{
			"success - vote proposal success",
			func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(0),
					proposalID,
					option,
					metadata,
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
			nil,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()

			var contract *vm.Contract
			contract, ctx = testutil.NewPrecompileContract(s.T(), ctx, s.keyring.GetAddr(0), s.precompile.Address(), tc.gas)

			voteArgs := tc.malleate()
			_, err := s.precompile.Vote(ctx, contract, s.network.GetStateDB(), &method, voteArgs)

			if tc.expError {
				testutil.RequireExactError(s.T(), err, tc.wantErrFn(s, voteArgs))
			} else {
				s.Require().NoError(err)
				tc.postCheck()
			}
		})
	}
}

func (s *PrecompileTestSuite) TestVoteWeighted() {
	var ctx sdk.Context
	method := s.precompile.Methods[gov.VoteWeightedMethod]
	newVoterAddr := utiltx.GenerateAddress()
	const proposalID uint64 = 1
	const metadata = "metadata"

	testCases := []struct {
		name      string
		malleate  func() []interface{}
		postCheck func()
		gas       uint64
		expError  bool
		wantErrFn func(*PrecompileTestSuite, []interface{}) error
	}{
		{
			"fail - empty input args",
			func() []interface{} {
				return []interface{}{}
			},
			func() {},
			200000,
			true,
			func(*PrecompileTestSuite, []interface{}) error {
				return cmn.NewRevertWithSolidityError(gov.ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(4), big.NewInt(0))
			},
		},
		{
			"fail - invalid voter address",
			func() []interface{} {
				return []interface{}{
					"",
					proposalID,
					[]gov.WeightedVoteOption{},
					metadata,
				}
			},
			func() {},
			200000,
			true,
			func(*PrecompileTestSuite, []interface{}) error {
				return cmn.NewRevertWithSolidityError(gov.ABI, cmn.SolidityErrInvalidAddress, "")
			},
		},
		{
			"fail - using a different voter address",
			func() []interface{} {
				return []interface{}{
					newVoterAddr,
					proposalID,
					[]gov.WeightedVoteOption{},
					metadata,
				}
			},
			func() {},
			200000,
			true,
			func(s *PrecompileTestSuite, args []interface{}) error {
				return cmn.NewRevertWithSolidityError(gov.ABI, cmn.SolidityErrRequesterIsNotMsgSender, s.keyring.GetAddr(0), args[0].(common.Address))
			},
		},
		{
			"fail - invalid vote option",
			func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(0),
					proposalID,
					[]gov.WeightedVoteOption{{Option: 10, Weight: "1.0"}},
					metadata,
				}
			},
			func() {},
			200000,
			true,
			func(*PrecompileTestSuite, []interface{}) error {
				return cmn.NewRevertWithSolidityError(gov.ABI, cmn.SolidityErrMsgServerFailed, gov.VoteWeightedMethod, `option:10 weight:"1.0" : invalid vote option`)
			},
		},
		{
			"fail - invalid weight sum",
			func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(0),
					proposalID,
					[]gov.WeightedVoteOption{
						{Option: 1, Weight: "0.5"},
						{Option: 2, Weight: "0.6"},
					},
					metadata,
				}
			},
			func() {},
			200000,
			true,
			func(*PrecompileTestSuite, []interface{}) error {
				return cmn.NewRevertWithSolidityError(gov.ABI, cmn.SolidityErrMsgServerFailed, gov.VoteWeightedMethod, "total weight overflow 1.00: invalid vote option")
			},
		},
		{
			"success - vote weighted proposal",
			func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(0),
					proposalID,
					[]gov.WeightedVoteOption{
						{Option: 1, Weight: "0.7"},
						{Option: 2, Weight: "0.3"},
					},
					metadata,
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
			nil,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()

			var contract *vm.Contract
			contract, ctx = testutil.NewPrecompileContract(s.T(), ctx, s.keyring.GetAddr(0), s.precompile.Address(), tc.gas)

			voteWeightedArgs := tc.malleate()
			_, err := s.precompile.VoteWeighted(ctx, contract, s.network.GetStateDB(), &method, voteWeightedArgs)

			if tc.expError {
				testutil.RequireExactError(s.T(), err, tc.wantErrFn(s, voteWeightedArgs))
			} else {
				s.Require().NoError(err)
				tc.postCheck()
			}
		})
	}
}
