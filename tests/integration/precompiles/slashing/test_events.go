package slashing

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/yihuang/go-abi"

	"github.com/cosmos/evm/precompiles/slashing"
	"github.com/cosmos/evm/x/vm/statedb"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *PrecompileTestSuite) TestUnjailEvent() {
	var (
		stateDB *statedb.StateDB
		ctx     sdk.Context
	)

	testCases := []struct {
		name        string
		malleate    func() common.Address
		postCheck   func()
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"success - the correct event is emitted",
			func() common.Address {
				validator, err := s.network.App.GetStakingKeeper().GetValidator(ctx, sdk.ValAddress(s.keyring.GetAccAddr(0)))
				s.Require().NoError(err)

				consAddr, err := validator.GetConsAddr()
				s.Require().NoError(err)

				err = s.network.App.GetSlashingKeeper().Jail(
					s.network.GetContext(),
					consAddr,
				)
				s.Require().NoError(err)

				return s.keyring.GetAddr(0)
			},
			func() {
				log := stateDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				// Check event signature matches the one emitted
				s.Require().Equal(slashing.ValidatorUnjailedEventTopic, common.HexToHash(log.Topics[0].Hex()))
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				// Check the validator address in the event matches
				var hash common.Hash
				_, err := abi.EncodeAddress(s.keyring.GetAddr(0), hash[:])
				s.Require().NoError(err)

				s.Require().Equal(hash, log.Topics[1])

				// Check the fully unpacked event matches the one emitted
				var unjailEvent slashing.ValidatorUnjailedEvent
				err = unjailEvent.DecodeTopics(log.Topics)
				s.Require().NoError(err)
				s.Require().Equal(s.keyring.GetAddr(0), unjailEvent.Validator)
			},
			20000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			stateDB = s.network.GetStateDB()
			ctx = s.network.GetContext()

			contract := vm.NewContract(s.keyring.GetAddr(0), s.precompile.Address(), uint256.NewInt(0), tc.gas, nil)
			ctx = ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
			initialGas := ctx.GasMeter().GasConsumed()
			s.Require().Zero(initialGas)

			method := slashing.UnjailCall{
				ValidatorAddress: tc.malleate(),
			}
			_, err := s.precompile.Unjail(ctx, method, stateDB, contract)

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
