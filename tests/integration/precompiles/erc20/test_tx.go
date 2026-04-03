package erc20

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
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
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
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
	fromAddr := s.keyring.GetKey(0).Addr
	negCoinErr := sdk.Coins{{Denom: tokenDenom, Amount: math.NewIntFromBigInt(big.NewInt(-1))}}.Validate()
	s.Require().Error(negCoinErr)
	testcases := []struct {
		name        string
		malleate    func() []interface{}
		postCheck   func()
		expErr      bool
		errContains string
		wantErr     error
	}{
		{
			"fail - negative amount",
			func() []interface{} {
				return []interface{}{toAddr, big.NewInt(-1)}
			},
			func() {},
			true,
			"",
			cmn.NewRevertWithSolidityError(s.precompile.ABI, cmn.SolidityErrInvalidAmount, negCoinErr.Error()),
		},
		{
			"fail - invalid to address",
			func() []interface{} {
				return []interface{}{"", big.NewInt(100)}
			},
			func() {},
			true,
			"",
			cmn.NewRevertWithSolidityError(s.precompile.ABI, cmn.SolidityErrInvalidAddress, ""),
		},
		{
			"fail - invalid amount",
			func() []interface{} {
				return []interface{}{toAddr, ""}
			},
			func() {},
			true,
			"",
			cmn.NewRevertWithSolidityError(s.precompile.ABI, cmn.SolidityErrInvalidAmount, ""),
		},
		{
			"fail - not enough balance",
			func() []interface{} {
				return []interface{}{toAddr, big.NewInt(2e18)}
			},
			func() {},
			true,
			"",
			cmn.NewRevertWithSolidityError(s.precompile.ABI, erc20.SolidityErrERC20InsufficientBalance, fromAddr, big.NewInt(1e18), big.NewInt(2e18)),
		},
		{
			"fail - not enough balance, sent amount is being vested",
			func() []interface{} {
				ctx := s.network.GetContext()
				accAddr := sdk.AccAddress(fromAddr.Bytes())
				err := s.network.App.GetBankKeeper().SendCoins(ctx, s.keyring.GetAccAddr(0), accAddr, sdk.NewCoins(sdk.NewCoin(s.network.GetBaseDenom(), math.NewInt(2e18))))
				s.Require().NoError(err)
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
			"",
			cmn.NewRevertWithSolidityError(s.precompile.ABI, erc20.SolidityErrERC20InsufficientBalance, fromAddr, big.NewInt(1e18), big.NewInt(2e18)),
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
			nil,
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			s.SetupTest()
			stateDB := s.network.GetStateDB()

			contract, ctx := testutil.NewPrecompileContract(s.T(), s.network.GetContext(), fromAddr, s.precompile.Address(), 0)

			err := s.network.App.GetBankKeeper().MintCoins(s.network.GetContext(), erc20types.ModuleName, XMPLCoin)
			s.Require().NoError(err, "failed to mint coins")
			err = s.network.App.GetBankKeeper().SendCoinsFromModuleToAccount(s.network.GetContext(), erc20types.ModuleName, fromAddr.Bytes(), XMPLCoin)
			s.Require().NoError(err, "failed to send coins from module to account")

			_, err = s.precompile.Transfer(ctx, contract, stateDB, &method, tc.malleate())
			if tc.expErr {
				s.Require().Error(err, "expected transfer transaction to fail")
				if tc.wantErr != nil {
					testutil.RequireExactError(s.T(), err, tc.wantErr)
				} else {
					s.Require().Contains(err.Error(), tc.errContains, "expected transfer transaction to fail with specific error")
				}
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
	owner := s.keyring.GetKey(0)
	spender := s.keyring.GetKey(1)
	negCoinErr := sdk.Coins{{Denom: tokenDenom, Amount: math.NewIntFromBigInt(big.NewInt(-1))}}.Validate()
	s.Require().Error(negCoinErr)

	testcases := []struct {
		name        string
		malleate    func() []interface{}
		postCheck   func()
		expErr      bool
		errContains string
		wantErr     error
	}{
		{
			"fail - negative amount",
			func() []interface{} {
				return []interface{}{owner.Addr, toAddr, big.NewInt(-1)}
			},
			func() {},
			true,
			"",
			cmn.NewRevertWithSolidityError(s.precompile.ABI, cmn.SolidityErrInvalidAmount, negCoinErr.Error()),
		},
		{
			"fail - invalid from address",
			func() []interface{} {
				return []interface{}{"", toAddr, big.NewInt(100)}
			},
			func() {},
			true,
			"",
			cmn.NewRevertWithSolidityError(s.precompile.ABI, cmn.SolidityErrInvalidAddress, ""),
		},
		{
			"fail - invalid to address",
			func() []interface{} {
				return []interface{}{owner.Addr, "", big.NewInt(100)}
			},
			func() {},
			true,
			"",
			cmn.NewRevertWithSolidityError(s.precompile.ABI, cmn.SolidityErrInvalidAddress, ""),
		},
		{
			"fail - invalid amount",
			func() []interface{} {
				return []interface{}{owner.Addr, toAddr, ""}
			},
			func() {},
			true,
			"",
			cmn.NewRevertWithSolidityError(s.precompile.ABI, cmn.SolidityErrInvalidAmount, ""),
		},
		{
			"fail - not enough allowance",
			func() []interface{} {
				return []interface{}{owner.Addr, toAddr, big.NewInt(100)}
			},
			func() {},
			true,
			"",
			cmn.NewRevertWithSolidityError(s.precompile.ABI, erc20.SolidityErrERC20InsufficientAllowance, spender.Addr, common.Big0, big.NewInt(100)),
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
			"",
			cmn.NewRevertWithSolidityError(s.precompile.ABI, erc20.SolidityErrERC20InsufficientBalance, owner.Addr, big.NewInt(1e18), big.NewInt(2e18)),
		},
		{
			"fail - spend on behalf of own account without allowance",
			func() []interface{} {
				err := s.network.App.GetBankKeeper().MintCoins(ctx, erc20types.ModuleName, XMPLCoin)
				s.Require().NoError(err, "failed to mint coins")
				err = s.network.App.GetBankKeeper().SendCoinsFromModuleToAccount(ctx, erc20types.ModuleName, spender.AccAddr, XMPLCoin)
				s.Require().NoError(err, "failed to send coins from module to account")

				return []interface{}{spender.Addr, toAddr, big.NewInt(100)}
			},
			func() {},
			true,
			"",
			cmn.NewRevertWithSolidityError(s.precompile.ABI, erc20.SolidityErrERC20InsufficientAllowance, spender.Addr, common.Big0, big.NewInt(100)),
		},
		{
			"pass - spend on behalf of own account with allowance",
			func() []interface{} {
				err := s.network.App.GetBankKeeper().MintCoins(ctx, erc20types.ModuleName, XMPLCoin)
				s.Require().NoError(err, "failed to mint coins")
				err = s.network.App.GetBankKeeper().SendCoinsFromModuleToAccount(ctx, erc20types.ModuleName, spender.AccAddr, XMPLCoin)
				s.Require().NoError(err, "failed to send coins from module to account")

				err = s.network.App.GetErc20Keeper().SetAllowance(ctx, s.precompile.Address(), spender.Addr, spender.Addr, big.NewInt(100))
				s.Require().NoError(err, "failed to set allowance")

				return []interface{}{spender.Addr, toAddr, big.NewInt(100)}
			},
			func() {
				toAddrBalance := s.network.App.GetBankKeeper().GetBalance(ctx, toAddr.Bytes(), tokenDenom)
				s.Require().Equal(big.NewInt(100), toAddrBalance.Amount.BigInt(), "expected toAddr to have 100 XMPL")
			},
			false,
			"",
			nil,
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
			nil,
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			stDB = s.network.GetStateDB()

			var contract *vm.Contract
			contract, ctx = testutil.NewPrecompileContract(s.T(), ctx, spender.Addr, s.precompile.Address(), 0)

			err := s.network.App.GetBankKeeper().MintCoins(ctx, erc20types.ModuleName, XMPLCoin)
			s.Require().NoError(err, "failed to mint coins")
			err = s.network.App.GetBankKeeper().SendCoinsFromModuleToAccount(ctx, erc20types.ModuleName, owner.AccAddr, XMPLCoin)
			s.Require().NoError(err, "failed to send coins from module to account")

			_, err = s.precompile.TransferFrom(ctx, contract, stDB, &method, tc.malleate())
			if tc.expErr {
				s.Require().Error(err, "expected transfer transaction to fail")
				if tc.wantErr != nil {
					testutil.RequireExactError(s.T(), err, tc.wantErr)
				} else {
					s.Require().Contains(err.Error(), tc.errContains, "expected transfer transaction to fail with specific error")
				}
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
