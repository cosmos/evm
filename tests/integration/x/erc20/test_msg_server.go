package erc20

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

	"github.com/cosmos/evm/testutil/integration/base/factory"
	"github.com/cosmos/evm/testutil/integration/evm/utils"
	"github.com/cosmos/evm/x/erc20/keeper"
	"github.com/cosmos/evm/x/erc20/types"
	erc20mocks "github.com/cosmos/evm/x/erc20/types/mocks"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

func (s *KeeperTestSuite) TestConvertERC20NativeERC20() {
	var (
		contractAddr common.Address
		coinName     string
	)
	testCases := []struct {
		name           string
		mint           int64
		transfer       int64
		malleate       func(common.Address)
		extra          func()
		contractType   int
		expPass        bool
		selfdestructed bool
	}{
		{
			"ok - sufficient funds",
			100,
			10,
			func(common.Address) {},
			func() {},
			contractMinterBurner,
			true,
			false,
		},
		{
			"ok - equal funds",
			10,
			10,
			func(common.Address) {},
			func() {},
			contractMinterBurner,
			true,
			false,
		},
		{
			"fail - insufficient funds - callEVM",
			0,
			10,
			func(common.Address) {},
			func() {},
			contractMinterBurner,
			false,
			false,
		},
		{
			"fail - minting disabled",
			100,
			10,
			func(common.Address) {
				params := types.DefaultParams()
				params.EnableErc20 = false
				err := utils.UpdateERC20Params(
					utils.UpdateParamsInput{
						Tf:      s.factory,
						Network: s.network,
						Pk:      s.keyring.GetPrivKey(0),
						Params:  params,
					},
				)
				s.Require().NoError(err)
			},
			func() {},
			contractMinterBurner,
			false,
			false,
		},
		{
			"fail - direct balance manipulation contract",
			100,
			10,
			func(common.Address) {},
			func() {},
			contractDirectBalanceManipulation,
			false,
			false,
		},
		{
			"pass - delayed malicious contract",
			10,
			10,
			func(common.Address) {},
			func() {},
			contractMaliciousDelayed,
			true,
			false,
		},
		{
			"fail - negative transfer contract",
			10,
			-10,
			func(common.Address) {},
			func() {},
			contractMinterBurner,
			false,
			false,
		},
		{
			"fail - force evm fail",
			100,
			10,
			func(common.Address) {},
			func() {
				mockEVMKeeper := &erc20mocks.EVMKeeper{}
				transferKeeper := s.network.App.GetTransferKeeper()
				erc20Keeper := keeper.NewKeeper(
					s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), s.network.App.GetAccountKeeper(),
					s.network.App.GetBankKeeper(), mockEVMKeeper, s.network.App.GetStakingKeeper(),
					&transferKeeper,
				)
				s.network.App.SetErc20Keeper(erc20Keeper)

				existingAcc := &statedb.Account{Nonce: uint64(1), Balance: uint256.NewInt(1)}
				balance := make([]uint8, 32)
				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil).Once()
				mockEVMKeeper.On("CallEVMWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return(nil, fmt.Errorf("forced ApplyMessage error"))
				mockEVMKeeper.On("GetAccountWithoutBalance", mock.Anything, mock.Anything).Return(existingAcc, nil)
				mockEVMKeeper.On("IsContract", mock.Anything, mock.Anything).Return(true)
			},
			contractMinterBurner,
			false,
			false,
		},
		{
			"fail - force get balance fail",
			100,
			10,
			func(common.Address) {},
			func() {
				mockEVMKeeper := &erc20mocks.EVMKeeper{}
				transferKeeper := s.network.App.GetTransferKeeper()
				erc20Keeper := keeper.NewKeeper(
					s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), s.network.App.GetAccountKeeper(),
					s.network.App.GetBankKeeper(), mockEVMKeeper, s.network.App.GetStakingKeeper(),
					&transferKeeper,
				)
				s.network.App.SetErc20Keeper(erc20Keeper)

				existingAcc := &statedb.Account{Nonce: uint64(1), Balance: uint256.NewInt(1)}
				balance := make([]uint8, 32)
				balance[31] = uint8(1)
				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil).Twice()
				mockEVMKeeper.On("CallEVMWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("forced balance error"))
				mockEVMKeeper.On("GetAccountWithoutBalance", mock.Anything, mock.Anything).Return(existingAcc, nil)
				mockEVMKeeper.On("IsContract", mock.Anything, mock.Anything).Return(true)
			},
			contractMinterBurner,
			false,
			false,
		},
		{
			"fail - force transfer unpack fail",
			100,
			10,
			func(common.Address) {},
			func() {
				mockEVMKeeper := &erc20mocks.EVMKeeper{}
				transferKeeper := s.network.App.GetTransferKeeper()
				erc20Keeper := keeper.NewKeeper(
					s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), s.network.App.GetAccountKeeper(),
					s.network.App.GetBankKeeper(), mockEVMKeeper, s.network.App.GetStakingKeeper(),
					&transferKeeper,
				)
				s.network.App.SetErc20Keeper(erc20Keeper)

				existingAcc := &statedb.Account{Nonce: uint64(1), Balance: uint256.NewInt(1)}
				balance := make([]uint8, 32)
				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil).Once()
				mockEVMKeeper.On("CallEVMWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{}, nil)
				mockEVMKeeper.On("GetAccountWithoutBalance", mock.Anything, mock.Anything).Return(existingAcc, nil)
				mockEVMKeeper.On("IsContract", mock.Anything, mock.Anything).Return(true)
			},
			contractMinterBurner,
			false,
			false,
		},

		{
			"fail - force invalid transfer fail",
			100,
			10,
			func(common.Address) {},
			func() {
				mockEVMKeeper := &erc20mocks.EVMKeeper{}
				transferKeeper := s.network.App.GetTransferKeeper()
				erc20Keeper := keeper.NewKeeper(
					s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), s.network.App.GetAccountKeeper(),
					s.network.App.GetBankKeeper(), mockEVMKeeper, s.network.App.GetStakingKeeper(),
					&transferKeeper,
				)
				s.network.App.SetErc20Keeper(erc20Keeper)

				existingAcc := &statedb.Account{Nonce: uint64(1), Balance: uint256.NewInt(1)}
				balance := make([]uint8, 32)
				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil).Once()
				mockEVMKeeper.On("CallEVMWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil)
				mockEVMKeeper.On("GetAccountWithoutBalance", mock.Anything, mock.Anything).Return(existingAcc, nil)
				mockEVMKeeper.On("IsContract", mock.Anything, mock.Anything).Return(true)
			},
			contractMinterBurner,
			false,
			false,
		},
		{
			"fail - force mint fail",
			100,
			10,
			func(common.Address) {},
			func() {
				ctrl := gomock.NewController(s.T())
				mockBankKeeper := erc20mocks.NewMockBankKeeper(ctrl)
				transferKeeper := s.network.App.GetTransferKeeper()
				erc20Keeper := keeper.NewKeeper(
					s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), s.network.App.GetAccountKeeper(),
					mockBankKeeper, s.network.App.GetEVMKeeper(), s.network.App.GetStakingKeeper(),
					&transferKeeper,
				)
				s.network.App.SetErc20Keeper(erc20Keeper)

				mockBankKeeper.EXPECT().MintCoins(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to mint")).AnyTimes()
				mockBankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to unescrow")).AnyTimes()
				mockBankKeeper.EXPECT().BlockedAddr(gomock.Any()).Return(false).AnyTimes()
				mockBankKeeper.EXPECT().GetBalance(gomock.Any(), gomock.Any(), gomock.Any()).Return(sdk.Coin{Denom: "coin", Amount: math.OneInt()}).AnyTimes()
				mockBankKeeper.EXPECT().IsSendEnabledCoin(gomock.Any(), gomock.Any()).Return(true).AnyTimes()
			},
			contractMinterBurner,
			false,
			false,
		},
		{
			"fail - force send minted fail",
			100,
			10,
			func(common.Address) {},
			func() {
				ctrl := gomock.NewController(s.T())
				mockBankKeeper := erc20mocks.NewMockBankKeeper(ctrl)
				transferKeeper := s.network.App.GetTransferKeeper()
				erc20Keeper := keeper.NewKeeper(
					s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), s.network.App.GetAccountKeeper(),
					mockBankKeeper, s.network.App.GetEVMKeeper(), s.network.App.GetStakingKeeper(),
					&transferKeeper,
				)
				s.network.App.SetErc20Keeper(erc20Keeper)

				mockBankKeeper.EXPECT().MintCoins(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockBankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to unescrow"))
				mockBankKeeper.EXPECT().BlockedAddr(gomock.Any()).Return(false)
				mockBankKeeper.EXPECT().GetBalance(gomock.Any(), gomock.Any(), gomock.Any()).Return(sdk.Coin{Denom: "coin", Amount: math.OneInt()})
				mockBankKeeper.EXPECT().IsSendEnabledCoin(gomock.Any(), gomock.Any()).Return(true).AnyTimes()
			},
			contractMinterBurner,
			false,
			false,
		},
		{
			"fail - force bank balance fail",
			100,
			10,
			func(common.Address) {},
			func() {
				ctrl := gomock.NewController(s.T())
				mockBankKeeper := erc20mocks.NewMockBankKeeper(ctrl)
				transferKeeper := s.network.App.GetTransferKeeper()
				erc20Keeper := keeper.NewKeeper(
					s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), s.network.App.GetAccountKeeper(),
					mockBankKeeper, s.network.App.GetEVMKeeper(), s.network.App.GetStakingKeeper(),
					&transferKeeper,
				)
				s.network.App.SetErc20Keeper(erc20Keeper)

				mockBankKeeper.EXPECT().MintCoins(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockBankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockBankKeeper.EXPECT().BlockedAddr(gomock.Any()).Return(false)
				mockBankKeeper.EXPECT().GetBalance(gomock.Any(), gomock.Any(), gomock.Any()).Return(sdk.Coin{Denom: coinName, Amount: math.OneInt()}).AnyTimes()
				mockBankKeeper.EXPECT().IsSendEnabledCoin(gomock.Any(), gomock.Any()).Return(true).AnyTimes()
			},
			contractMinterBurner,
			false,
			false,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			var err error
			s.mintFeeCollector = true
			defer func() {
				s.mintFeeCollector = false
			}()

			s.SetupTest()

			contractAddr, err = s.setupRegisterERC20Pair(tc.contractType)
			s.Require().NoError(err)

			tc.malleate(contractAddr)
			s.Require().NotNil(contractAddr)

			coinName = types.CreateDenom(contractAddr.String())
			sender := s.keyring.GetAccAddr(0)

			_, err = s.MintERC20Token(contractAddr, s.keyring.GetAddr(0), big.NewInt(tc.mint))
			s.Require().NoError(err)
			// update context with latest committed changes

			tc.extra()

			convertERC20Msg := types.NewMsgConvertERC20(
				math.NewInt(tc.transfer),
				sender,
				contractAddr,
				s.keyring.GetAddr(0),
			)

			ctx := s.network.GetContext()

			if tc.expPass {
				_, err = s.factory.CommitCosmosTx(s.keyring.GetPrivKey(0), factory.CosmosTxArgs{Msgs: []sdk.Msg{convertERC20Msg}})
				s.Require().NoError(err, tc.name)

				cosmosBalance := s.network.App.GetBankKeeper().GetBalance(ctx, sender, coinName)

				acc := s.network.App.GetEVMKeeper().GetAccountWithoutBalance(ctx, contractAddr)
				if tc.selfdestructed {
					s.Require().Nil(acc, "expected contract to be destroyed")
				} else {
					s.Require().NotNil(acc)
				}

				isContract := s.network.App.GetEVMKeeper().IsContract(s.network.GetContext(), contractAddr)
				if tc.selfdestructed || !isContract {
					id := s.network.App.GetErc20Keeper().GetTokenPairID(ctx, contractAddr.String())
					_, found := s.network.App.GetErc20Keeper().GetTokenPair(ctx, id)
					s.Require().False(found)
				} else {
					s.Require().Equal(cosmosBalance.Amount, math.NewInt(tc.transfer))
				}
			} else {
				_, err = s.network.App.GetErc20Keeper().ConvertERC20(ctx, convertERC20Msg)
				s.Require().Error(err, tc.name)
			}
		})
	}
	s.mintFeeCollector = false
}

func (s *KeeperTestSuite) TestConvertNativeERC20ToEVMERC20() {
	var (
		contractAddr common.Address
		coinName     string
	)
	testCases := []struct {
		name           string
		mint           int64
		transfer       int64
		malleate       func(common.Address)
		extra          func()
		contractType   int
		expPass        bool
		selfdestructed bool
	}{
		{
			"ok - sufficient funds",
			100,
			10,
			func(common.Address) {},
			func() {},
			contractMinterBurner,
			true,
			false,
		},
		{
			"ok - equal funds",
			10,
			10,
			func(common.Address) {},
			func() {},
			contractMinterBurner,
			true,
			false,
		},
		{
			"fail - negative transfer of coins",
			10,
			-10,
			func(common.Address) {},
			func() {},
			contractMinterBurner,
			false,
			false,
		},
		{
			"fail - force evm fail",
			100,
			10,
			func(common.Address) {},
			func() {
				mockEVMKeeper := &erc20mocks.EVMKeeper{}
				transferKeeper := s.network.App.GetTransferKeeper()
				erc20Keeper := keeper.NewKeeper(
					s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), s.network.App.GetAccountKeeper(),
					s.network.App.GetBankKeeper(), mockEVMKeeper, s.network.App.GetStakingKeeper(),
					&transferKeeper,
				)
				s.network.App.SetErc20Keeper(erc20Keeper)

				existingAcc := &statedb.Account{Nonce: uint64(1), Balance: uint256.NewInt(1)}
				balance := make([]uint8, 32)
				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, fmt.Errorf("forced ApplyMessage error")).Once()
				mockEVMKeeper.On("CallEVMWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return(nil, fmt.Errorf("forced ApplyMessage error"))
				mockEVMKeeper.On("GetAccountWithoutBalance", mock.Anything, mock.Anything).Return(existingAcc, nil)
				mockEVMKeeper.On("IsContract", mock.Anything, mock.Anything).Return(true)
			},
			contractMinterBurner,
			false,
			false,
		},
		{
			"fail - force get balance fail",
			100,
			10,
			func(common.Address) {},
			func() {
				mockEVMKeeper := &erc20mocks.EVMKeeper{}
				transferKeeper := s.network.App.GetTransferKeeper()
				erc20Keeper := keeper.NewKeeper(
					s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), s.network.App.GetAccountKeeper(),
					s.network.App.GetBankKeeper(), mockEVMKeeper, s.network.App.GetStakingKeeper(),
					&transferKeeper,
				)
				s.network.App.SetErc20Keeper(erc20Keeper)

				existingAcc := &statedb.Account{Nonce: uint64(1), Balance: uint256.NewInt(1)}
				balance := make([]uint8, 32)
				balance[31] = uint8(1)
				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil).Times(3)
				mockEVMKeeper.On("CallEVMWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("forced balance error"))
				mockEVMKeeper.On("GetAccountWithoutBalance", mock.Anything, mock.Anything).Return(existingAcc, nil)
				mockEVMKeeper.On("IsContract", mock.Anything, mock.Anything).Return(true)
			},
			contractMinterBurner,
			false,
			false,
		},
		{
			"fail - force transfer unpack fail",
			100,
			10,
			func(common.Address) {},
			func() {
				mockEVMKeeper := &erc20mocks.EVMKeeper{}
				transferKeeper := s.network.App.GetTransferKeeper()
				erc20Keeper := keeper.NewKeeper(
					s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), s.network.App.GetAccountKeeper(),
					s.network.App.GetBankKeeper(), mockEVMKeeper, s.network.App.GetStakingKeeper(),
					&transferKeeper,
				)
				s.network.App.SetErc20Keeper(erc20Keeper)

				existingAcc := &statedb.Account{Nonce: uint64(1), Balance: uint256.NewInt(1)}
				balance := make([]uint8, 32)
				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil).Twice()
				mockEVMKeeper.On("CallEVMWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{}, nil)
				mockEVMKeeper.On("GetAccountWithoutBalance", mock.Anything, mock.Anything).Return(existingAcc, nil)
				mockEVMKeeper.On("IsContract", mock.Anything, mock.Anything).Return(true)
			},
			contractMinterBurner,
			false,
			false,
		},

		{
			"fail - force invalid transfer fail",
			100,
			10,
			func(common.Address) {},
			func() {
				mockEVMKeeper := &erc20mocks.EVMKeeper{}
				transferKeeper := s.network.App.GetTransferKeeper()
				erc20Keeper := keeper.NewKeeper(
					s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), s.network.App.GetAccountKeeper(),
					s.network.App.GetBankKeeper(), mockEVMKeeper, s.network.App.GetStakingKeeper(),
					&transferKeeper,
				)
				s.network.App.SetErc20Keeper(erc20Keeper)

				existingAcc := &statedb.Account{Nonce: uint64(1), Balance: uint256.NewInt(1)}
				balance := make([]uint8, 32)
				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil).Twice()
				mockEVMKeeper.On("CallEVMWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil)
				mockEVMKeeper.On("GetAccountWithoutBalance", mock.Anything, mock.Anything).Return(existingAcc, nil)
				mockEVMKeeper.On("IsContract", mock.Anything, mock.Anything).Return(true)
			},
			contractMinterBurner,
			false,
			false,
		},
		{
			"fail - force send fail",
			100,
			10,
			func(common.Address) {},
			func() {
				ctrl := gomock.NewController(s.T())
				mockBankKeeper := erc20mocks.NewMockBankKeeper(ctrl)
				transferKeeper := s.network.App.GetTransferKeeper()
				erc20Keeper := keeper.NewKeeper(
					s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), s.network.App.GetAccountKeeper(),
					mockBankKeeper, s.network.App.GetEVMKeeper(), s.network.App.GetStakingKeeper(),
					&transferKeeper,
				)
				s.network.App.SetErc20Keeper(erc20Keeper)

				mockBankKeeper.EXPECT().MintCoins(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to mint")).AnyTimes()
				mockBankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to unescrow")).AnyTimes()
				mockBankKeeper.EXPECT().BlockedAddr(gomock.Any()).Return(false).AnyTimes()
				mockBankKeeper.EXPECT().GetBalance(gomock.Any(), gomock.Any(), gomock.Any()).Return(sdk.Coin{Denom: "coin", Amount: math.OneInt()}).AnyTimes()
				mockBankKeeper.EXPECT().IsSendEnabledCoin(gomock.Any(), gomock.Any()).Return(true).AnyTimes()
			},
			contractMinterBurner,
			false,
			false,
		},
		{
			"fail - burn coins fail",
			100,
			10,
			func(common.Address) {},
			func() {
				ctrl := gomock.NewController(s.T())
				mockBankKeeper := erc20mocks.NewMockBankKeeper(ctrl)
				transferKeeper := s.network.App.GetTransferKeeper()
				erc20Keeper := keeper.NewKeeper(
					s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), s.network.App.GetAccountKeeper(),
					mockBankKeeper, s.network.App.GetEVMKeeper(), s.network.App.GetStakingKeeper(),
					&transferKeeper,
				)
				s.network.App.SetErc20Keeper(erc20Keeper)

				mockBankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockBankKeeper.EXPECT().BurnCoins(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to burn")).AnyTimes()
				mockBankKeeper.EXPECT().BlockedAddr(gomock.Any()).Return(false)
				mockBankKeeper.EXPECT().IsSendEnabledCoin(gomock.Any(), gomock.Any()).Return(true).AnyTimes()
			},
			contractMinterBurner,
			false,
			false,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			var err error
			s.mintFeeCollector = true
			defer func() {
				s.mintFeeCollector = false
			}()
			s.SetupTest()

			contractAddr, err = s.setupRegisterERC20Pair(tc.contractType)
			s.Require().NoError(err)

			tc.malleate(contractAddr)
			s.Require().NotNil(contractAddr)
			// update context with latest committed changes
			sender := s.keyring.GetAccAddr(0)
			senderHex := s.keyring.GetAddr(0)

			// mint tokens to sender
			_, err = s.MintERC20Token(contractAddr, senderHex, big.NewInt(tc.mint))
			s.Require().NoError(err)

			// convert tokens to native first
			convertERC20Msg := types.NewMsgConvertERC20(
				math.NewInt(tc.mint),
				sender,
				contractAddr,
				senderHex,
			)
			_, err = s.factory.CommitCosmosTx(s.keyring.GetPrivKey(0), factory.CosmosTxArgs{Msgs: []sdk.Msg{convertERC20Msg}})
			s.Require().NoError(err)

			tc.extra()

			coinName = types.CreateDenom(contractAddr.String())

			evmTokenBalanceBefore, err := s.BalanceOf(contractAddr, senderHex) // actual: 100, expected: 0
			s.Require().NoError(err)
			s.Require().Equal(big.NewInt(0).Int64(), evmTokenBalanceBefore.(*big.Int).Int64())

			// then convert native tokens back into EVM tokens
			convertNativeMsg := types.NewMsgConvertCoin(sdk.Coin{Denom: coinName, Amount: math.NewInt(tc.transfer)}, senderHex, sender)

			if tc.expPass {
				_, err = s.factory.CommitCosmosTx(s.keyring.GetPrivKey(0), factory.CosmosTxArgs{Msgs: []sdk.Msg{convertNativeMsg}})
				s.Require().NoError(err, tc.name)
				cosmosBalance := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), sender, coinName)
				evmTokenBalanceAfter, err := s.BalanceOf(contractAddr, senderHex)
				s.Require().NoError(err)

				acc := s.network.App.GetEVMKeeper().GetAccountWithoutBalance(s.network.GetContext(), contractAddr)
				if tc.selfdestructed {
					s.Require().Nil(acc, "expected contract to be destroyed")
				} else {
					s.Require().NotNil(acc)
				}

				isContract := s.network.App.GetEVMKeeper().IsContract(s.network.GetContext(), contractAddr)
				if tc.selfdestructed || !isContract {
					id := s.network.App.GetErc20Keeper().GetTokenPairID(s.network.GetContext(), contractAddr.String())
					_, found := s.network.App.GetErc20Keeper().GetTokenPair(s.network.GetContext(), id)
					s.Require().False(found)
				} else {
					s.Require().Equal(cosmosBalance.Amount, math.NewInt(tc.mint-tc.transfer))
					s.Require().Equal(evmTokenBalanceAfter.(*big.Int).Int64(), math.NewInt(tc.transfer).Int64())
				}
			} else {
				_, err = s.network.App.GetErc20Keeper().ConvertCoin(s.network.GetContext(), convertNativeMsg)
				s.Require().Error(err, tc.name)
			}
		})
	}
	s.mintFeeCollector = false
}

func (s *KeeperTestSuite) TestUpdateParams() {
	testCases := []struct {
		name      string
		request   *types.MsgUpdateParams
		expectErr bool
	}{
		{
			name:      "fail - invalid authority",
			request:   &types.MsgUpdateParams{Authority: "foobar"},
			expectErr: true,
		},
		{
			name: "pass - valid Update msg",
			request: &types.MsgUpdateParams{
				Authority: authtypes.NewModuleAddress(govtypes.ModuleName).String(),
				Params:    types.DefaultParams(),
			},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		s.Run("MsgUpdateParams", func() {
			s.SetupTest()
			_, err := s.network.App.GetErc20Keeper().UpdateParams(s.network.GetContext(), tc.request)
			if tc.expectErr {
				s.Require().Error(err)
			} else {
				s.Require().NoError(err)
			}
		})
	}
}
