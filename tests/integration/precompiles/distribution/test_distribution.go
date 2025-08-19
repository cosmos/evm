package distribution

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/distribution"
	"github.com/cosmos/evm/precompiles/testutil"
	chainutil "github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/testutil/constants"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/distribution/types"
)

func (s *PrecompileTestSuite) TestIsTransaction() {
	testCases := []struct {
		name   string
		method abi.Method
		isTx   bool
	}{
		{
			distribution.SetWithdrawAddressMethod,
			s.precompile.Methods[distribution.SetWithdrawAddressMethod],
			true,
		},
		{
			distribution.WithdrawDelegatorRewardMethod,
			s.precompile.Methods[distribution.WithdrawDelegatorRewardMethod],
			true,
		},
		{
			distribution.WithdrawValidatorCommissionMethod,
			s.precompile.Methods[distribution.WithdrawValidatorCommissionMethod],
			true,
		},
		{
			distribution.FundCommunityPoolMethod,
			s.precompile.Methods[distribution.FundCommunityPoolMethod],
			true,
		},
		{
			distribution.ValidatorDistributionInfoMethod,
			s.precompile.Methods[distribution.ValidatorDistributionInfoMethod],
			false,
		},
		{
			"invalid",
			abi.Method{},
			false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.Require().Equal(s.precompile.IsTransaction(&tc.method), tc.isTx)
		})
	}
}

// TestRun tests the precompile's Run method.
func (s *PrecompileTestSuite) TestRun() {
	var (
		ctx sdk.Context
		err error
	)
	testcases := []struct {
		name        string
		malleate    func() (common.Address, []byte)
		readOnly    bool
		expPass     bool
		errContains string
	}{
		{
			name: "pass - set withdraw address transaction",
			malleate: func() (common.Address, []byte) {
				valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
				s.Require().NoError(err)
				val, _ := s.network.App.GetStakingKeeper().GetValidator(ctx, valAddr)
				coins := sdk.NewCoins(sdk.NewCoin(constants.ExampleAttoDenom, math.NewInt(1e18)))
				s.Require().NoError(s.network.App.GetDistrKeeper().AllocateTokensToValidator(ctx, val, sdk.NewDecCoinsFromCoins(coins...)))

				input, err := s.precompile.Pack(
					distribution.SetWithdrawAddressMethod,
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(0).String(),
				)
				s.Require().NoError(err, "failed to pack input")
				return s.keyring.GetAddr(0), input
			},
			readOnly: false,
			expPass:  true,
		},
		{
			name: "pass - withdraw validator commissions transaction",
			malleate: func() (common.Address, []byte) {
				hexAddr := common.Bytes2Hex(s.keyring.GetAddr(0).Bytes())
				valAddr, err := sdk.ValAddressFromHex(hexAddr)
				s.Require().NoError(err)
				caller := common.BytesToAddress(valAddr)

				commAmt := math.LegacyNewDecWithPrec(1000000000000000000, 1)
				valCommission := sdk.DecCoins{sdk.NewDecCoinFromDec(constants.ExampleAttoDenom, commAmt)}
				// set outstanding rewards
				s.Require().NoError(s.network.App.GetDistrKeeper().SetValidatorOutstandingRewards(ctx, valAddr, types.ValidatorOutstandingRewards{Rewards: valCommission}))
				// set commission
				s.Require().NoError(s.network.App.GetDistrKeeper().SetValidatorAccumulatedCommission(ctx, valAddr, types.ValidatorAccumulatedCommission{Commission: valCommission}))

				// set distribution module account balance which pays out the rewards
				coins := sdk.NewCoins(sdk.NewCoin(s.bondDenom, commAmt.RoundInt()))
				err = s.mintCoinsForDistrMod(ctx, coins)
				s.Require().NoError(err, "failed to fund distr module account")

				input, err := s.precompile.Pack(
					distribution.WithdrawValidatorCommissionMethod,
					valAddr.String(),
				)
				s.Require().NoError(err, "failed to pack input")
				return caller, input
			},
			readOnly: false,
			expPass:  true,
		},
		{
			name: "pass - withdraw delegator rewards transaction",
			malleate: func() (common.Address, []byte) {
				val := s.network.GetValidators()[0]
				ctx, err = s.prepareStakingRewards(
					ctx,
					stakingRewards{
						Delegator: s.keyring.GetAccAddr(0),
						Validator: val,
						RewardAmt: testRewardsAmt,
					},
				)
				s.Require().NoError(err, "failed to prepare staking rewards")

				input, err := s.precompile.Pack(
					distribution.WithdrawDelegatorRewardMethod,
					s.keyring.GetAddr(0),
					val.OperatorAddress,
				)
				s.Require().NoError(err, "failed to pack input")

				return s.keyring.GetAddr(0), input
			},
			readOnly: false,
			expPass:  true,
		},
		{
			name: "pass - claim rewards transaction",
			malleate: func() (common.Address, []byte) {
				ctx, err = s.prepareStakingRewards(
					ctx,
					stakingRewards{
						Delegator: s.keyring.GetAccAddr(0),
						Validator: s.network.GetValidators()[0],
						RewardAmt: testRewardsAmt,
					},
				)
				s.Require().NoError(err, "failed to prepare staking rewards")

				input, err := s.precompile.Pack(
					distribution.ClaimRewardsMethod,
					s.keyring.GetAddr(0),
					uint32(2),
				)
				s.Require().NoError(err, "failed to pack input")

				return s.keyring.GetAddr(0), input
			},
			readOnly: false,
			expPass:  true,
		},
		{
			name: "pass - fund community pool transaction",
			malleate: func() (common.Address, []byte) {
				input, err := s.precompile.Pack(
					distribution.FundCommunityPoolMethod,
					s.keyring.GetAddr(0),
					[]cmn.Coin{
						{
							Denom:  constants.ExampleAttoDenom,
							Amount: big.NewInt(1e18),
						},
					},
				)
				s.Require().NoError(err, "failed to pack input")

				return s.keyring.GetAddr(0), input
			},
			readOnly: false,
			expPass:  true,
		},
		{
			name: "pass - fund multi coins community pool transaction",
			malleate: func() (common.Address, []byte) {
				input, err := s.precompile.Pack(
					distribution.FundCommunityPoolMethod,
					s.keyring.GetAddr(0),
					[]cmn.Coin{
						{
							Denom:  constants.ExampleAttoDenom,
							Amount: big.NewInt(1e18),
						},
						{
							Denom:  "foo",
							Amount: big.NewInt(1e18),
						},
						{
							Denom:  "bar",
							Amount: big.NewInt(1e18),
						},
					},
				)
				s.Require().NoError(err, "failed to pack input")

				return s.keyring.GetAddr(0), input
			},
			readOnly: false,
			expPass:  true,
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			// setup basic test suite
			s.SetupTest()
			ctx = s.network.GetContext()
			baseFee := s.network.App.GetEVMKeeper().GetBaseFee(ctx)

			// malleate testcase
			caller, input := tc.malleate()

			contract := vm.NewPrecompile(caller, s.precompile.Address(), uint256.NewInt(0), uint64(1e6))
			contract.Input = input

			contractAddr := contract.Address()
			// Build and sign Ethereum transaction
			txArgs := evmtypes.EvmTxArgs{
				ChainID:   evmtypes.GetEthChainConfig().ChainID,
				Nonce:     0,
				To:        &contractAddr,
				Amount:    nil,
				GasLimit:  100000,
				GasPrice:  chainutil.ExampleMinGasPrices,
				GasFeeCap: baseFee,
				GasTipCap: big.NewInt(1),
				Accesses:  &gethtypes.AccessList{},
			}
			msgEthereumTx, err := s.factory.GenerateMsgEthereumTx(s.keyring.GetPrivKey(0), txArgs)
			s.Require().NoError(err, "failed to generate Ethereum message")

			signedMsg, err := s.factory.SignMsgEthereumTx(s.keyring.GetPrivKey(0), msgEthereumTx)
			s.Require().NoError(err, "failed to sign Ethereum message")

			// Instantiate config
			proposerAddress := ctx.BlockHeader().ProposerAddress
			cfg, err := s.network.App.GetEVMKeeper().EVMConfig(ctx, proposerAddress)
			s.Require().NoError(err, "failed to instantiate EVM config")

			msg := signedMsg.AsMessage(baseFee)

			// Instantiate EVM
			evm := s.network.App.GetEVMKeeper().NewEVM(
				ctx, *msg, cfg, nil, s.network.GetStateDB(),
			)

			precompiles, found, err := s.network.App.GetEVMKeeper().GetPrecompileInstance(ctx, contractAddr)
			s.Require().NoError(err, "failed to instantiate precompile")
			s.Require().True(found, "not found precompile")
			evm.WithPrecompiles(precompiles.Map)
			// Run precompiled contract
			bz, err := s.precompile.Run(evm, contract, tc.readOnly)

			// Check results
			if tc.expPass {
				s.Require().NoError(err, "expected no error when running the precompile")
				s.Require().NotNil(bz, "expected returned bytes not to be nil")
			} else {
				s.Require().Error(err, "expected error to be returned when running the precompile")
				s.Require().Nil(bz, "expected returned bytes to be nil")
				s.Require().ErrorContains(err, tc.errContains)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestCMS() {
	var (
		ctx sdk.Context
		err error
	)
	testcases := []struct {
		name        string
		malleate    func() (common.Address, []byte)
		expPass     bool
		errContains string
	}{
		{
			name: "pass - set withdraw address transaction",
			malleate: func() (common.Address, []byte) {
				valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
				s.Require().NoError(err)
				val, _ := s.network.App.GetStakingKeeper().GetValidator(ctx, valAddr)
				coins := sdk.NewCoins(sdk.NewCoin(constants.ExampleAttoDenom, math.NewInt(1e18)))
				s.Require().NoError(s.network.App.GetDistrKeeper().AllocateTokensToValidator(ctx, val, sdk.NewDecCoinsFromCoins(coins...)))

				input, err := s.precompile.Pack(
					distribution.SetWithdrawAddressMethod,
					s.keyring.GetAddr(0),
					s.keyring.GetAddr(0).String(),
				)
				s.Require().NoError(err, "failed to pack input")
				return s.keyring.GetAddr(0), input
			},
			expPass: true,
		},
		{
			name: "pass - withdraw validator commissions transaction",
			malleate: func() (common.Address, []byte) {
				hexAddr := common.Bytes2Hex(s.keyring.GetAddr(0).Bytes())
				valAddr, err := sdk.ValAddressFromHex(hexAddr)
				s.Require().NoError(err)
				caller := common.BytesToAddress(valAddr)

				commAmt := math.LegacyNewDecWithPrec(1000000000000000000, 1)
				valCommission := sdk.DecCoins{sdk.NewDecCoinFromDec(constants.ExampleAttoDenom, commAmt)}
				// set outstanding rewards
				s.Require().NoError(s.network.App.GetDistrKeeper().SetValidatorOutstandingRewards(ctx, valAddr, types.ValidatorOutstandingRewards{Rewards: valCommission}))
				// set commission
				s.Require().NoError(s.network.App.GetDistrKeeper().SetValidatorAccumulatedCommission(ctx, valAddr, types.ValidatorAccumulatedCommission{Commission: valCommission}))

				// set distribution module account balance which pays out the rewards
				coins := sdk.NewCoins(sdk.NewCoin(s.bondDenom, commAmt.RoundInt()))
				err = s.mintCoinsForDistrMod(ctx, coins)
				s.Require().NoError(err, "failed to fund distr module account")

				input, err := s.precompile.Pack(
					distribution.WithdrawValidatorCommissionMethod,
					valAddr.String(),
				)
				s.Require().NoError(err, "failed to pack input")
				return caller, input
			},
			expPass: true,
		},
		{
			name: "pass - withdraw delegator rewards transaction",
			malleate: func() (common.Address, []byte) {
				val := s.network.GetValidators()[0]
				ctx, err = s.prepareStakingRewards(
					ctx,
					stakingRewards{
						Delegator: s.keyring.GetAccAddr(0),
						Validator: val,
						RewardAmt: testRewardsAmt,
					},
				)
				s.Require().NoError(err, "failed to prepare staking rewards")

				input, err := s.precompile.Pack(
					distribution.WithdrawDelegatorRewardMethod,
					s.keyring.GetAddr(0),
					val.OperatorAddress,
				)
				s.Require().NoError(err, "failed to pack input")

				return s.keyring.GetAddr(0), input
			},
			expPass: true,
		},
		{
			name: "pass - claim rewards transaction",
			malleate: func() (common.Address, []byte) {
				ctx, err = s.prepareStakingRewards(
					ctx,
					stakingRewards{
						Delegator: s.keyring.GetAccAddr(0),
						Validator: s.network.GetValidators()[0],
						RewardAmt: testRewardsAmt,
					},
				)
				s.Require().NoError(err, "failed to prepare staking rewards")

				input, err := s.precompile.Pack(
					distribution.ClaimRewardsMethod,
					s.keyring.GetAddr(0),
					uint32(2),
				)
				s.Require().NoError(err, "failed to pack input")

				return s.keyring.GetAddr(0), input
			},
			expPass: true,
		},
		{
			name: "pass - fund community pool transaction",
			malleate: func() (common.Address, []byte) {
				input, err := s.precompile.Pack(
					distribution.FundCommunityPoolMethod,
					s.keyring.GetAddr(0),
					[]cmn.Coin{
						{
							Denom:  constants.ExampleAttoDenom,
							Amount: big.NewInt(1e18),
						},
					},
				)
				s.Require().NoError(err, "failed to pack input")

				return s.keyring.GetAddr(0), input
			},
			expPass: true,
		},
		{
			name: "pass - fund multi coins community pool transaction",
			malleate: func() (common.Address, []byte) {
				input, err := s.precompile.Pack(
					distribution.FundCommunityPoolMethod,
					s.keyring.GetAddr(0),
					[]cmn.Coin{
						{
							Denom:  constants.ExampleAttoDenom,
							Amount: big.NewInt(1e18),
						},
						{
							Denom:  "foo",
							Amount: big.NewInt(1e18),
						},
						{
							Denom:  "bar",
							Amount: big.NewInt(1e18),
						},
					},
				)
				s.Require().NoError(err, "failed to pack input")

				return s.keyring.GetAddr(0), input
			},
			expPass: true,
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			// setup basic test suite
			s.SetupTest()
			ctx = s.network.GetContext()
			cms := &testutil.TrackingMultiStore{
				Store:            s.network.App.GetBaseApp().CommitMultiStore().CacheMultiStore(),
				Writes:           0,
				HistoricalStores: nil,
			}
			ctx = ctx.WithMultiStore(cms)
			baseFee := s.network.App.GetEVMKeeper().GetBaseFee(ctx)

			// malleate testcase
			caller, input := tc.malleate()
			contract := vm.NewPrecompile(caller, s.precompile.Address(), uint256.NewInt(0), uint64(1e6))

			contractAddr := contract.Address()
			// Build and sign Ethereum transaction
			txArgs := evmtypes.EvmTxArgs{
				Input:     input,
				ChainID:   evmtypes.GetEthChainConfig().ChainID,
				Nonce:     0,
				To:        &contractAddr,
				Amount:    nil,
				GasLimit:  1000000,
				GasPrice:  chainutil.ExampleMinGasPrices,
				GasFeeCap: baseFee,
				GasTipCap: big.NewInt(1),
				Accesses:  &gethtypes.AccessList{},
			}
			msgEthereumTx, err := s.factory.GenerateMsgEthereumTx(s.keyring.GetPrivKey(0), txArgs)
			s.Require().NoError(err, "failed to generate Ethereum message")

			signedMsg, err := s.factory.SignMsgEthereumTx(s.keyring.GetPrivKey(0), msgEthereumTx)
			s.Require().NoError(err, "failed to sign Ethereum message")

			resp, err := s.network.App.GetEVMKeeper().EthereumTx(ctx, &signedMsg)

			// Check results
			if tc.expPass {
				s.Require().NoError(err, "expected no error when running the precompile")
				s.Require().NotNil(resp.Ret, "expected returned bytes not to be nil")
				// NOTES: After stack-based snapshot mechanism is added for precompile call,
				// CacheMultiStore.Write() is always called once when tx succeeds.
				// It is because CacheMultiStore() is not called when creating snapshot for MultiStore,
				// Count of Write() is not accumulated.
				testutil.ValidateWrites(s.T(), cms, 1)
			} else {
				s.Require().Error(err, "expected error to be returned when running the precompile")
				s.Require().Nil(resp.Ret, "expected returned bytes to be nil")
				s.Require().ErrorContains(err, tc.errContains)
				// NOTES: After stack-based snapshot mechanism is added for precompile call,
				// CacheMultiStore.Write() is not called when tx fails.
				testutil.ValidateWrites(s.T(), cms, 0)
			}
		})
	}
}
