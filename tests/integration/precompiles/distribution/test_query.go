package distribution

import (
	"fmt"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/distribution"
	"github.com/cosmos/evm/testutil"
	testutiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/yihuang/go-abi"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/testutil/mock"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var expValAmount int64 = 1

type distrTestCases struct {
	name        string
	malleate    func() []interface{}
	postCheck   func(bz []byte)
	gas         uint64
	expErr      bool
	errContains string
}

var baseTestCases = []distrTestCases{
	{
		"fail - empty input args",
		func() []interface{} {
			return []interface{}{}
		},
		func([]byte) {},
		100000,
		true,
		"invalid number of arguments",
	},
	{
		"fail - invalid validator address",
		func() []interface{} {
			return []interface{}{
				"invalid",
			}
		},
		func([]byte) {},
		100000,
		true,
		"invalid: unknown address",
	},
}

func (s *PrecompileTestSuite) TestValidatorDistributionInfo() {
	var ctx sdk.Context
	testCases := []distrTestCases{
		{
			"fail - nonexistent validator address",
			func() []interface{} {
				pv := mock.NewPV()
				pk, err := pv.GetPubKey()
				s.Require().NoError(err)
				return []interface{}{
					sdk.ValAddress(pk.Address().Bytes()).String(),
				}
			},
			func([]byte) {},
			100000,
			true,
			"validator does not exist",
		},
		{
			"fail - existent validator but without self delegation",
			func() []interface{} {
				return []interface{}{
					s.network.GetValidators()[0].OperatorAddress,
				}
			},
			func([]byte) {},
			100000,
			true,
			"no delegation for (address, validator) tuple",
		},
		{
			"success",
			func() []interface{} {
				valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
				s.Require().NoError(err)
				s.Require().NoError(err)

				// fund account for self delegation
				amt := math.NewInt(1)
				err = s.fundAccountWithBaseDenom(ctx, valAddr.Bytes(), amt)
				s.Require().NoError(err)

				// make a self delegation
				_, err = s.network.App.GetStakingKeeper().Delegate(ctx, valAddr.Bytes(), amt, stakingtypes.Unspecified, s.network.GetValidators()[0], true)
				s.Require().NoError(err)
				return []interface{}{
					s.network.GetValidators()[0].OperatorAddress,
				}
			},
			func(bz []byte) {
				var out distribution.ValidatorDistributionInfoReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)

				valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
				s.Require().NoError(err)

				s.Require().Equal(sdk.AccAddress(valAddr.Bytes()).String(), out.DistributionInfo.OperatorAddress)
				s.Require().Equal(0, len(out.DistributionInfo.Commission))
				s.Require().Equal(0, len(out.DistributionInfo.SelfBondRewards))
			},
			100000,
			false,
			"",
		},
	}
	testCases = append(testCases, baseTestCases...)

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()
			args, err := testutil.CallFunction(distribution.NewValidatorDistributionInfoCall, tc.malleate())
			s.Require().NoError(err)

			out, err := s.precompile.ValidatorDistributionInfo(ctx, *args.(*distribution.ValidatorDistributionInfoCall))

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotEmpty(out)
				bz, err := out.Encode()
				s.Require().NoError(err)
				tc.postCheck(bz)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestValidatorOutstandingRewards() {
	var ctx sdk.Context
	testCases := []distrTestCases{
		{
			"fail - nonexistent validator address",
			func() []interface{} {
				pv := mock.NewPV()
				pk, err := pv.GetPubKey()
				s.Require().NoError(err)
				return []interface{}{
					sdk.ValAddress(pk.Address().Bytes()).String(),
				}
			},
			func(bz []byte) {
				var out distribution.ValidatorOutstandingRewardsReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)
				s.Require().Equal(0, len(out.Rewards))
			},
			100000,
			true,
			"validator does not exist",
		},
		{
			"success - existent validator, no outstanding rewards",
			func() []interface{} {
				return []interface{}{
					s.network.GetValidators()[0].OperatorAddress,
				}
			},
			func(bz []byte) {
				var out distribution.ValidatorOutstandingRewardsReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)
				s.Require().Equal(0, len(out.Rewards))
			},
			100000,
			false,
			"",
		},
		{
			"success - with outstanding rewards",
			func() []interface{} {
				valRewards := sdk.DecCoins{sdk.NewDecCoinFromDec(s.bondDenom, math.LegacyNewDec(1))}
				// set outstanding rewards
				valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
				s.Require().NoError(err)

				err = s.network.App.GetDistrKeeper().SetValidatorOutstandingRewards(ctx, valAddr, types.ValidatorOutstandingRewards{Rewards: valRewards})
				s.Require().NoError(err)

				return []interface{}{
					s.network.GetValidators()[0].OperatorAddress,
				}
			},
			func(bz []byte) {
				var out distribution.ValidatorOutstandingRewardsReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)
				s.Require().Equal(1, len(out.Rewards))
				s.Require().Equal(uint8(18), out.Rewards[0].Precision)
				s.Require().Equal(s.bondDenom, out.Rewards[0].Denom)
				s.Require().Equal(expValAmount, out.Rewards[0].Amount.Int64())
			},
			100000,
			false,
			"",
		},
	}
	testCases = append(testCases, baseTestCases...)

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()
			args, err := testutil.CallFunction(distribution.NewValidatorOutstandingRewardsCall, tc.malleate())
			s.Require().NoError(err)
			out, err := s.precompile.ValidatorOutstandingRewards(ctx, *args.(*distribution.ValidatorOutstandingRewardsCall))

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotEmpty(out)
				bz, err := out.Encode()
				s.Require().NoError(err)
				tc.postCheck(bz)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestValidatorCommission() {
	var ctx sdk.Context
	testCases := []distrTestCases{
		{
			"fail - nonexistent validator address",
			func() []interface{} {
				pv := mock.NewPV()
				pk, err := pv.GetPubKey()
				s.Require().NoError(err)
				return []interface{}{
					sdk.ValAddress(pk.Address().Bytes()).String(),
				}
			},
			func(bz []byte) {
				var out distribution.ValidatorCommissionReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)
				s.Require().Equal(0, len(out.Commission))
			},
			100000,
			true,
			"validator does not exist",
		},
		{
			"success - existent validator, no accumulated commission",
			func() []interface{} {
				return []interface{}{
					s.network.GetValidators()[0].OperatorAddress,
				}
			},
			func(bz []byte) {
				var out distribution.ValidatorCommissionReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)
				s.Require().Equal(0, len(out.Commission))
			},
			100000,
			false,
			"",
		},
		{
			"success - with accumulated commission",
			func() []interface{} {
				commAmt := math.LegacyNewDec(1)
				validator := s.network.GetValidators()[0]
				valAddr, err := sdk.ValAddressFromBech32(validator.GetOperator())
				s.Require().NoError(err)
				valCommission := sdk.DecCoins{sdk.NewDecCoinFromDec(s.bondDenom, commAmt)}
				err = s.network.App.GetDistrKeeper().SetValidatorAccumulatedCommission(ctx, valAddr, types.ValidatorAccumulatedCommission{Commission: valCommission})
				s.Require().NoError(err)

				// set distribution module account balance which pays out the commission
				coins := sdk.NewCoins(sdk.NewCoin(s.bondDenom, commAmt.RoundInt()))
				err = s.mintCoinsForDistrMod(ctx, coins)
				s.Require().NoError(err)

				return []interface{}{
					validator.OperatorAddress,
				}
			},
			func(bz []byte) {
				var out distribution.ValidatorCommissionReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)
				s.Require().Equal(1, len(out.Commission))
				s.Require().Equal(uint8(18), out.Commission[0].Precision)
				s.Require().Equal(s.bondDenom, out.Commission[0].Denom)
				s.Require().Equal(expValAmount, out.Commission[0].Amount.Int64())
			},
			100000,
			false,
			"",
		},
	}
	testCases = append(testCases, baseTestCases...)

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()
			args, err := testutil.CallFunction(distribution.NewValidatorCommissionCall, tc.malleate())
			s.Require().NoError(err)
			out, err := s.precompile.ValidatorCommission(ctx, *args.(*distribution.ValidatorCommissionCall))

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotEmpty(out)
				bz, err := out.Encode()
				s.Require().NoError(err)
				tc.postCheck(bz)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestValidatorSlashes() {
	var ctx sdk.Context
	testCases := []distrTestCases{
		{
			"fail - invalid validator address",
			func() []interface{} {
				return []interface{}{
					"invalid", uint64(1), uint64(5), query.PageRequest{},
				}
			},
			func([]byte) {
			},
			100000,
			true,
			"invalid validator address",
		},
		{
			"fail - invalid starting height type",
			func() []interface{} {
				return []interface{}{
					s.network.GetValidators()[0].OperatorAddress,
					int64(1), uint64(5),
					query.PageRequest{},
				}
			},
			func([]byte) {
			},
			100000,
			true,
			"invalid type for startingHeight: expected uint64, received int64",
		},
		{
			"fail - starting height greater than ending height",
			func() []interface{} {
				return []interface{}{
					s.network.GetValidators()[0].OperatorAddress,
					uint64(6), uint64(5),
					query.PageRequest{},
				}
			},
			func([]byte) {
			},
			100000,
			true,
			"starting height greater than ending height",
		},
		{
			"success - nonexistent validator address",
			func() []interface{} {
				pv := mock.NewPV()
				pk, err := pv.GetPubKey()
				s.Require().NoError(err)
				return []interface{}{
					sdk.ValAddress(pk.Address().Bytes()).String(),
					uint64(1),
					uint64(5),
					query.PageRequest{},
				}
			},
			func(bz []byte) {
				var out distribution.ValidatorSlashesReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output")
				s.Require().Equal(0, len(out.Slashes))
				s.Require().Equal(uint64(0), out.PageResponse.Total)
			},
			100000,
			false,
			"",
		},
		{
			"success - existent validator, no slashes",
			func() []interface{} {
				return []interface{}{
					s.network.GetValidators()[0].OperatorAddress,
					uint64(1),
					uint64(5),
					query.PageRequest{},
				}
			},
			func(bz []byte) {
				var out distribution.ValidatorSlashesReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output")
				s.Require().Equal(0, len(out.Slashes))
				s.Require().Equal(uint64(0), out.PageResponse.Total)
			},
			100000,
			false,
			"",
		},
		{
			"success - with slashes",
			func() []interface{} {
				valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
				s.Require().NoError(err)
				err = s.network.App.GetDistrKeeper().SetValidatorSlashEvent(ctx, valAddr, 2, 1, types.ValidatorSlashEvent{ValidatorPeriod: 1, Fraction: math.LegacyNewDec(5)})
				s.Require().NoError(err)
				return []interface{}{
					s.network.GetValidators()[0].OperatorAddress,
					uint64(1), uint64(5),
					query.PageRequest{},
				}
			},
			func(bz []byte) {
				var out distribution.ValidatorSlashesReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output")
				s.Require().Equal(1, len(out.Slashes))
				s.Require().Equal(math.LegacyNewDec(5).BigInt(), out.Slashes[0].Fraction.Value)
				s.Require().Equal(uint64(1), out.Slashes[0].ValidatorPeriod)
				s.Require().Equal(uint64(1), out.PageResponse.Total)
			},
			100000,
			false,
			"",
		},
		{
			"success - with slashes w/pagination",
			func() []interface{} {
				valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
				s.Require().NoError(err)
				err = s.network.App.GetDistrKeeper().SetValidatorSlashEvent(ctx, valAddr, 2, 1, types.ValidatorSlashEvent{ValidatorPeriod: 1, Fraction: math.LegacyNewDec(5)})
				s.Require().NoError(err)
				return []interface{}{
					s.network.GetValidators()[0].OperatorAddress,
					uint64(1),
					uint64(5),
					query.PageRequest{Limit: 1, CountTotal: true},
				}
			},
			func(bz []byte) {
				var out distribution.ValidatorSlashesReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output")
				s.Require().Equal(1, len(out.Slashes))
				s.Require().Equal(math.LegacyNewDec(5).BigInt(), out.Slashes[0].Fraction.Value)
				s.Require().Equal(uint64(1), out.Slashes[0].ValidatorPeriod)
				s.Require().Equal(uint64(1), out.PageResponse.Total)
			},
			100000,
			false,
			"",
		},
	}
	testCases = append(testCases, baseTestCases[0])

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()
			args, err := testutil.CallFunction(distribution.NewValidatorSlashesCall, tc.malleate())
			s.Require().NoError(err)
			out, err := s.precompile.ValidatorSlashes(ctx, *args.(*distribution.ValidatorSlashesCall))

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotEmpty(out)
				bz, err := out.Encode()
				s.Require().NoError(err)
				tc.postCheck(bz)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestDelegationRewards() {
	var (
		ctx sdk.Context
		err error
	)
	testCases := []distrTestCases{
		{
			"fail - invalid validator address",
			func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(0),
					"invalid",
				}
			},
			func([]byte) {},
			100000,
			true,
			"invalid: unknown address",
		},
		{
			"fail - nonexistent validator address",
			func() []interface{} {
				pv := mock.NewPV()
				pk, err := pv.GetPubKey()
				s.Require().NoError(err)
				return []interface{}{
					s.keyring.GetAddr(0),
					sdk.ValAddress(pk.Address().Bytes()).String(),
				}
			},
			func([]byte) {},
			100000,
			true,
			"validator does not exist",
		},
		{
			"fail - existent validator, no delegation",
			func() []interface{} {
				newAddr, _ := testutiltx.NewAddrKey()
				return []interface{}{
					newAddr,
					s.network.GetValidators()[0].OperatorAddress,
				}
			},
			func([]byte) {},
			100000,
			true,
			"no delegation for (address, validator) tuple",
		},
		{
			"success - existent validator & delegation, but no rewards",
			func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(0),
					s.network.GetValidators()[0].OperatorAddress,
				}
			},
			func(bz []byte) {
				var out distribution.DelegationRewardsReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)
				s.Require().Equal(0, len(out.Rewards))
			},
			100000,
			false,
			"",
		},
		{
			"success - with rewards",
			func() []interface{} {
				ctx, err = s.prepareStakingRewards(ctx, stakingRewards{s.keyring.GetAddr(0).Bytes(), s.network.GetValidators()[0], testRewardsAmt})
				s.Require().NoError(err, "failed to prepare staking rewards", err)
				return []interface{}{
					s.keyring.GetAddr(0),
					s.network.GetValidators()[0].OperatorAddress,
				}
			},
			func(bz []byte) {
				var out distribution.DelegationRewardsReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)
				s.Require().Equal(1, len(out.Rewards))
				s.Require().Equal(uint8(18), out.Rewards[0].Precision)
				s.Require().Equal(s.bondDenom, out.Rewards[0].Denom)
				s.Require().Equal(expRewardsAmt.Int64(), out.Rewards[0].Amount.Int64())
			},
			100000,
			false,
			"",
		},
	}
	testCases = append(testCases, baseTestCases[0])

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()
			args, err := testutil.CallFunction(distribution.NewDelegationRewardsCall, tc.malleate())
			s.Require().NoError(err)
			out, err := s.precompile.DelegationRewards(ctx, *args.(*distribution.DelegationRewardsCall))

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotEmpty(out)
				bz, err := out.Encode()
				s.Require().NoError(err)
				tc.postCheck(bz)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestDelegationTotalRewards() {
	var (
		ctx sdk.Context
		err error
	)
	testCases := []distrTestCases{
		{
			"fail - invalid delegator address",
			func() []interface{} {
				return []interface{}{
					"invalid",
				}
			},
			func([]byte) {},
			100000,
			true,
			fmt.Sprintf(cmn.ErrInvalidDelegator, "invalid"),
		},
		{
			"success - no delegations",
			func() []interface{} {
				newAddr, _ := testutiltx.NewAddrKey()
				return []interface{}{
					newAddr,
				}
			},
			func(bz []byte) {
				var out distribution.DelegationTotalRewardsReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)
				s.Require().Equal(0, len(out.Rewards))
				s.Require().Equal(0, len(out.Total))
			},
			100000,
			false,
			"",
		},
		{
			"success - existent validator & delegation, but no rewards",
			func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(0),
				}
			},
			func(bz []byte) {
				var out distribution.DelegationTotalRewardsReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)

				validatorsCount := len(s.network.GetValidators())
				s.Require().Equal(validatorsCount, len(out.Rewards))

				// no rewards
				s.Require().Equal(0, len(out.Rewards[0].Reward))
				s.Require().Equal(0, len(out.Rewards[1].Reward))
				s.Require().Equal(0, len(out.Rewards[2].Reward))
				s.Require().Equal(0, len(out.Total))
			},
			100000,
			false,
			"",
		},
		{
			"success - with rewards",
			func() []interface{} {
				ctx, err = s.prepareStakingRewards(ctx, stakingRewards{s.keyring.GetAccAddr(0), s.network.GetValidators()[0], testRewardsAmt})
				s.Require().NoError(err, "failed to prepare staking rewards", err)

				return []interface{}{
					s.keyring.GetAddr(0),
				}
			},
			func(bz []byte) {
				var (
					i int
				)
				var out distribution.DelegationTotalRewardsReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)

				validators := s.network.GetValidators()
				valWithRewards := validators[0]
				validatorsCount := len(s.network.GetValidators())
				s.Require().Equal(validatorsCount, len(out.Rewards))

				// the response order may change
				for index, or := range out.Rewards {
					if or.ValidatorAddress == valWithRewards.OperatorAddress {
						i = index
					} else {
						s.Require().Equal(0, len(out.Rewards[index].Reward))
					}
				}

				// only validator[i] has rewards
				s.Require().Equal(1, len(out.Rewards[i].Reward))
				s.Require().Equal(s.bondDenom, out.Rewards[i].Reward[0].Denom)
				s.Require().Equal(uint8(math.LegacyPrecision), out.Rewards[i].Reward[0].Precision)
				s.Require().Equal(expRewardsAmt.Int64(), out.Rewards[i].Reward[0].Amount.Int64())

				s.Require().Equal(1, len(out.Total))
				s.Require().Equal(expRewardsAmt.Int64(), out.Total[0].Amount.Int64())
			},
			100000,
			false,
			"",
		},
	}
	testCases = append(testCases, baseTestCases[0])

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()

			args, err := testutil.CallFunction(distribution.NewDelegationTotalRewardsCall, tc.malleate())
			s.Require().NoError(err)
			out, err := s.precompile.DelegationTotalRewards(ctx, *args.(*distribution.DelegationTotalRewardsCall))

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotEmpty(out)
				bz, err := out.Encode()
				s.Require().NoError(err)
				tc.postCheck(bz)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestDelegatorValidators() {
	var ctx sdk.Context
	testCases := []distrTestCases{
		{
			"fail - invalid delegator address",
			func() []interface{} {
				return []interface{}{
					"invalid",
				}
			},
			func([]byte) {},
			100000,
			true,
			fmt.Sprintf(cmn.ErrInvalidDelegator, "invalid"),
		},
		{
			"success - no delegations",
			func() []interface{} {
				newAddr, _ := testutiltx.NewAddrKey()
				return []interface{}{
					newAddr,
				}
			},
			func(bz []byte) {
				var out distribution.DelegatorValidatorsReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)
				s.Require().Equal(0, len(out.Validators))
			},
			100000,
			false,
			"",
		},
		{
			"success - existent delegations",
			func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(0),
				}
			},
			func(bz []byte) {
				var out distribution.DelegatorValidatorsReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)
				s.Require().Equal(3, len(out.Validators))
				for _, val := range s.network.GetValidators() {
					s.Require().Contains(
						out,
						val.OperatorAddress,
						"expected operator address %q to be in output",
						val.OperatorAddress,
					)
				}
			},
			100000,
			false,
			"",
		},
	}
	testCases = append(testCases, baseTestCases[0])

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()
			args, err := testutil.CallFunction(distribution.NewDelegatorValidatorsCall, tc.malleate())
			s.Require().NoError(err)
			out, err := s.precompile.DelegatorValidators(ctx, *args.(*distribution.DelegatorValidatorsCall))

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotEmpty(out)
				bz, err := out.Encode()
				s.Require().NoError(err)
				tc.postCheck(bz)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestDelegatorWithdrawAddress() {
	var ctx sdk.Context
	testCases := []distrTestCases{
		{
			"fail - invalid delegator address",
			func() []interface{} {
				return []interface{}{
					"invalid",
				}
			},
			func([]byte) {},
			100000,
			true,
			fmt.Sprintf(cmn.ErrInvalidDelegator, "invalid"),
		},
		{
			"success - withdraw address same as delegator address",
			func() []interface{} {
				return []interface{}{
					s.keyring.GetAddr(0),
				}
			},
			func(bz []byte) {
				var out distribution.DelegatorWithdrawAddressReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)
				s.Require().Equal(sdk.AccAddress(s.keyring.GetAddr(0).Bytes()).String(), out)
			},
			100000,
			false,
			"",
		},
	}
	testCases = append(testCases, baseTestCases[0])

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()
			args, err := testutil.CallFunction(distribution.NewDelegatorWithdrawAddressCall, tc.malleate())
			s.Require().NoError(err)
			out, err := s.precompile.DelegatorWithdrawAddress(ctx, *args.(*distribution.DelegatorWithdrawAddressCall))

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotEmpty(out)
				bz, err := out.Encode()
				s.Require().NoError(err)
				tc.postCheck(bz)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestCommunityPool() {
	var ctx sdk.Context
	testCases := []distrTestCases{
		{
			"success - empty community pool",
			func() []interface{} {
				return []interface{}{}
			},
			func(bz []byte) {
				var out distribution.CommunityPoolReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)
				s.Require().Equal(0, len(out.Coins))
			},
			100000,
			false,
			"",
		},
		{
			"success - with community pool",
			func() []interface{} {
				amt := math.NewInt(expValAmount)
				err := s.network.App.GetDistrKeeper().FundCommunityPool(ctx, sdk.NewCoins(sdk.NewCoin(s.bondDenom, amt)), s.keyring.GetAccAddr(0))
				s.Require().NoError(err)

				return []interface{}{}
			},
			func(bz []byte) {
				var out distribution.CommunityPoolReturn
				_, err := out.Decode(bz)
				s.Require().NoError(err, "failed to unpack output", err)
				s.Require().Equal(1, len(out.Coins))
				s.Require().Equal(uint8(18), out.Coins[0].Precision)
				s.Require().Equal(s.bondDenom, out.Coins[0].Denom)
				s.Require().Equal(expValAmount, out.Coins[0].Amount.Int64())
			},
			100000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()
			out, err := s.precompile.CommunityPool(ctx, abi.EmptyTuple{})

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotEmpty(out)
				bz, err := out.Encode()
				s.Require().NoError(err)
				tc.postCheck(bz)
			}
		})
	}
}
