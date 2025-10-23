package gov

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/gov"
	"github.com/cosmos/evm/x/vm/statedb"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *PrecompileTestSuite) TestVoteEvent() {
	var (
		stDB *statedb.StateDB
		ctx  sdk.Context
	)

	testCases := []struct {
		name        string
		malleate    func(voter common.Address, proposalId uint64, option uint8, metadata string) *gov.VoteCall
		postCheck   func()
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"success - the correct event is emitted",
			func(voter common.Address, proposalId uint64, option uint8, metadata string) *gov.VoteCall {
				return &gov.VoteCall{
					Voter:      voter,
					ProposalId: proposalId,
					Option:     option,
					Metadata:   metadata,
				}
			},
			func() {
				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				// Check event signature matches the one emitted
				s.Require().Equal(gov.VoteEventTopic, common.HexToHash(log.Topics[0].Hex()))
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				// Check the fully unpacked event matches the one emitted
				var voteEvent gov.VoteEvent
				err := cmn.UnpackLog(&voteEvent, *log)
				s.Require().NoError(err)
				s.Require().Equal(s.keyring.GetAddr(0), voteEvent.Voter)
				s.Require().Equal(uint64(1), voteEvent.ProposalId)
				s.Require().Equal(uint8(1), voteEvent.Option)
			},
			20000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.SetupTest()
		stDB = s.network.GetStateDB()
		ctx = s.network.GetContext()

		contract := vm.NewContract(s.keyring.GetAddr(0), s.precompile.Address(), uint256.NewInt(0), tc.gas, nil)
		ctx = ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
		initialGas := ctx.GasMeter().GasConsumed()
		s.Require().Zero(initialGas)

		_, err := s.precompile.Vote(ctx, tc.malleate(s.keyring.GetAddr(0), 1, 1, "metadata"), stDB, contract)

		if tc.expError {
			s.Require().Error(err)
			s.Require().Contains(err.Error(), tc.errContains)
		} else {
			s.Require().NoError(err)
			tc.postCheck()
		}
	}
}

func (s *PrecompileTestSuite) TestVoteWeightedEvent() {
	var (
		stDB *statedb.StateDB
		ctx  sdk.Context
	)

	testCases := []struct {
		name        string
		malleate    func(voter common.Address, proposalId uint64, options gov.WeightedVoteOptions) *gov.VoteWeightedCall
		postCheck   func()
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"success - the correct VoteWeighted event is emitted",
			func(voter common.Address, proposalId uint64, options gov.WeightedVoteOptions) *gov.VoteWeightedCall {
				return &gov.VoteWeightedCall{
					Voter:      voter,
					ProposalId: proposalId,
					Options:    options,
					Metadata:   "",
				}
			},
			func() {
				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				// Check event signature matches the one emitted
				s.Require().Equal(gov.VoteWeightedEventTopic, common.HexToHash(log.Topics[0].Hex()))
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				// Check the fully unpacked event matches the one emitted
				var voteWeightedEvent gov.VoteWeightedEvent
				err := cmn.UnpackLog(&voteWeightedEvent, *log)
				s.Require().NoError(err)
				s.Require().Equal(s.keyring.GetAddr(0), voteWeightedEvent.Voter)
				s.Require().Equal(uint64(1), voteWeightedEvent.ProposalId)
				s.Require().Equal(2, len(voteWeightedEvent.Options))
				s.Require().Equal(uint8(1), voteWeightedEvent.Options[0].Option)
				s.Require().Equal("0.70", voteWeightedEvent.Options[0].Weight)
				s.Require().Equal(uint8(2), voteWeightedEvent.Options[1].Option)
				s.Require().Equal("0.30", voteWeightedEvent.Options[1].Weight)
			},
			20000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			stDB = s.network.GetStateDB()
			ctx = s.network.GetContext()

			contract := vm.NewContract(s.keyring.GetAddr(0), s.precompile.Address(), uint256.NewInt(0), tc.gas, nil)
			ctx = ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
			initialGas := ctx.GasMeter().GasConsumed()
			s.Require().Zero(initialGas)

			options := gov.WeightedVoteOptions{
				{Option: 1, Weight: "0.70"},
				{Option: 2, Weight: "0.30"},
			}

			_, err := s.precompile.VoteWeighted(ctx, tc.malleate(s.keyring.GetAddr(0), 1, options), stDB, contract)

			if tc.expError {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				tc.postCheck()
			}
		})
	}
}
