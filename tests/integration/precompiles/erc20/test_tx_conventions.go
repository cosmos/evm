package erc20

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/evm/precompiles/erc20"
	"github.com/cosmos/evm/precompiles/testutil"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/statedb"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// logsWithEvent returns the logs whose first topic is the given precompile event.
func (s *PrecompileTestSuite) logsWithEvent(stDB *statedb.StateDB, event string) []*ethtypes.Log {
	id := s.precompile.Events[event].ID
	var out []*ethtypes.Log
	for _, l := range stDB.Logs() {
		if len(l.Topics) > 0 && l.Topics[0] == id {
			out = append(out, l)
		}
	}
	return out
}

// fundOwner mints XMPLCoin to the given account.
func (s *PrecompileTestSuite) fundOwner(ctx sdk.Context, owner sdk.AccAddress) {
	s.Require().NoError(s.network.App.GetBankKeeper().MintCoins(ctx, erc20types.ModuleName, XMPLCoin))
	s.Require().NoError(s.network.App.GetBankKeeper().SendCoinsFromModuleToAccount(ctx, erc20types.ModuleName, owner, XMPLCoin))
}

// TestTransferZeroAmount checks EIP-20: "Transfers of 0 values MUST be treated as normal
// transfers and fire the Transfer event."
func (s *PrecompileTestSuite) TestTransferZeroAmount() {
	owner := s.keyring.GetKey(0)
	spender := s.keyring.GetKey(1)

	s.Run("transfer", func() {
		s.SetupTest()
		ctx := s.network.GetContext()
		stDB := s.network.GetStateDB()
		s.fundOwner(ctx, owner.AccAddr)

		contract, ctx := testutil.NewPrecompileContract(s.T(), ctx, owner.Addr, s.precompile.Address(), 0)
		method := s.precompile.Methods[erc20.TransferMethod]
		bz, err := s.precompile.Transfer(ctx, contract, stDB, &method, []interface{}{toAddr, big.NewInt(0)})
		s.Require().NoError(err, "a zero-amount transfer must succeed")

		out, err := method.Outputs.Unpack(bz)
		s.Require().NoError(err)
		s.Require().Equal(true, out[0])
		s.Require().Len(s.logsWithEvent(stDB, erc20.EventTypeTransfer), 1, "a zero-amount transfer must emit Transfer")
		s.Require().Equal(XMPLCoin.AmountOf(tokenDenom), s.network.App.GetBankKeeper().GetBalance(ctx, owner.AccAddr, tokenDenom).Amount)
		s.Require().True(s.network.App.GetBankKeeper().GetBalance(ctx, toAddr.Bytes(), tokenDenom).Amount.IsZero())
	})

	s.Run("transferFrom without allowance", func() {
		s.SetupTest()
		ctx := s.network.GetContext()
		stDB := s.network.GetStateDB()
		s.fundOwner(ctx, owner.AccAddr)

		contract, ctx := testutil.NewPrecompileContract(s.T(), ctx, spender.Addr, s.precompile.Address(), 0)
		method := s.precompile.Methods[erc20.TransferFromMethod]
		_, err := s.precompile.TransferFrom(ctx, contract, stDB, &method, []interface{}{owner.Addr, toAddr, big.NewInt(0)})
		s.Require().NoError(err, "a zero-amount transferFrom needs no allowance")
		s.Require().Len(s.logsWithEvent(stDB, erc20.EventTypeTransfer), 1)
	})
}

// TestTransferFromMaxAllowance checks that an allowance of type(uint256).max is treated
// as unlimited: transferFrom does not decrement it and does not emit Approval, as in
// OpenZeppelin's ERC20. A finite allowance is still decremented.
func (s *PrecompileTestSuite) TestTransferFromMaxAllowance() {
	owner := s.keyring.GetKey(0)
	spender := s.keyring.GetKey(1)

	cases := []struct {
		name         string
		allowance    *big.Int
		expAllowance *big.Int
		expApprovals int
	}{
		{"max allowance is not decremented", abi.MaxUint256, abi.MaxUint256, 0},
		{"finite allowance is decremented", big.NewInt(300), big.NewInt(200), 1},
		{"max allowance minus one is decremented", new(big.Int).Sub(abi.MaxUint256, big.NewInt(1)), new(big.Int).Sub(abi.MaxUint256, big.NewInt(101)), 1},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx := s.network.GetContext()
			stDB := s.network.GetStateDB()
			s.fundOwner(ctx, owner.AccAddr)
			erc20Keeper := s.network.App.GetErc20Keeper()
			s.Require().NoError(erc20Keeper.SetAllowance(ctx, s.precompile.Address(), owner.Addr, spender.Addr, tc.allowance))

			contract, ctx := testutil.NewPrecompileContract(s.T(), ctx, spender.Addr, s.precompile.Address(), 0)
			method := s.precompile.Methods[erc20.TransferFromMethod]
			_, err := s.precompile.TransferFrom(ctx, contract, stDB, &method, []interface{}{owner.Addr, toAddr, big.NewInt(100)})
			s.Require().NoError(err)

			got, err := erc20Keeper.GetAllowance(ctx, s.precompile.Address(), owner.Addr, spender.Addr)
			s.Require().NoError(err)
			s.Require().Equal(0, tc.expAllowance.Cmp(got), "allowance: got %s, want %s", got, tc.expAllowance)
			s.Require().Len(s.logsWithEvent(stDB, erc20.EventTypeApproval), tc.expApprovals)
			s.Require().Equal(int64(100), s.network.App.GetBankKeeper().GetBalance(ctx, toAddr.Bytes(), tokenDenom).Amount.Int64())
		})
	}
}
