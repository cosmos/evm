package evm_test

import (
	"fmt"
	"math/big"

	gethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/evm/ante/evm"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/os/factory"
	"github.com/cosmos/evm/testutil/integration/os/grpc"
	testkeyring "github.com/cosmos/evm/testutil/integration/os/keyring"
	"github.com/cosmos/evm/testutil/integration/os/network"
	"github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
)

func (suite *EvmAnteTestSuite) TestCanTransfer() {
	keyring := testkeyring.New(1)
	senderKey := keyring.GetKey(0)

	var (
		unitNetwork *network.UnitTestNetwork
		grpcHandler grpc.Handler
		txFactory   factory.TxFactory
	)

	testCases := []struct {
		name          string
		expectedError error
		isLondon      bool
		malleate      func(txArgs *evmtypes.EvmTxArgs)
	}{
		{
			name:          "fail: isLondon and insufficient fee",
			expectedError: errortypes.ErrInsufficientFee,
			isLondon:      true,
			malleate: func(txArgs *evmtypes.EvmTxArgs) {
				txArgs.GasFeeCap = big.NewInt(0)
			},
		},
		{
			name:          "fail: invalid tx with insufficient balance",
			expectedError: errortypes.ErrInsufficientFunds,
			isLondon:      true,
			malleate: func(txArgs *evmtypes.EvmTxArgs) {
				balanceResp, err := grpcHandler.GetBalanceFromEVM(senderKey.AccAddr)
				suite.Require().NoError(err)

				balance, ok := math.NewIntFromString(balanceResp.Balance)
				suite.Require().True(ok)
				invalidAmount := balance.Add(math.NewInt(1)).BigInt()
				txArgs.Amount = invalidAmount
			},
		},
		{
			name:          "success: valid tx and sufficient balance",
			expectedError: nil,
			isLondon:      true,
			malleate: func(*evmtypes.EvmTxArgs) {
			},
		},
		{
			"fail: valid tx and insufficient balance with vesting tokens",
			errortypes.ErrInsufficientFunds,
			true,
			func(txArgs *evmtypes.EvmTxArgs) {
				balanceResp, err := grpcHandler.GetBalanceFromEVM(senderKey.AccAddr)
				suite.Require().NoError(err)

				balance, ok := math.NewIntFromString(balanceResp.Balance)
				suite.Require().True(ok)
				balance = balance.Quo(types.ConversionFactor())

				// replace with vesting account
				ctx := unitNetwork.GetContext()
				baseAccount := unitNetwork.App.AccountKeeper.GetAccount(ctx, senderKey.AccAddr).(*authtypes.BaseAccount)
				baseDenom := unitNetwork.GetBaseDenom()
				currTime := unitNetwork.GetContext().BlockTime().Unix()
				acc, err := vestingtypes.NewContinuousVestingAccount(baseAccount, sdk.NewCoins(sdk.NewCoin(baseDenom, balance)), unitNetwork.GetContext().BlockTime().Unix(), currTime+100)
				suite.Require().NoError(err)
				unitNetwork.App.AccountKeeper.SetAccount(ctx, acc)

				spendable := unitNetwork.App.BankKeeper.SpendableCoin(ctx, senderKey.AccAddr, baseDenom).Amount.Int64()
				suite.Require().Equal(spendable, int64(0))

				evmBalanceRes, err := grpcHandler.GetBalanceFromEVM(senderKey.AccAddr)
				suite.Require().NoError(err)
				evmBalance, ok := math.NewIntFromString(evmBalanceRes.Balance)
				suite.Require().True(ok)
				suite.Require().Equal(evmBalance.Int64(), int64(0))

				totalBalance := unitNetwork.App.BankKeeper.GetBalance(ctx, senderKey.AccAddr, baseDenom)
				suite.Require().Equal(totalBalance.Amount, balance)
			},
		},
		{
			"success: valid tx with vesting tokens",
			nil,
			true,
			func(txArgs *evmtypes.EvmTxArgs) {
				balanceResp, err := grpcHandler.GetBalanceFromEVM(senderKey.AccAddr)
				suite.Require().NoError(err)

				balance, ok := math.NewIntFromString(balanceResp.Balance)
				suite.Require().True(ok)
				balance = balance.Quo(types.ConversionFactor())

				// replace with vesting account
				ctx := unitNetwork.GetContext()
				baseAccount := unitNetwork.App.AccountKeeper.GetAccount(ctx, senderKey.AccAddr).(*authtypes.BaseAccount)
				baseDenom := unitNetwork.GetBaseDenom()
				currTime := unitNetwork.GetContext().BlockTime().Unix()
				acc, err := vestingtypes.NewContinuousVestingAccount(baseAccount, sdk.NewCoins(sdk.NewCoin(baseDenom, balance)), unitNetwork.GetContext().BlockTime().Unix(), currTime+100)
				suite.Require().NoError(err)
				unitNetwork.App.AccountKeeper.SetAccount(ctx, acc)

				spendable := unitNetwork.App.BankKeeper.SpendableCoin(ctx, senderKey.AccAddr, baseDenom).Amount
				suite.Require().Equal(spendable.String(), "0")

				evmBalanceRes, err := grpcHandler.GetBalanceFromEVM(senderKey.AccAddr)
				suite.Require().NoError(err)
				evmBalance := evmBalanceRes.Balance
				suite.Require().Equal(evmBalance, "0")

				totalBalance := unitNetwork.App.BankKeeper.GetBalance(ctx, senderKey.AccAddr, baseDenom)
				suite.Require().Equal(totalBalance.Amount, balance)

				mintAmt := sdk.NewCoins(sdk.NewCoin(baseDenom, balance))
				err = unitNetwork.App.BankKeeper.MintCoins(ctx, "mint", mintAmt)
				suite.Require().NoError(err)

				err = unitNetwork.App.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", senderKey.AccAddr, mintAmt)
				suite.Require().NoError(err)

				spendable = unitNetwork.App.BankKeeper.SpendableCoin(ctx, senderKey.AccAddr, baseDenom).Amount
				suite.Require().Equal(spendable.String(), balance.String())

				evmBalanceRes, err = grpcHandler.GetBalanceFromEVM(senderKey.AccAddr)
				suite.Require().NoError(err)
				evmBalance = evmBalanceRes.Balance
				suite.Require().Equal(evmBalance, balanceResp.Balance)

				totalBalance = unitNetwork.App.BankKeeper.GetBalance(ctx, senderKey.AccAddr, baseDenom)
				suite.Require().Equal(totalBalance.Amount, balance.Mul(math.NewInt(2)))
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("%v_%v_%v", evmtypes.GetTxTypeName(suite.ethTxType), suite.chainID, tc.name), func() {
			unitNetwork = network.NewUnitTestNetwork(
				network.WithChainID(testconstants.ChainID{
					ChainID:    suite.chainID,
					EVMChainID: suite.evmChainID,
				}),
				network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
			)

			grpcHandler = grpc.NewIntegrationHandler(unitNetwork)
			txFactory = factory.New(unitNetwork, grpcHandler)

			baseFeeResp, err := grpcHandler.GetEvmBaseFee()
			suite.Require().NoError(err)
			ethCfg := unitNetwork.GetEVMChainConfig()
			evmParams, err := grpcHandler.GetEvmParams()
			suite.Require().NoError(err)
			ctx := unitNetwork.GetContext()
			signer := gethtypes.MakeSigner(ethCfg, big.NewInt(ctx.BlockHeight()), uint64(ctx.BlockTime().Unix())) //#nosec G115 -- int overflow is not a concern here
			txArgs, err := txFactory.GenerateDefaultTxTypeArgs(senderKey.Addr, suite.ethTxType)
			suite.Require().NoError(err)
			txArgs.Amount = big.NewInt(100)

			tc.malleate(&txArgs)

			msg := evmtypes.NewTx(&txArgs)
			msg.From = senderKey.Addr.String()
			signMsg, err := txFactory.SignMsgEthereumTx(senderKey.Priv, *msg)
			suite.Require().NoError(err)
			coreMsg, err := signMsg.AsMessage(signer, baseFeeResp.BaseFee.BigInt())
			suite.Require().NoError(err)

			// Function under test
			err = evm.CanTransfer(
				unitNetwork.GetContext(),
				unitNetwork.App.EVMKeeper,
				*coreMsg,
				baseFeeResp.BaseFee.BigInt(),
				evmParams.Params,
				tc.isLondon,
			)

			if tc.expectedError != nil {
				suite.Require().Error(err)
				suite.Contains(err.Error(), tc.expectedError.Error())

			} else {
				suite.Require().NoError(err)
			}
		})
	}
}
