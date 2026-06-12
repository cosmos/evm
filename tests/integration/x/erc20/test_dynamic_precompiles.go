package erc20

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

func (s *KeeperTestSuite) TestRegisterERC20CodeHash() {
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
				err := s.network.App.GetEVMKeeper().SetAccount(ctx, account, statedb.Account{
					CodeHash: codeHash,
					Nonce:    nonce,
					Balance:  balance,
				})
				s.Require().NoError(err)
			},
			true,
			false,
		},
		{
			"existent vesting account",
			func() {
				accountAddr := sdk.AccAddress(account.Bytes())
				err := s.network.App.GetBankKeeper().SendCoins(ctx, s.keyring.GetAccAddr(0), accountAddr, sdk.NewCoins(sdk.NewCoin(s.network.GetBaseDenom(), math.NewInt(balance.ToBig().Int64()))))
				s.Require().NoError(err)
				// replace with vesting account
				balanceResp, err := s.handler.GetBalanceFromEVM(accountAddr)
				s.Require().NoError(err)

				bal, ok := math.NewIntFromString(balanceResp.Balance)
				s.Require().True(ok)

				baseAccount := s.network.App.GetAccountKeeper().GetAccount(ctx, accountAddr).(*authtypes.BaseAccount)
				baseDenom := s.network.GetBaseDenom()
				currTime := s.network.GetContext().BlockTime().Unix()
				acc, err := vestingtypes.NewContinuousVestingAccount(baseAccount, sdk.NewCoins(sdk.NewCoin(baseDenom, bal)), s.network.GetContext().BlockTime().Unix(), currTime+100)
				s.Require().NoError(err)
				s.network.App.GetAccountKeeper().SetAccount(ctx, acc)

				spendable := s.network.App.GetBankKeeper().SpendableCoin(ctx, accountAddr, baseDenom).Amount
				s.Require().Equal(spendable.String(), "0")

				evmBalanceRes, err := s.handler.GetBalanceFromEVM(accountAddr)
				s.Require().NoError(err)
				evmBalance := evmBalanceRes.Balance
				s.Require().Equal(evmBalance, "0")

				err = s.network.App.GetEVMKeeper().SetAccount(ctx, account, statedb.Account{
					CodeHash: codeHash,
					Nonce:    nonce,
					Balance:  balance,
				})
				s.Require().NoError(err)
			},
			true,
			true,
		},
	}
	for _, tc := range testCases {
		s.SetupTest() // reset
		ctx = s.network.GetContext()
		tc.malleate()

		err := s.network.App.GetErc20Keeper().RegisterERC20CodeHash(ctx, account)
		s.Require().NoError(err)

		acc := s.network.App.GetEVMKeeper().GetAccount(ctx, account)
		s.Require().Equal(codeHash, acc.CodeHash)
		if tc.existent {
			s.Require().Equal(balance, acc.Balance)
			s.Require().Equal(nonce, acc.Nonce)
			if tc.vesting {
				totalBalance = s.network.App.GetBankKeeper().GetBalance(ctx, account.Bytes(), s.network.GetBaseDenom()).Amount
				s.Require().Equal(totalBalance.BigInt(), common.U2560.Add(balance, balance).ToBig())
			}
		} else {
			s.Require().Equal(common.U2560, acc.Balance)
			s.Require().Equal(uint64(0), acc.Nonce)
		}

		err = s.network.App.GetErc20Keeper().UnRegisterERC20CodeHash(ctx, account)
		s.Require().NoError(err)

		acc = s.network.App.GetEVMKeeper().GetAccount(ctx, account)
		s.Require().Equal(emptyCodeHash, acc.CodeHash)
		if tc.existent {
			s.Require().Equal(balance, acc.Balance)
			s.Require().Equal(nonce, acc.Nonce)
			if tc.vesting {
				totalBalance = s.network.App.GetBankKeeper().GetBalance(ctx, account.Bytes(), s.network.GetBaseDenom()).Amount
				s.Require().Equal(totalBalance.BigInt(), common.U2560.Add(balance, balance).ToBig())
			}
		} else {
			s.Require().Equal(common.U2560, acc.Balance)
			s.Require().Equal(uint64(0), acc.Nonce)
		}

	}
}
