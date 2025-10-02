package vm

import (
	"math/big"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	"github.com/cosmos/evm/testutil/integration/evm/utils"
	"github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

func (s *KeeperTestSuite) TestEthereumTx() {
	s.EnableFeemarket = true
	defer func() { s.EnableFeemarket = false }()
	s.SetupTest()

	args := types.EvmTxArgs{
		// Have insufficient gas
		GasLimit: 10,
	}
	_, err := s.Factory.GenerateSignedEthTx(s.Keyring.GetPrivKey(0), args)
	s.Require().Error(err)

	testCases := []struct {
		name        string
		getMsg      func() *types.MsgEthereumTx
		expectedErr error
		postCheck   func()
	}{
		{
			name: "success - transfer funds tx",
			getMsg: func() *types.MsgEthereumTx {
				recipient := s.Keyring.GetAddr(1)
				args := types.EvmTxArgs{
					To:     &recipient,
					Amount: big.NewInt(1e18),
				}
				tx, err := s.Factory.GenerateSignedEthTx(s.Keyring.GetPrivKey(0), args)
				s.Require().NoError(err)
				return tx.GetMsgs()[0].(*types.MsgEthereumTx)
			},
			expectedErr: nil,
			postCheck:   nil,
		},
		{
			name: "success - set code authorization tx (EIP-7702)",
			getMsg: func() *types.MsgEthereumTx {
				authority := s.Keyring.GetKey(0)
				target := s.Keyring.GetAddr(1)

				accResp, err := s.Handler.GetEvmAccount(authority.Addr)
				s.Require().NoError(err)

				auth := ethtypes.SetCodeAuthorization{
					ChainID: *uint256.NewInt(types.GetChainConfig().GetChainId()),
					Address: target,
					Nonce:   accResp.GetNonce(),
				}
				signedAuth := s.SignSetCodeAuthorization(authority, auth)

				args := types.EvmTxArgs{
					To:                &target,
					AuthorizationList: []ethtypes.SetCodeAuthorization{signedAuth},
				}
				tx, err := s.Factory.GenerateSignedEthTx(s.Keyring.GetPrivKey(0), args)
				s.Require().NoError(err)
				return tx.GetMsgs()[0].(*types.MsgEthereumTx)
			},
			expectedErr: nil,
			postCheck: func() {
				authorityAddr := s.Keyring.GetAddr(0)
				targetAddr := s.Keyring.GetAddr(1)
				codeHash := s.Network.App.GetEVMKeeper().GetCodeHash(s.Network.GetContext(), authorityAddr)
				code := s.Network.App.GetEVMKeeper().GetCode(s.Network.GetContext(), codeHash)
				delegationAddr, ok := ethtypes.ParseDelegation(code)
				s.Require().True(ok)
				s.Require().Equal(targetAddr, delegationAddr)
			},
		},
		{
			name: "fail - unsigned set code authorization",
			getMsg: func() *types.MsgEthereumTx {
				authority := s.Keyring.GetKey(0)
				target := s.Keyring.GetAddr(1)

				accResp, err := s.Handler.GetEvmAccount(authority.Addr)
				s.Require().NoError(err)

				auth := ethtypes.SetCodeAuthorization{
					ChainID: *uint256.NewInt(types.GetChainConfig().GetChainId()),
					Address: target,
					Nonce:   accResp.GetNonce(),
				}

				args := types.EvmTxArgs{
					To:                &target,
					AuthorizationList: []ethtypes.SetCodeAuthorization{auth},
				}
				tx, err := s.Factory.GenerateSignedEthTx(s.Keyring.GetPrivKey(0), args)
				s.Require().NoError(err)
				return tx.GetMsgs()[0].(*types.MsgEthereumTx)
			},
			expectedErr: nil,
			postCheck: func() {
				authorityAddr := s.Keyring.GetAddr(0)
				codeHash := s.Network.App.GetEVMKeeper().GetCodeHash(s.Network.GetContext(), authorityAddr)
				code := s.Network.App.GetEVMKeeper().GetCode(s.Network.GetContext(), codeHash)
				_, ok := ethtypes.ParseDelegation(code)
				s.Require().False(ok)
				s.Require().Len(code, 0)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Fund fee collector account
			ctx := s.Network.GetContext()
			coins := sdktypes.NewCoins(sdktypes.NewCoin(types.GetEVMCoinDenom(), sdkmath.NewInt(1e18)))
			err := s.Network.App.GetBankKeeper().MintCoins(ctx, "mint", coins)
			s.Require().NoError(err)
			err = s.Network.App.GetBankKeeper().SendCoinsFromModuleToModule(ctx, "mint", "fee_collector", coins)
			s.Require().NoError(err)

			// Get EthereumTx msg
			msg := tc.getMsg()

			// Function to be tested
			res, err := s.Network.App.GetEVMKeeper().EthereumTx(s.Network.GetContext(), msg)

			events := s.Network.GetContext().EventManager().Events()
			if tc.expectedErr != nil {
				s.Require().Error(err)
				// no events should have been emitted
				s.Require().Empty(events)
			} else {
				s.Require().NoError(err)
				s.Require().False(res.Failed())

				// check expected events were emitted
				s.Require().NotEmpty(events)
				s.Require().True(utils.ContainsEventType(events.ToABCIEvents(), types.EventTypeEthereumTx))
				s.Require().True(utils.ContainsEventType(events.ToABCIEvents(), sdktypes.EventTypeMessage))
			}

			if tc.postCheck != nil {
				tc.postCheck()
			}

			err = s.Network.NextBlock()
			s.Require().NoError(err)
		})
	}
	s.EnableFeemarket = false
}

func (s *KeeperTestSuite) TestUpdateParams() {
	s.SetupTest()
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
		s.Run("MsgUpdateParams", func() {
			msg := tc.getMsg()
			_, err := s.Network.App.GetEVMKeeper().UpdateParams(s.Network.GetContext(), msg)
			if tc.expectedErr != nil {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.expectedErr.Error())
			} else {
				s.Require().NoError(err)
			}
		})

		err := s.Network.NextBlock()
		s.Require().NoError(err)
	}
}

func (s *KeeperTestSuite) TestRegisterPreinstalls() {
	s.SetupTest()
	testCases := []struct {
		name        string
		getMsg      func() *types.MsgRegisterPreinstalls
		expectedErr error
	}{
		{
			name: "fail - invalid authority",
			getMsg: func() *types.MsgRegisterPreinstalls {
				return &types.MsgRegisterPreinstalls{Authority: "foobar"}
			},
			expectedErr: govtypes.ErrInvalidSigner,
		},
		{
			name: "pass - valid Update msg",
			getMsg: func() *types.MsgRegisterPreinstalls {
				return &types.MsgRegisterPreinstalls{
					Authority: authtypes.NewModuleAddress(govtypes.ModuleName).String(),
					Preinstalls: []types.Preinstall{{
						Name:    "Test1",
						Address: "0xb364E75b1189DcbBF7f0C856456c1ba8e4d6481b",
						Code:    "0x000000000",
					}},
				}
			},
			expectedErr: nil,
		},
		{
			name: "fail - double registration",
			getMsg: func() *types.MsgRegisterPreinstalls {
				return &types.MsgRegisterPreinstalls{
					Authority:   authtypes.NewModuleAddress(govtypes.ModuleName).String(),
					Preinstalls: types.DefaultPreinstalls,
				}
			},
			expectedErr: types.ErrInvalidPreinstall,
		},
	}

	for _, tc := range testCases {
		s.Run("MsgRegisterPreinstalls_"+tc.name, func() {
			msg := tc.getMsg()
			_, err := s.Network.App.GetEVMKeeper().RegisterPreinstalls(s.Network.GetContext(), msg)
			if tc.expectedErr != nil {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.expectedErr.Error())
			} else {
				s.Require().NoError(err)
			}
		})

		err := s.Network.NextBlock()
		s.Require().NoError(err)
	}
}
