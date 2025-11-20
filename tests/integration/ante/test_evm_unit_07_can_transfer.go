package ante

import (
	"fmt"
	"math/big"

	"github.com/cosmos/evm/ante/evm"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	"github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
)

func (s *EvmUnitAnteTestSuite) TestCanTransfer() {
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
				txArgs.GasPrice = nil // make sure it's not legacy tx
				txArgs.GasFeeCap = big.NewInt(0)
			},
		},
		{
			name:          "fail: invalid tx with insufficient balance",
			expectedError: errortypes.ErrInsufficientFunds,
			isLondon:      true,
			malleate: func(txArgs *evmtypes.EvmTxArgs) {
				balanceResp, err := grpcHandler.GetBalanceFromEVM(senderKey.AccAddr)
				s.Require().NoError(err)

				balance, ok := math.NewIntFromString(balanceResp.Balance)
				s.Require().True(ok)
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
				s.Require().NoError(err)

				balance, ok := math.NewIntFromString(balanceResp.Balance)
				s.Require().True(ok)
				balance = balance.Quo(types.ConversionFactor())

				// replace with vesting account
				ctx := unitNetwork.GetContext()
				baseAccount := unitNetwork.App.GetAccountKeeper().GetAccount(ctx, senderKey.AccAddr).(*authtypes.BaseAccount)
				baseDenom := unitNetwork.GetBaseDenom()
				currTime := unitNetwork.GetContext().BlockTime().Unix()
				acc, err := vestingtypes.NewContinuousVestingAccount(baseAccount, sdk.NewCoins(sdk.NewCoin(baseDenom, balance)), unitNetwork.GetContext().BlockTime().Unix(), currTime+100)
				s.Require().NoError(err)
				unitNetwork.App.GetAccountKeeper().SetAccount(ctx, acc)

				spendable := unitNetwork.App.GetBankKeeper().SpendableCoin(ctx, senderKey.AccAddr, baseDenom).Amount.Int64()
				s.Require().Equal(spendable, int64(0))

				evmBalanceRes, err := grpcHandler.GetBalanceFromEVM(senderKey.AccAddr)
				s.Require().NoError(err)
				evmBalance, ok := math.NewIntFromString(evmBalanceRes.Balance)
				s.Require().True(ok)
				s.Require().Equal(evmBalance.Int64(), int64(0))

				totalBalance := unitNetwork.App.GetBankKeeper().GetBalance(ctx, senderKey.AccAddr, baseDenom)
				s.Require().Equal(totalBalance.Amount, balance)
			},
		},
		{
			"success: valid tx with vesting tokens",
			nil,
			true,
			func(txArgs *evmtypes.EvmTxArgs) {
				balanceResp, err := grpcHandler.GetBalanceFromEVM(senderKey.AccAddr)
				s.Require().NoError(err)

				balance, ok := math.NewIntFromString(balanceResp.Balance)
				s.Require().True(ok)
				balance = balance.Quo(types.ConversionFactor())

				// replace with vesting account
				ctx := unitNetwork.GetContext()
				baseAccount := unitNetwork.App.GetAccountKeeper().GetAccount(ctx, senderKey.AccAddr).(*authtypes.BaseAccount)
				baseDenom := unitNetwork.GetBaseDenom()
				currTime := unitNetwork.GetContext().BlockTime().Unix()
				acc, err := vestingtypes.NewContinuousVestingAccount(baseAccount, sdk.NewCoins(sdk.NewCoin(baseDenom, balance)), unitNetwork.GetContext().BlockTime().Unix(), currTime+100)
				s.Require().NoError(err)
				unitNetwork.App.GetAccountKeeper().SetAccount(ctx, acc)

				spendable := unitNetwork.App.GetBankKeeper().SpendableCoin(ctx, senderKey.AccAddr, baseDenom).Amount
				s.Require().Equal(spendable.String(), "0")

				evmBalanceRes, err := grpcHandler.GetBalanceFromEVM(senderKey.AccAddr)
				s.Require().NoError(err)
				evmBalance := evmBalanceRes.Balance
				s.Require().Equal(evmBalance, "0")

				totalBalance := unitNetwork.App.GetBankKeeper().GetBalance(ctx, senderKey.AccAddr, baseDenom)
				s.Require().Equal(totalBalance.Amount, balance)

				mintAmt := sdk.NewCoins(sdk.NewCoin(baseDenom, balance))
				err = unitNetwork.App.GetBankKeeper().MintCoins(ctx, "mint", mintAmt)
				s.Require().NoError(err)

				err = unitNetwork.App.GetBankKeeper().SendCoinsFromModuleToAccount(ctx, "mint", senderKey.AccAddr, mintAmt)
				s.Require().NoError(err)

				spendable = unitNetwork.App.GetBankKeeper().SpendableCoin(ctx, senderKey.AccAddr, baseDenom).Amount
				s.Require().Equal(spendable.String(), balance.String())

				evmBalanceRes, err = grpcHandler.GetBalanceFromEVM(senderKey.AccAddr)
				s.Require().NoError(err)
				evmBalance = evmBalanceRes.Balance
				s.Require().Equal(evmBalance, balanceResp.Balance)

				totalBalance = unitNetwork.App.GetBankKeeper().GetBalance(ctx, senderKey.AccAddr, baseDenom)
				s.Require().Equal(totalBalance.Amount, balance.Mul(math.NewInt(2)))
			},
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("%v_%v_%v", evmtypes.GetTxTypeName(s.EthTxType), s.ChainID, tc.name), func() {
			unitNetwork = network.NewUnitTestNetwork(
				s.create,
				network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
			)

			grpcHandler = grpc.NewIntegrationHandler(unitNetwork)
			txFactory = factory.New(unitNetwork, grpcHandler)

			baseFeeResp, err := grpcHandler.GetEvmBaseFee()
			s.Require().NoError(err)
			evmParams, err := grpcHandler.GetEvmParams()
			s.Require().NoError(err)
			txArgs, err := txFactory.GenerateDefaultTxTypeArgs(senderKey.Addr, s.EthTxType)
			s.Require().NoError(err)
			txArgs.Amount = big.NewInt(100)

			tc.malleate(&txArgs)

			msg := evmtypes.NewTx(&txArgs)
			msg.From = senderKey.Addr.Bytes()
			signMsg, err := txFactory.SignMsgEthereumTx(senderKey.Priv, *msg)
			s.Require().NoError(err)
			coreMsg := signMsg.AsMessage(baseFeeResp.BaseFee.BigInt())

			// Function under test
			err = evm.CanTransfer(
				unitNetwork.GetContext(),
				unitNetwork.App.GetEVMKeeper(),
				*coreMsg,
				baseFeeResp.BaseFee.BigInt(),
				evmParams.Params,
				tc.isLondon,
			)

			if tc.expectedError != nil {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.expectedError.Error())

			} else {
				s.Require().NoError(err)
			}
		})
	}
}
