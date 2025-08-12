package ante

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/ante/evm"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	"github.com/cosmos/evm/x/precisebank/types"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
)

func (s *EvmUnitAnteTestSuite) TestVerifyAccountBalance() {
	// Setup
	keyring := testkeyring.New(2)
	unitNetwork := network.NewUnitTestNetwork(
		s.create,
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
		network.WithChainID(testconstants.ChainID{
			ChainID:    s.ChainID,
			EVMChainID: s.EvmChainID,
		}),
	)
	grpcHandler := grpc.NewIntegrationHandler(unitNetwork)
	txFactory := factory.New(unitNetwork, grpcHandler)
	senderKey := keyring.GetKey(1)

	testCodeHash := common.BytesToHash([]byte("test_code_hash"))
	keeper := unitNetwork.App.GetEVMKeeper()
	keeper.SetCode(unitNetwork.GetContext(), testCodeHash.Bytes(), []byte("test_code"))

	testCases := []struct {
		name                   string
		expectedError          error
		generateAccountAndArgs func() (*statedb.Account, evmtypes.EvmTxArgs)
	}{
		{
			name:          "fail: sender is not EOA",
			expectedError: errortypes.ErrInvalidType,
			generateAccountAndArgs: func() (*statedb.Account, evmtypes.EvmTxArgs) {
				statedbAccount := getDefaultStateDBAccount(unitNetwork, senderKey.Addr)
				txArgs, err := txFactory.GenerateDefaultTxTypeArgs(senderKey.Addr, s.EthTxType)
				s.Require().NoError(err)

				statedbAccount.CodeHash = testCodeHash.Bytes()
				s.Require().NoError(err)
				return statedbAccount, txArgs
			},
		},
		{
			name:          "fail: sender balance is lower than the transaction cost",
			expectedError: errortypes.ErrInsufficientFunds,
			generateAccountAndArgs: func() (*statedb.Account, evmtypes.EvmTxArgs) {
				statedbAccount := getDefaultStateDBAccount(unitNetwork, senderKey.Addr)
				txArgs, err := txFactory.GenerateDefaultTxTypeArgs(senderKey.Addr, s.EthTxType)
				s.Require().NoError(err)

				// Make tx cost greater than balance
				balanceResp, err := grpcHandler.GetBalanceFromEVM(senderKey.AccAddr)
				s.Require().NoError(err)

				balance, ok := math.NewIntFromString(balanceResp.Balance)
				s.Require().True(ok)
				invalidAmount := balance.Add(math.NewInt(100))
				txArgs.Amount = invalidAmount.BigInt()
				return statedbAccount, txArgs
			},
		},
		{
			name:          "fail: tx cost is negative",
			expectedError: errortypes.ErrInvalidCoins,
			generateAccountAndArgs: func() (*statedb.Account, evmtypes.EvmTxArgs) {
				statedbAccount := getDefaultStateDBAccount(unitNetwork, senderKey.Addr)
				txArgs, err := txFactory.GenerateDefaultTxTypeArgs(senderKey.Addr, s.EthTxType)
				s.Require().NoError(err)

				// Make tx cost negative. This has to be a big value because
				// it has to be bigger than the fee for the full cost to be negative
				invalidAmount := big.NewInt(-1e18)
				txArgs.Amount = invalidAmount
				return statedbAccount, txArgs
			},
		},
		{
			name:          "fail: sender spendable balance is lower than the transaction cost, total balance equals transaction cost",
			expectedError: errortypes.ErrInsufficientFunds,
			generateAccountAndArgs: func() (*statedb.Account, evmtypes.EvmTxArgs) {
				txArgs, err := txFactory.GenerateDefaultTxTypeArgs(senderKey.Addr, s.EthTxType)
				s.Require().NoError(err)

				// Make tx cost greater than balance
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

				statedbAccount := getDefaultStateDBAccount(unitNetwork, senderKey.Addr)
				s.Require().Equal(spendable.String(), math.NewIntFromBigInt(statedbAccount.Balance.ToBig()).Quo(types.ConversionFactor()).String())
				return statedbAccount, txArgs
			},
		},
		{
			name:          "success: tx cost equals spendable balance in vesting account",
			expectedError: nil,
			generateAccountAndArgs: func() (*statedb.Account, evmtypes.EvmTxArgs) {
				txArgs, err := txFactory.GenerateDefaultTxTypeArgs(senderKey.Addr, s.EthTxType)
				s.Require().NoError(err)

				// Make tx cost greater than balance
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

				statedbAccount := getDefaultStateDBAccount(unitNetwork, senderKey.Addr)
				s.Require().Equal(spendable.String(), math.NewIntFromBigInt(statedbAccount.Balance.ToBig()).Quo(types.ConversionFactor()).String())
				return statedbAccount, txArgs
			},
		},
		{
			name:          "success: tx is successful and account is created if its nil",
			expectedError: errortypes.ErrInsufficientFunds,
			generateAccountAndArgs: func() (*statedb.Account, evmtypes.EvmTxArgs) {
				txArgs, err := txFactory.GenerateDefaultTxTypeArgs(senderKey.Addr, s.EthTxType)
				s.Require().NoError(err)
				return nil, txArgs
			},
		},
		{
			name:          "success: tx is successful if account is EOA and exists",
			expectedError: nil,
			generateAccountAndArgs: func() (*statedb.Account, evmtypes.EvmTxArgs) {
				statedbAccount := getDefaultStateDBAccount(unitNetwork, senderKey.Addr)
				txArgs, err := txFactory.GenerateDefaultTxTypeArgs(senderKey.Addr, s.EthTxType)
				s.Require().NoError(err)
				return statedbAccount, txArgs
			},
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("%v_%v_%v", evmtypes.GetTxTypeName(s.EthTxType), s.ChainID, tc.name), func() {
			// Perform test logic
			statedbAccount, txArgs := tc.generateAccountAndArgs()
			ethTx := txArgs.ToTx()

			//  Function to be tested
			err := evm.VerifyAccountBalance(
				unitNetwork.GetContext(),
				unitNetwork.App.GetEVMKeeper(),
				unitNetwork.App.GetAccountKeeper(),
				statedbAccount,
				senderKey.Addr,
				ethTx,
			)

			if tc.expectedError != nil {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.expectedError.Error())
			} else {
				s.Require().NoError(err)
			}
			// Make sure the account is created either wa
			acc, err := grpcHandler.GetAccount(senderKey.AccAddr.String())
			s.Require().NoError(err)
			s.Require().NotEmpty(acc)

			// Clean block for next test
			err = unitNetwork.NextBlock()
			s.Require().NoError(err)
		})
	}
}

func getDefaultStateDBAccount(unitNetwork *network.UnitTestNetwork, addr common.Address) *statedb.Account {
	statedb := unitNetwork.GetStateDB()
	return statedb.Keeper().GetAccount(unitNetwork.GetContext(), addr)
}
