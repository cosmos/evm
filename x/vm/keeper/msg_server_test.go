package keeper_test

import (
	"math/big"

	ethparams "github.com/ethereum/go-ethereum/params"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/evm/testutil/integration/os/utils"
	"github.com/cosmos/evm/x/vm/types"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

func (suite *KeeperTestSuite) TestEthereumTx() {
	suite.enableFeemarket = true
	suite.mintFeeCollector = true
	defer func() {
		suite.enableFeemarket = false
		suite.mintFeeCollector = false
	}()
	suite.SetupTest()
	testCases := []struct {
		name        string
		getMsg      func() *types.MsgEthereumTx
		expectedErr error
	}{
		{
			"fail - insufficient gas",
			func() *types.MsgEthereumTx {
				args := types.EvmTxArgs{
					// Have insufficient gas
					GasLimit: 10,
				}
				tx, err := suite.factory.GenerateSignedEthTx(suite.keyring.GetPrivKey(0), args)
				suite.Require().NoError(err)
				return tx.GetMsgs()[0].(*types.MsgEthereumTx)
			},
			types.ErrInvalidGasCap,
		},
		{
			"success - transfer funds tx",
			func() *types.MsgEthereumTx {
				recipient := suite.keyring.GetAddr(1)
				args := types.EvmTxArgs{
					To:     &recipient,
					Amount: big.NewInt(1e18),
				}
				tx, err := suite.factory.GenerateSignedEthTx(suite.keyring.GetPrivKey(0), args)
				suite.Require().NoError(err)
				return tx.GetMsgs()[0].(*types.MsgEthereumTx)
			},
			nil,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			msg := tc.getMsg()

			// Ensure fee collector has sufficient balance for each subtest
			if suite.mintFeeCollector {
				feeCollectorAddr := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
				denom := types.GetEVMCoinExtendedDenom()
				currentBalance := suite.network.App.BankKeeper.GetBalance(suite.network.GetContext(), feeCollectorAddr, denom)

				baseFee := suite.network.App.EVMKeeper.GetBaseFee(suite.network.GetContext())
				if baseFee == nil {
					baseFee = big.NewInt(0)
				}

				gasLimit := new(big.Int).SetUint64(msg.GetGas())
				requiredBalance := sdkmath.NewIntFromBigInt(new(big.Int).Mul(gasLimit, baseFee)).
					Add(sdkmath.NewIntFromUint64(ethparams.TxGas - 1))

				if currentBalance.Amount.LT(requiredBalance) {
					coinsToAdd := sdktypes.NewCoins(sdktypes.NewCoin(denom, requiredBalance.Sub(currentBalance.Amount)))
					err := suite.network.App.BankKeeper.MintCoins(suite.network.GetContext(), types.ModuleName, coinsToAdd)
					suite.Require().NoError(err)
					err = suite.network.App.BankKeeper.SendCoinsFromModuleToModule(suite.network.GetContext(), types.ModuleName, authtypes.FeeCollectorName, coinsToAdd)
					suite.Require().NoError(err)
				}
			}

			// Function to be tested
			res, err := suite.network.App.EVMKeeper.EthereumTx(suite.network.GetContext(), msg)

			events := suite.network.GetContext().EventManager().Events()
			if tc.expectedErr != nil {
				suite.Require().Error(err)
				// no events should have been emitted
				suite.Require().Empty(events)
			} else {
				suite.Require().NoError(err)
				suite.Require().False(res.Failed())

				// check expected events were emitted
				suite.Require().NotEmpty(events)
				suite.Require().True(utils.ContainsEventType(events.ToABCIEvents(), types.EventTypeEthereumTx))
				suite.Require().True(utils.ContainsEventType(events.ToABCIEvents(), types.EventTypeTxLog))
				suite.Require().True(utils.ContainsEventType(events.ToABCIEvents(), sdktypes.EventTypeMessage))
			}

			err = suite.network.NextBlock()
			suite.Require().NoError(err)
		})
	}
}

func (suite *KeeperTestSuite) TestUpdateParams() {
	suite.SetupTest()
	testCases := []struct {
		name        string
		getMsg      func() *types.MsgUpdateParams
		expectedErr error
	}{
		{
			name: "fail - invalid authority",
			getMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{Authority: "foobar"}
			},
			expectedErr: govtypes.ErrInvalidSigner,
		},
		{
			name: "pass - valid Update msg",
			getMsg: func() *types.MsgUpdateParams {
				return &types.MsgUpdateParams{
					Authority: authtypes.NewModuleAddress(govtypes.ModuleName).String(),
					Params:    types.DefaultParams(),
				}
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		suite.Run("MsgUpdateParams", func() {
			msg := tc.getMsg()
			_, err := suite.network.App.EVMKeeper.UpdateParams(suite.network.GetContext(), msg)
			if tc.expectedErr != nil {
				suite.Require().Error(err)
				suite.Contains(err.Error(), tc.expectedErr.Error())
			} else {
				suite.Require().NoError(err)
			}
		})

		err := suite.network.NextBlock()
		suite.Require().NoError(err)
	}
}
