package erc20

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/common/mocks"
	"github.com/cosmos/evm/precompiles/erc20"
	"github.com/cosmos/evm/precompiles/testutil"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/statedb"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/cosmos/cosmos-sdk/x/bank/types"
)

var (
	tokenDenom = "xmpl"
	// XMPLCoin is a dummy coin used for testing purposes.
	XMPLCoin = sdk.NewCoins(sdk.NewInt64Coin(tokenDenom, 1e18))
	// toAddr is a dummy address used for testing purposes.
	toAddr = GenerateAddress()
)

func (s *PrecompileTestSuite) TestTransfer() {
	method := s.precompile.Methods[erc20.TransferMethod]
	// fromAddr is the address of the keyring account used for testing.
	fromAddr := s.keyring.GetKey(0).Addr
	testcases := []struct {
		name        string
		malleate    func() []interface{}
		postCheck   func()
		expErr      bool
		errContains string
	}{
		{
			"fail - negative amount",
			func() []interface{} {
				return []interface{}{toAddr, big.NewInt(-1)}
			},
			func() {},
			true,
			"coin -1xmpl amount is not positive",
		},
		{
			"fail - invalid to address",
			func() []interface{} {
				return []interface{}{"", big.NewInt(100)}
			},
			func() {},
			true,
			"invalid to address",
		},
		{
			"fail - invalid amount",
			func() []interface{} {
				return []interface{}{toAddr, ""}
			},
			func() {},
			true,
			"invalid amount",
		},
		{
			"fail - not enough balance",
			func() []interface{} {
				return []interface{}{toAddr, big.NewInt(2e18)}
			},
			func() {},
			true,
			erc20.ErrTransferAmountExceedsBalance.Error(),
		},
		{
			"fail - not enough balance, sent amount is being vested",
			func() []interface{} {
				ctx := s.network.GetContext()
				accAddr := sdk.AccAddress(fromAddr.Bytes())
				err := s.network.App.GetBankKeeper().SendCoins(ctx, s.keyring.GetAccAddr(0), accAddr, sdk.NewCoins(sdk.NewCoin(s.network.GetBaseDenom(), math.NewInt(2e18))))
				s.Require().NoError(err)
				// replace with vesting account
				balanceResp, err := s.grpcHandler.GetBalanceFromEVM(accAddr)
				s.Require().NoError(err)

				balance, ok := math.NewIntFromString(balanceResp.Balance)
				s.Require().True(ok)

				baseAccount := s.network.App.GetAccountKeeper().GetAccount(ctx, accAddr).(*authtypes.BaseAccount)
				baseDenom := s.network.GetBaseDenom()
				currTime := s.network.GetContext().BlockTime().Unix()
				acc, err := vestingtypes.NewContinuousVestingAccount(baseAccount, sdk.NewCoins(sdk.NewCoin(baseDenom, balance)), s.network.GetContext().BlockTime().Unix(), currTime+100)
				s.Require().NoError(err)
				s.network.App.GetAccountKeeper().SetAccount(ctx, acc)

				spendable := s.network.App.GetBankKeeper().SpendableCoin(ctx, accAddr, baseDenom).Amount
				s.Require().Equal(spendable.String(), "0")

				evmBalanceRes, err := s.grpcHandler.GetBalanceFromEVM(accAddr)
				s.Require().NoError(err)
				evmBalance := evmBalanceRes.Balance
				s.Require().Equal(evmBalance, "0")

				tb, overflow := uint256.FromBig(s.network.App.GetBankKeeper().GetBalance(ctx, accAddr, baseDenom).Amount.BigInt())
				s.Require().False(overflow)
				s.Require().Equal(tb.ToBig(), balance.BigInt())

				return []interface{}{
					toAddr, big.NewInt(2e18),
				}
			},
			func() {},
			true,
			erc20.ErrTransferAmountExceedsBalance.Error(),
		},
		{
			"pass",
			func() []interface{} {
				return []interface{}{toAddr, big.NewInt(100)}
			},
			func() {
				toAddrBalance := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), toAddr.Bytes(), tokenDenom)
				s.Require().Equal(big.NewInt(100), toAddrBalance.Amount.BigInt(), "expected toAddr to have 100 XMPL")
			},
			false,
			"",
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			s.SetupTest()
			stateDB := s.network.GetStateDB()

			var contract *vm.Contract
			contract, ctx := testutil.NewPrecompileContract(s.T(), s.network.GetContext(), fromAddr, s.precompile.Address(), 0)

			// Mint some coins to the module account and then send to the from address
			err := s.network.App.GetBankKeeper().MintCoins(s.network.GetContext(), erc20types.ModuleName, XMPLCoin)
			s.Require().NoError(err, "failed to mint coins")
			err = s.network.App.GetBankKeeper().SendCoinsFromModuleToAccount(s.network.GetContext(), erc20types.ModuleName, fromAddr.Bytes(), XMPLCoin)
			s.Require().NoError(err, "failed to send coins from module to account")

			_, err = s.precompile.Transfer(ctx, contract, stateDB, &method, tc.malleate())
			if tc.expErr {
				s.Require().Error(err, "expected transfer transaction to fail")
				s.Require().Contains(err.Error(), tc.errContains, "expected transfer transaction to fail with specific error")
			} else {
				s.Require().NoError(err, "expected transfer transaction succeeded")
				tc.postCheck()
			}
		})
	}
}

func (s *PrecompileTestSuite) TestTransferFrom() {
	var (
		ctx  sdk.Context
		stDB *statedb.StateDB
	)
	method := s.precompile.Methods[erc20.TransferFromMethod]
	// owner of the tokens
	owner := s.keyring.GetKey(0)
	// spender of the tokens
	spender := s.keyring.GetKey(1)

	testcases := []struct {
		name        string
		malleate    func() []interface{}
		postCheck   func()
		expErr      bool
		errContains string
	}{
		{
			"fail - negative amount",
			func() []interface{} {
				return []interface{}{owner.Addr, toAddr, big.NewInt(-1)}
			},
			func() {},
			true,
			"coin -1xmpl amount is not positive",
		},
		{
			"fail - invalid from address",
			func() []interface{} {
				return []interface{}{"", toAddr, big.NewInt(100)}
			},
			func() {},
			true,
			"invalid from address",
		},
		{
			"fail - invalid to address",
			func() []interface{} {
				return []interface{}{owner.Addr, "", big.NewInt(100)}
			},
			func() {},
			true,
			"invalid to address",
		},
		{
			"fail - invalid amount",
			func() []interface{} {
				return []interface{}{owner.Addr, toAddr, ""}
			},
			func() {},
			true,
			"invalid amount",
		},
		{
			"fail - not enough allowance",
			func() []interface{} {
				return []interface{}{owner.Addr, toAddr, big.NewInt(100)}
			},
			func() {},
			true,
			erc20.ErrInsufficientAllowance.Error(),
		},
		{
			"fail - not enough balance",
			func() []interface{} {
				err := s.network.App.GetErc20Keeper().SetAllowance(s.network.GetContext(), s.precompile.Address(), owner.Addr, spender.Addr, big.NewInt(5e18))
				s.Require().NoError(err, "failed to set allowance")

				return []interface{}{owner.Addr, toAddr, big.NewInt(2e18)}
			},
			func() {},
			true,
			erc20.ErrTransferAmountExceedsBalance.Error(),
		},
		{
			"fail - spend on behalf of own account without allowance",
			func() []interface{} {
				// Mint some coins to the module account and then send to the spender address
				err := s.network.App.GetBankKeeper().MintCoins(ctx, erc20types.ModuleName, XMPLCoin)
				s.Require().NoError(err, "failed to mint coins")
				err = s.network.App.GetBankKeeper().SendCoinsFromModuleToAccount(ctx, erc20types.ModuleName, spender.AccAddr, XMPLCoin)
				s.Require().NoError(err, "failed to send coins from module to account")

				// NOTE: no allowance is necessary to spend on behalf of the same account
				return []interface{}{spender.Addr, toAddr, big.NewInt(100)}
			},
			func() {
				toAddrBalance := s.network.App.GetBankKeeper().GetBalance(ctx, toAddr.Bytes(), tokenDenom)
				s.Require().Equal(big.NewInt(100), toAddrBalance.Amount.BigInt(), "expected toAddr to have 100 XMPL")
			},
			true,
			"",
		},
		{
			"pass - spend on behalf of own account with allowance",
			func() []interface{} {
				// Mint some coins to the module account and then send to the spender address
				err := s.network.App.GetBankKeeper().MintCoins(ctx, erc20types.ModuleName, XMPLCoin)
				s.Require().NoError(err, "failed to mint coins")
				err = s.network.App.GetBankKeeper().SendCoinsFromModuleToAccount(ctx, erc20types.ModuleName, spender.AccAddr, XMPLCoin)
				s.Require().NoError(err, "failed to send coins from module to account")

				err = s.network.App.GetErc20Keeper().SetAllowance(ctx, s.precompile.Address(), spender.Addr, spender.Addr, big.NewInt(100))
				s.Require().NoError(err, "failed to set allowance")

				// NOTE: no allowance is necessary to spend on behalf of the same account
				return []interface{}{spender.Addr, toAddr, big.NewInt(100)}
			},
			func() {
				toAddrBalance := s.network.App.GetBankKeeper().GetBalance(ctx, toAddr.Bytes(), tokenDenom)
				s.Require().Equal(big.NewInt(100), toAddrBalance.Amount.BigInt(), "expected toAddr to have 100 XMPL")
			},
			false,
			"",
		},
		{
			"pass - spend on behalf of other account",
			func() []interface{} {
				err := s.network.App.GetErc20Keeper().SetAllowance(ctx, s.precompile.Address(), owner.Addr, spender.Addr, big.NewInt(300))
				s.Require().NoError(err, "failed to set allowance")

				return []interface{}{owner.Addr, toAddr, big.NewInt(100)}
			},
			func() {
				toAddrBalance := s.network.App.GetBankKeeper().GetBalance(ctx, toAddr.Bytes(), tokenDenom)
				s.Require().Equal(big.NewInt(100), toAddrBalance.Amount.BigInt(), "expected toAddr to have 100 XMPL")
			},
			false,
			"",
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			stDB = s.network.GetStateDB()

			var contract *vm.Contract
			contract, ctx = testutil.NewPrecompileContract(s.T(), ctx, spender.Addr, s.precompile.Address(), 0)

			// Mint some coins to the module account and then send to the from address
			err := s.network.App.GetBankKeeper().MintCoins(ctx, erc20types.ModuleName, XMPLCoin)
			s.Require().NoError(err, "failed to mint coins")
			err = s.network.App.GetBankKeeper().SendCoinsFromModuleToAccount(ctx, erc20types.ModuleName, owner.AccAddr, XMPLCoin)
			s.Require().NoError(err, "failed to send coins from module to account")

			_, err = s.precompile.TransferFrom(ctx, contract, stDB, &method, tc.malleate())
			if tc.expErr {
				s.Require().Error(err, "expected transfer transaction to fail")
				s.Require().Contains(err.Error(), tc.errContains, "expected transfer transaction to fail with specific error")
			} else {
				s.Require().NoError(err, "expected transfer transaction succeeded")
				tc.postCheck()
			}
		})
	}
}

func (s *PrecompileTestSuite) TestSend() {
	s.SetupTest()

	testcases := []struct {
		name     string
		malleate func() cmn.BankKeeper
		expFail  bool
	}{
		{
			name: "send with BankKeeper",
			malleate: func() cmn.BankKeeper {
				return s.network.App.GetBankKeeper()
			},
			expFail: false,
		},
		{
			name: "send with pointer to BaseKeeper",
			malleate: func() cmn.BankKeeper {
				bk, ok := s.network.App.GetBankKeeper().(bankkeeper.BaseKeeper)
				s.Require().True(ok, "expected app bank keeper to be a BaseKeeper value")
				return &bk
			},
			expFail: false,
		},
		{
			name: "send with a keeper wrapping the bank keeper",
			malleate: func() cmn.BankKeeper {
				return wrappedBankKeeper{Keeper: s.network.App.GetBankKeeper()}
			},
			expFail: false,
		},
		{
			name: "send with MockBankKeeper",
			malleate: func() cmn.BankKeeper {
				return mocks.NewBankKeeper(s.T())
			},
			expFail: true,
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			bankKeeper := tc.malleate()
			msgServ := erc20.NewMsgServerImpl(bankKeeper)
			s.Require().NotNil(msgServ)
			err := msgServ.Send(s.network.GetContext(), &types.MsgSend{
				FromAddress: s.keyring.GetAccAddr(0).String(),
				ToAddress:   s.keyring.GetAccAddr(1).String(),
				Amount:      sdk.NewCoins(sdk.NewCoin(vmtypes.GetEVMCoinExtendedDenom(), math.OneInt())),
			})
			if tc.expFail {
				s.Require().ErrorContains(err, "invalid keeper type")
			} else {
				s.Require().NoError(err)
			}
		})
	}
}

// TestSendChecks covers the checks Send runs before moving coins. Each
// rejected transfer must return its error without reaching SendCoins.
func (s *PrecompileTestSuite) TestSendChecks() {
	s.SetupTest()

	denom := vmtypes.GetEVMCoinExtendedDenom()
	from := s.keyring.GetAccAddr(0).String()
	to := s.keyring.GetAccAddr(1).String()
	oneCoin := sdk.NewCoins(sdk.NewCoin(denom, math.OneInt()))

	testcases := []struct {
		name        string
		msg         *types.MsgSend
		keeper      restrictedBankKeeper
		expErr      error
		errContains string
	}{
		{
			name: "pass - transfer allowed",
			msg:  &types.MsgSend{FromAddress: from, ToAddress: to, Amount: oneCoin},
		},
		{
			name:        "fail - invalid from address",
			msg:         &types.MsgSend{FromAddress: "invalid", ToAddress: to, Amount: oneCoin},
			expErr:      sdkerrors.ErrInvalidAddress,
			errContains: "invalid from address",
		},
		{
			name:        "fail - invalid to address",
			msg:         &types.MsgSend{FromAddress: from, ToAddress: "invalid", Amount: oneCoin},
			expErr:      sdkerrors.ErrInvalidAddress,
			errContains: "invalid to address",
		},
		{
			name:   "fail - zero amount",
			msg:    &types.MsgSend{FromAddress: from, ToAddress: to, Amount: sdk.Coins{sdk.NewCoin(denom, math.ZeroInt())}},
			expErr: sdkerrors.ErrInvalidCoins,
		},
		{
			name:   "fail - sends disabled for the denom",
			msg:    &types.MsgSend{FromAddress: from, ToAddress: to, Amount: oneCoin},
			keeper: restrictedBankKeeper{sendEnabledErr: types.ErrSendDisabled.Wrapf("%s transfers are currently disabled", denom)},
			expErr: types.ErrSendDisabled,
		},
		{
			name:        "fail - recipient is a blocked address",
			msg:         &types.MsgSend{FromAddress: from, ToAddress: to, Amount: oneCoin},
			keeper:      restrictedBankKeeper{blocked: true},
			expErr:      sdkerrors.ErrUnauthorized,
			errContains: "is not allowed to receive funds",
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			keeper := tc.keeper
			keeper.Keeper = s.network.App.GetBankKeeper()
			keeper.sendCalled = new(bool)

			err := erc20.NewMsgServerImpl(keeper).Send(s.network.GetContext(), tc.msg)
			if tc.expErr == nil {
				s.Require().NoError(err)
				s.Require().True(*keeper.sendCalled, "expected SendCoins to run")
				return
			}

			s.Require().ErrorIs(err, tc.expErr)
			if tc.errContains != "" {
				s.Require().ErrorContains(err, tc.errContains)
			}
			s.Require().False(*keeper.sendCalled, "SendCoins must not run when Send rejects the transfer")
		})
	}
}

// wrappedBankKeeper stands in for an app-level keeper that wraps the bank
// keeper (decimal conversion, metering, send restrictions).
type wrappedBankKeeper struct {
	bankkeeper.Keeper
}

// restrictedBankKeeper wraps the bank keeper so a test can force the
// send-enabled and blocked-recipient checks to fail, and records whether
// SendCoins was reached.
type restrictedBankKeeper struct {
	bankkeeper.Keeper
	sendEnabledErr error
	blocked        bool
	sendCalled     *bool
}

func (k restrictedBankKeeper) IsSendEnabledCoins(ctx context.Context, coins ...sdk.Coin) error {
	if k.sendEnabledErr != nil {
		return k.sendEnabledErr
	}
	return k.Keeper.IsSendEnabledCoins(ctx, coins...)
}

func (k restrictedBankKeeper) BlockedAddr(addr sdk.AccAddress) bool {
	return k.blocked || k.Keeper.BlockedAddr(addr)
}

func (k restrictedBankKeeper) SendCoins(ctx context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coins) error {
	*k.sendCalled = true
	return k.Keeper.SendCoins(ctx, fromAddr, toAddr, amt)
}
