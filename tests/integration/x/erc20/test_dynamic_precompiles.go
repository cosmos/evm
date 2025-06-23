package erc20

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/statedb"

	sdk "github.com/cosmos/cosmos-sdk/types"
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
	)

	account := utiltx.GenerateAddress()

	testCases := []struct {
		name     string
		malleate func()
		existent bool
	}{
		{
			"ok",
			func() {
			},
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
		} else {
			s.Require().Equal(common.U2560, acc.Balance)
			s.Require().Equal(uint64(0), acc.Nonce)
		}

	}
}
