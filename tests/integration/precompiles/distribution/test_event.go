package distribution

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/yihuang/go-abi"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/distribution"
	"github.com/cosmos/evm/precompiles/testutil"
	"github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/x/vm/statedb"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (s *PrecompileTestSuite) TestSetWithdrawAddressEvent() {
	var (
		ctx  sdk.Context
		stDB *statedb.StateDB
	)
	testCases := []struct {
		name        string
		malleate    func(operatorAddress string) *distribution.SetWithdrawAddressCall
		postCheck   func()
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"success - the correct event is emitted",
			func(string) *distribution.SetWithdrawAddressCall {
				return distribution.NewSetWithdrawAddressCall(s.keyring.GetAddr(0),
					s.keyring.GetAddr(0).String(),
				)
			},
			func() {
				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				// Check event signature matches the one emitted
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				// Check the fully unpacked event matches the one emitted
				var setWithdrawerAddrEvent distribution.SetWithdrawerAddressEvent
				err := abi.DecodeEvent(&setWithdrawerAddrEvent, log.Topics, log.Data)
				s.Require().NoError(err)
				s.Require().Equal(s.keyring.GetAddr(0), setWithdrawerAddrEvent.Caller)
				bech32AddrPrefix := sdk.GetConfig().GetBech32AccountAddrPrefix()
				s.Require().Equal(sdk.MustBech32ifyAddressBytes(bech32AddrPrefix, s.keyring.GetAddr(0).Bytes()), setWithdrawerAddrEvent.WithdrawerAddress)
			},
			20000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.SetupTest()
		ctx = s.network.GetContext()
		stDB = s.network.GetStateDB()

		contract := vm.NewContract(s.keyring.GetAddr(0), s.precompile.Address(), uint256.NewInt(0), tc.gas, nil)
		ctx = ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
		initialGas := ctx.GasMeter().GasConsumed()
		s.Require().Zero(initialGas)

		_, err := s.precompile.SetWithdrawAddress(ctx, *tc.malleate(s.network.GetValidators()[0].OperatorAddress), stDB, contract)

		if tc.expError {
			s.Require().Error(err)
			s.Require().Contains(err.Error(), tc.errContains)
		} else {
			s.Require().NoError(err)
			tc.postCheck()
		}
	}
}

func (s *PrecompileTestSuite) TestWithdrawDelegatorRewardEvent() {
	var (
		ctx  sdk.Context
		stDB *statedb.StateDB
	)
	testCases := []struct {
		name        string
		malleate    func(val stakingtypes.Validator) *distribution.WithdrawDelegatorRewardsCall
		postCheck   func()
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"success - the correct event is emitted",
			func(val stakingtypes.Validator) *distribution.WithdrawDelegatorRewardsCall {
				var err error

				ctx, err = s.prepareStakingRewards(ctx, stakingRewards{
					Validator: val,
					Delegator: s.keyring.GetAccAddr(0),
					RewardAmt: testRewardsAmt,
				})
				s.Require().NoError(err)
				return distribution.NewWithdrawDelegatorRewardsCall(s.keyring.GetAddr(0),
					val.OperatorAddress,
				)
			},
			func() {
				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				// Check event signature matches the one emitted
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				optAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
				s.Require().NoError(err)
				optHexAddr := common.BytesToAddress(optAddr)

				// Check the fully unpacked event matches the one emitted
				var delegatorRewards distribution.WithdrawDelegatorRewardEvent
				err = abi.DecodeEvent(&delegatorRewards, log.Topics, log.Data)
				s.Require().NoError(err)
				s.Require().Equal(s.keyring.GetAddr(0), delegatorRewards.DelegatorAddress)
				s.Require().Equal(optHexAddr, delegatorRewards.ValidatorAddress)
				s.Require().Equal(expRewardsAmt.BigInt(), delegatorRewards.Amount)
			},
			20000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.SetupTest()
		ctx = s.network.GetContext()
		stDB = s.network.GetStateDB()

		contract := vm.NewContract(s.keyring.GetAddr(0), s.precompile.Address(), uint256.NewInt(0), tc.gas, nil)
		ctx = ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
		initialGas := ctx.GasMeter().GasConsumed()
		s.Require().Zero(initialGas)

		_, err := s.precompile.WithdrawDelegatorReward(ctx, *tc.malleate(s.network.GetValidators()[0]), stDB, contract)

		if tc.expError {
			s.Require().Error(err)
			s.Require().Contains(err.Error(), tc.errContains)
		} else {
			s.Require().NoError(err)
			tc.postCheck()
		}
	}
}

func (s *PrecompileTestSuite) TestWithdrawValidatorCommissionEvent() {
	var (
		ctx  sdk.Context
		stDB *statedb.StateDB
		amt  = math.NewInt(1e18)
	)
	testCases := []struct {
		name        string
		malleate    func(operatorAddress string) *distribution.WithdrawValidatorCommissionCall
		postCheck   func()
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"success - the correct event is emitted",
			func(operatorAddress string) *distribution.WithdrawValidatorCommissionCall {
				valAddr, err := sdk.ValAddressFromBech32(operatorAddress)
				s.Require().NoError(err)
				valCommission := sdk.DecCoins{sdk.NewDecCoinFromDec(constants.ExampleAttoDenom, math.LegacyNewDecFromInt(amt))}
				// set outstanding rewards
				s.Require().NoError(s.network.App.GetDistrKeeper().SetValidatorOutstandingRewards(ctx, valAddr, types.ValidatorOutstandingRewards{Rewards: valCommission}))
				// set commission
				s.Require().NoError(s.network.App.GetDistrKeeper().SetValidatorAccumulatedCommission(ctx, valAddr, types.ValidatorAccumulatedCommission{Commission: valCommission}))
				// set funds to distr mod to pay for commission
				coins := sdk.NewCoins(sdk.NewCoin(constants.ExampleAttoDenom, amt))
				err = s.mintCoinsForDistrMod(ctx, coins)
				s.Require().NoError(err)
				return distribution.NewWithdrawValidatorCommissionCall(operatorAddress)
			},
			func() {
				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				// Check event signature matches the one emitted
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				// Check the fully unpacked event matches the one emitted
				var validatorRewards distribution.WithdrawValidatorCommissionEvent
				err := abi.DecodeEvent(&validatorRewards, log.Topics, log.Data)
				s.Require().NoError(err)
				s.Require().Equal(crypto.Keccak256Hash([]byte(s.network.GetValidators()[0].OperatorAddress)), validatorRewards.ValidatorAddress)
				s.Require().Equal(amt.BigInt(), validatorRewards.Commission)
			},
			20000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.SetupTest()
		ctx = s.network.GetContext()
		stDB = s.network.GetStateDB()

		valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
		s.Require().NoError(err)
		validatorAddress := common.BytesToAddress(valAddr)
		contract := vm.NewContract(validatorAddress, s.precompile.Address(), uint256.NewInt(0), tc.gas, nil)
		ctx = ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
		initialGas := ctx.GasMeter().GasConsumed()
		s.Require().Zero(initialGas)

		_, err = s.precompile.WithdrawValidatorCommission(ctx, *tc.malleate(s.network.GetValidators()[0].OperatorAddress), stDB, contract)

		if tc.expError {
			s.Require().Error(err)
			s.Require().Contains(err.Error(), tc.errContains)
		} else {
			s.Require().NoError(err)
			tc.postCheck()
		}
	}
}

func (s *PrecompileTestSuite) TestClaimRewardsEvent() {
	var (
		ctx  sdk.Context
		stDB *statedb.StateDB
	)
	testCases := []struct {
		name      string
		coins     sdk.Coins
		postCheck func()
	}{
		{
			"success",
			sdk.NewCoins(sdk.NewCoin(constants.ExampleAttoDenom, math.NewInt(1e18))),
			func() {
				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())
				// Check event signature matches the one emitted
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				var claimRewardsEvent distribution.ClaimRewardsEvent
				err := abi.DecodeEvent(&claimRewardsEvent, log.Topics, log.Data)
				s.Require().NoError(err)
				s.Require().Equal(common.BytesToAddress(s.keyring.GetAddr(0).Bytes()), claimRewardsEvent.DelegatorAddress)
				s.Require().Equal(big.NewInt(1e18), claimRewardsEvent.Amount)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			stDB = s.network.GetStateDB()
			err := s.precompile.EmitClaimRewardsEvent(ctx, stDB, s.keyring.GetAddr(0), tc.coins)
			s.Require().NoError(err)
			tc.postCheck()
		})
	}
}

func (s *PrecompileTestSuite) TestFundCommunityPoolEvent() {
	var (
		ctx  sdk.Context
		stDB *statedb.StateDB
	)
	testCases := []struct {
		name      string
		coins     sdk.Coins
		postCheck func(sdk.Coins)
	}{
		{
			"success - the correct event is emitted",
			sdk.NewCoins(sdk.NewCoin(constants.ExampleAttoDenom, math.NewInt(1e18))),
			func(coins sdk.Coins) {
				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())
				// Check event signature matches the one emitted
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				var fundCommunityPoolEvent distribution.FundCommunityPoolEvent
				err := abi.DecodeEvent(&fundCommunityPoolEvent, log.Topics, log.Data)
				s.Require().NoError(err)
				s.Require().Equal(s.keyring.GetAddr(0), fundCommunityPoolEvent.Depositor)
				s.Require().Equal(constants.ExampleAttoDenom, fundCommunityPoolEvent.Denom)
				s.Require().Equal(big.NewInt(1e18), fundCommunityPoolEvent.Amount)
			},
		},
		{
			// New multi-coin deposit test case
			name: "success - multiple coins => multiple events emitted",
			coins: sdk.NewCoins(
				sdk.NewCoin(constants.ExampleAttoDenom, math.NewInt(10)),   // coin #1
				sdk.NewCoin(constants.OtherCoinDenoms[0], math.NewInt(20)), // coin #2
				sdk.NewCoin(constants.OtherCoinDenoms[1], math.NewInt(30)), // coin #3
			).Sort(),
			postCheck: func(coins sdk.Coins) {
				logs := stDB.Logs()
				s.Require().Len(logs, 3, "expected exactly one event log *per coin*")

				// For convenience, map the sdk.Coins to their big.Int amounts for checking
				expected := []struct {
					amount *big.Int
					// denom  string // If your event includes a Denom field
				}{
					{amount: big.NewInt(10)},
					{amount: big.NewInt(30)},
					{amount: big.NewInt(20)}, // sorted by denom
				}

				for i, log := range logs {
					s.Require().Equal(log.Address, s.precompile.Address(), "log address must match the precompile address")

					// Check event signature
					s.Require().Equal(uint64(ctx.BlockHeight()), log.BlockNumber) //nolint:gosec // G115

					var fundCommunityPoolEvent distribution.FundCommunityPoolEvent
					err := abi.DecodeEvent(&fundCommunityPoolEvent, log.Topics, log.Data)
					s.Require().NoError(err)

					s.Require().Equal(s.keyring.GetAddr(0), fundCommunityPoolEvent.Depositor)
					s.Require().Equal(coins.GetDenomByIndex(i), fundCommunityPoolEvent.Denom)
					s.Require().Equal(expected[i].amount, fundCommunityPoolEvent.Amount)
				}
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			stDB = s.network.GetStateDB()

			err := s.precompile.EmitFundCommunityPoolEvent(ctx, stDB, s.keyring.GetAddr(0), tc.coins)
			s.Require().NoError(err)
			tc.postCheck(tc.coins)
		})
	}
}

func (s *PrecompileTestSuite) TestDepositValidatorRewardsPoolEvent() {
	var (
		ctx  sdk.Context
		stDB *statedb.StateDB
		amt  = math.NewInt(1e18)
	)
	testCases := []struct {
		name        string
		malleate    func(operatorAddress string) (*distribution.DepositValidatorRewardsPoolCall, sdk.Coins)
		postCheck   func(sdk.Coins)
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"success - the correct event is emitted",
			func(operatorAddress string) (*distribution.DepositValidatorRewardsPoolCall, sdk.Coins) {
				coins := []cmn.Coin{
					{
						Denom:  constants.ExampleAttoDenom,
						Amount: big.NewInt(1e18),
					},
				}
				sdkCoins, err := cmn.NewSdkCoinsFromCoins(coins)
				s.Require().NoError(err)

				return distribution.NewDepositValidatorRewardsPoolCall(s.keyring.GetAddr(0),
					operatorAddress,
					coins,
				), sdkCoins.Sort()
			},
			func(sdkCoins sdk.Coins) {
				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
				s.Require().NoError(err)

				// Check event signature matches the one emitted
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				// Check the fully unpacked event matches the one emitted
				var depositValidatorRewardsPool distribution.DepositValidatorRewardsPoolEvent
				err = abi.DecodeEvent(&depositValidatorRewardsPool, log.Topics, log.Data)
				s.Require().NoError(err)
				s.Require().Equal(depositValidatorRewardsPool.Depositor, s.keyring.GetAddr(0))
				s.Require().Equal(depositValidatorRewardsPool.ValidatorAddress, common.BytesToAddress(valAddr.Bytes()))
				s.Require().Equal(depositValidatorRewardsPool.Denom, constants.ExampleAttoDenom)
				s.Require().Equal(depositValidatorRewardsPool.Amount, amt.BigInt())
			},
			20000,
			false,
			"",
		},
		{
			"success - the correct event is emitted for multiple coins",
			func(operatorAddress string) (*distribution.DepositValidatorRewardsPoolCall, sdk.Coins) {
				coins := []cmn.Coin{
					{
						Denom:  constants.ExampleAttoDenom,
						Amount: big.NewInt(1e18),
					},
					{
						Denom:  s.otherDenoms[0],
						Amount: big.NewInt(2e18),
					},
					{
						Denom:  s.otherDenoms[1],
						Amount: big.NewInt(3e18),
					},
				}
				sdkCoins, err := cmn.NewSdkCoinsFromCoins(coins)
				s.Require().NoError(err)

				return distribution.NewDepositValidatorRewardsPoolCall(s.keyring.GetAddr(0),
					operatorAddress,
					coins,
				), sdkCoins.Sort()
			},
			func(sdkCoins sdk.Coins) {
				for i, log := range stDB.Logs() {
					s.Require().Equal(log.Address, s.precompile.Address())

					valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
					s.Require().NoError(err)

					// Check event signature matches the one emitted
					s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

					// Check the fully unpacked event matches the one emitted
					var depositValidatorRewardsPool distribution.DepositValidatorRewardsPoolEvent
					err = abi.DecodeEvent(&depositValidatorRewardsPool, log.Topics, log.Data)
					s.Require().NoError(err)
					s.Require().Equal(depositValidatorRewardsPool.Depositor, s.keyring.GetAddr(0))
					s.Require().Equal(depositValidatorRewardsPool.ValidatorAddress, common.BytesToAddress(valAddr.Bytes()))
					s.Require().Equal(depositValidatorRewardsPool.Denom, sdkCoins[i].Denom)
					s.Require().Equal(depositValidatorRewardsPool.Amount, sdkCoins[i].Amount.BigInt())
				}
			},
			20000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.SetupTest()
		ctx = s.network.GetContext()
		stDB = s.network.GetStateDB()

		var contract *vm.Contract
		contract, ctx = testutil.NewPrecompileContract(s.T(), ctx, s.keyring.GetAddr(0), s.precompile.Address(), tc.gas)

		args, sdkCoins := tc.malleate(s.network.GetValidators()[0].OperatorAddress)
		_, err := s.precompile.DepositValidatorRewardsPool(ctx, *args, stDB, contract)

		if tc.expError {
			s.Require().Error(err)
			s.Require().Contains(err.Error(), tc.errContains)
		} else {
			s.Require().NoError(err)
			tc.postCheck(sdkCoins)
		}
	}
}
