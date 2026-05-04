package keeper_test

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/statedb"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
)

func (suite *KeeperTestSuite) TestRegisterERC20CodeHash() {
	var (
		ctx sdk.Context
		// bytecode and codeHash is the same for all IBC coins
		// cause they're all using the same contract
		bytecode             = common.FromHex(types.Erc20Bytecode)
		codeHash             = crypto.Keccak256(bytecode)
		nonce         uint64 = 10
		balance              = uint256.NewInt(100)
		emptyCodeHash        = crypto.Keccak256(nil)
		totalBalance         = math.NewInt(100)
	)

	account := utiltx.GenerateAddress()

	testCases := []struct {
		name     string
		malleate func()
		existent bool
		vesting  bool
	}{
		{
			"ok",
			func() {
			},
			false,
			false,
		},
		{
			"existent account",
			func() {
				err := suite.network.App.EVMKeeper.SetAccount(ctx, account, statedb.Account{
					CodeHash: codeHash,
					Nonce:    nonce,
					Balance:  balance,
				})
				suite.Require().NoError(err)
			},
			true,
			false,
		},
		{
			"existent vesting account",
			func() {
				accountAddr := sdk.AccAddress(account.Bytes())
				err := suite.network.App.BankKeeper.SendCoins(ctx, suite.keyring.GetAccAddr(0), accountAddr, sdk.NewCoins(sdk.NewCoin(suite.network.GetBaseDenom(), math.NewInt(balance.ToBig().Int64()))))
				suite.Require().NoError(err)
				// replace with vesting account
				balanceResp, err := suite.handler.GetBalanceFromEVM(accountAddr)
				suite.Require().NoError(err)

				bal, ok := math.NewIntFromString(balanceResp.Balance)
				suite.Require().True(ok)

				baseAccount := suite.network.App.AccountKeeper.GetAccount(ctx, accountAddr).(*authtypes.BaseAccount)
				baseDenom := suite.network.GetBaseDenom()
				currTime := suite.network.GetContext().BlockTime().Unix()
				acc, err := vestingtypes.NewContinuousVestingAccount(baseAccount, sdk.NewCoins(sdk.NewCoin(baseDenom, bal)), suite.network.GetContext().BlockTime().Unix(), currTime+100)
				suite.Require().NoError(err)
				suite.network.App.AccountKeeper.SetAccount(ctx, acc)

				spendable := suite.network.App.BankKeeper.SpendableCoin(ctx, accountAddr, baseDenom).Amount
				suite.Require().Equal(spendable.String(), "0")

				evmBalanceRes, err := suite.handler.GetBalanceFromEVM(accountAddr)
				suite.Require().NoError(err)
				evmBalance := evmBalanceRes.Balance
				suite.Require().Equal(evmBalance, "0")

				err = suite.network.App.EVMKeeper.SetAccount(ctx, account, statedb.Account{
					CodeHash: codeHash,
					Nonce:    nonce,
					Balance:  balance,
				})
				suite.Require().NoError(err)
			},
			true,
			true,
		},
	}
	for _, tc := range testCases {
		suite.SetupTest() // reset
		ctx = suite.network.GetContext()
		tc.malleate()

		err := suite.network.App.Erc20Keeper.RegisterERC20CodeHash(ctx, account)
		suite.Require().NoError(err)

		acc := suite.network.App.EVMKeeper.GetAccount(ctx, account)
		suite.Require().Equal(codeHash, acc.CodeHash)
		if tc.existent {
			suite.Require().Equal(balance, acc.Balance)
			suite.Require().Equal(nonce, acc.Nonce)
			if tc.vesting {
				totalBalance = suite.network.App.BankKeeper.GetBalance(ctx, account.Bytes(), suite.network.GetBaseDenom()).Amount
				suite.Require().Equal(totalBalance.BigInt(), common.U2560.Add(balance, balance).ToBig())
			}
		} else {
			suite.Require().Equal(common.U2560, acc.Balance)
			suite.Require().Equal(uint64(0), acc.Nonce)
		}

		err = suite.network.App.Erc20Keeper.UnRegisterERC20CodeHash(ctx, account)
		suite.Require().NoError(err)

		acc = suite.network.App.EVMKeeper.GetAccount(ctx, account)
		suite.Require().Equal(emptyCodeHash, acc.CodeHash)
		if tc.existent {
			suite.Require().Equal(balance, acc.Balance)
			suite.Require().Equal(nonce, acc.Nonce)
			if tc.vesting {
				totalBalance = suite.network.App.BankKeeper.GetBalance(ctx, account.Bytes(), suite.network.GetBaseDenom()).Amount
				suite.Require().Equal(totalBalance.BigInt(), common.U2560.Add(balance, balance).ToBig())
			}
		} else {
			suite.Require().Equal(common.U2560, acc.Balance)
			suite.Require().Equal(uint64(0), acc.Nonce)
		}

	}
}
