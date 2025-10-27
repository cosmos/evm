package distribution

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

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
	method := s.precompile.Methods[distribution.SetWithdrawAddressMethod]
	testCases := []struct {
		name        string
		malleate    func(operatorAddress string) []interface{}
		postCheck   func()
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"success - the correct event is emitted",
			func(string) []interface{} {
				return []interface{}{
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(0).String(),
				}
			},
			func() {
				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				// Check event signature matches the one emitted
				event := s.precompile.Events[distribution.EventTypeSetWithdrawAddress]
				s.Require().Equal(crypto.Keccak256Hash([]byte(event.Sig)), common.HexToHash(log.Topics[0].Hex()))
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				// Check the fully unpacked event matches the one emitted
				var setWithdrawerAddrEvent distribution.EventSetWithdrawAddress
				err := cmn.UnpackLog(s.precompile.ABI, &setWithdrawerAddrEvent, distribution.EventTypeSetWithdrawAddress, *log)
				s.Require().NoError(err)
				s.Require().Equal(s.keyring.GetAddr(0), setWithdrawerAddrEvent.Caller)
				s.Require().Equal(sdk.MustBech32ifyAddressBytes("cosmos", s.keyring.GetAddr(0).Bytes()), setWithdrawerAddrEvent.WithdrawerAddress)
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

		_, err := s.precompile.SetWithdrawAddress(ctx, contract, stDB, &method, tc.malleate(s.network.GetValidators()[0].OperatorAddress))

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
	method := s.precompile.Methods[distribution.WithdrawDelegatorRewardMethod]
	testCases := []struct {
		name        string
		malleate    func(val stakingtypes.Validator) []interface{}
		postCheck   func()
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"success - the correct event is emitted",
			func(val stakingtypes.Validator) []interface{} {
				var err error

				ctx, err = s.prepareStakingRewards(ctx, stakingRewards{
					Validator: val,
					Delegator: s.keyring.GetAccAddr(0),
					RewardAmt: testRewardsAmt,
				})
				s.Require().NoError(err)
				return []interface{}{
					s.keyring.GetAddr(0),
					val.OperatorAddress,
				}
			},
			func() {
				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				// Check event signature matches the one emitted
				event := s.precompile.Events[distribution.EventTypeWithdrawDelegatorReward]
				s.Require().Equal(crypto.Keccak256Hash([]byte(event.Sig)), common.HexToHash(log.Topics[0].Hex()))
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				optAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
				s.Require().NoError(err)
				optHexAddr := common.BytesToAddress(optAddr)

				// Check the fully unpacked event matches the one emitted
				var delegatorRewards distribution.EventWithdrawDelegatorReward
				err = cmn.UnpackLog(s.precompile.ABI, &delegatorRewards, distribution.EventTypeWithdrawDelegatorReward, *log)
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

		_, err := s.precompile.WithdrawDelegatorReward(ctx, contract, stDB, &method, tc.malleate(s.network.GetValidators()[0]))

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
	method := s.precompile.Methods[distribution.WithdrawValidatorCommissionMethod]
	testCases := []struct {
		name        string
		malleate    func(operatorAddress string) []interface{}
		postCheck   func()
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"success - the correct event is emitted",
			func(operatorAddress string) []interface{} {
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
				return []interface{}{
					operatorAddress,
				}
			},
			func() {
				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				// Check event signature matches the one emitted
				event := s.precompile.Events[distribution.EventTypeWithdrawValidatorCommission]
				s.Require().Equal(crypto.Keccak256Hash([]byte(event.Sig)), common.HexToHash(log.Topics[0].Hex()))
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				// Check the fully unpacked event matches the one emitted
				var validatorRewards distribution.EventWithdrawValidatorRewards
				err := cmn.UnpackLog(s.precompile.ABI, &validatorRewards, distribution.EventTypeWithdrawValidatorCommission, *log)
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

		_, err = s.precompile.WithdrawValidatorCommission(ctx, contract, stDB, &method, tc.malleate(s.network.GetValidators()[0].OperatorAddress))

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
				event := s.precompile.Events[distribution.EventTypeClaimRewards]
				s.Require().Equal(event.ID, common.HexToHash(log.Topics[0].Hex()))
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				var claimRewardsEvent distribution.EventClaimRewards
				err := cmn.UnpackLog(s.precompile.ABI, &claimRewardsEvent, distribution.EventTypeClaimRewards, *log)
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
				event := s.precompile.Events[distribution.EventTypeFundCommunityPool]
				s.Require().Equal(event.ID, common.HexToHash(log.Topics[0].Hex()))
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				var fundCommunityPoolEvent distribution.EventFundCommunityPool
				err := cmn.UnpackLog(s.precompile.ABI, &fundCommunityPoolEvent, distribution.EventTypeFundCommunityPool, *log)
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
					event := s.precompile.Events[distribution.EventTypeFundCommunityPool]
					s.Require().Equal(event.ID, common.HexToHash(log.Topics[0].Hex()))
					s.Require().Equal(uint64(ctx.BlockHeight()), log.BlockNumber) //nolint:gosec // G115

					var fundCommunityPoolEvent distribution.EventFundCommunityPool
					err := cmn.UnpackLog(s.precompile.ABI, &fundCommunityPoolEvent, distribution.EventTypeFundCommunityPool, *log)
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
	method := s.precompile.Methods[distribution.DepositValidatorRewardsPoolMethod]
	testCases := []struct {
		name        string
		malleate    func(operatorAddress string) ([]interface{}, sdk.Coins)
		postCheck   func(sdk.Coins)
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"success - the correct event is emitted",
			func(operatorAddress string) ([]interface{}, sdk.Coins) {
				coins := []cmn.Coin{
					{
						Denom:  constants.ExampleAttoDenom,
						Amount: big.NewInt(1e18),
					},
				}
				sdkCoins, err := cmn.NewSdkCoinsFromCoins(coins)
				s.Require().NoError(err)

				return []interface{}{
					s.keyring.GetAddr(0),
					operatorAddress,
					coins,
				}, sdkCoins.Sort()
			},
			func(sdkCoins sdk.Coins) {
				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
				s.Require().NoError(err)

				// Check event signature matches the one emitted
				event := s.precompile.Events[distribution.EventTypeDepositValidatorRewardsPool]
				s.Require().Equal(crypto.Keccak256Hash([]byte(event.Sig)), common.HexToHash(log.Topics[0].Hex()))
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				// Check the fully unpacked event matches the one emitted
				var depositValidatorRewardsPool distribution.EventDepositValidatorRewardsPool
				err = cmn.UnpackLog(s.precompile.ABI, &depositValidatorRewardsPool, distribution.EventTypeDepositValidatorRewardsPool, *log)
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
			func(operatorAddress string) ([]interface{}, sdk.Coins) {
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

				return []interface{}{
					s.keyring.GetAddr(0),
					operatorAddress,
					coins,
				}, sdkCoins.Sort()
			},
			func(sdkCoins sdk.Coins) {
				for i, log := range stDB.Logs() {
					s.Require().Equal(log.Address, s.precompile.Address())

					valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
					s.Require().NoError(err)

					// Check event signature matches the one emitted
					event := s.precompile.Events[distribution.EventTypeDepositValidatorRewardsPool]
					s.Require().Equal(crypto.Keccak256Hash([]byte(event.Sig)), common.HexToHash(log.Topics[0].Hex()))
					s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

					// Check the fully unpacked event matches the one emitted
					var depositValidatorRewardsPool distribution.EventDepositValidatorRewardsPool
					err = cmn.UnpackLog(s.precompile.ABI, &depositValidatorRewardsPool, distribution.EventTypeDepositValidatorRewardsPool, *log)
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
		_, err := s.precompile.DepositValidatorRewardsPool(ctx, contract, stDB, &method, args)

		if tc.expError {
			s.Require().Error(err)
			s.Require().Contains(err.Error(), tc.errContains)
		} else {
			s.Require().NoError(err)
			tc.postCheck(sdkCoins)
		}
	}
}
