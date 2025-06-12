package keeper_test

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/mock"
	gomock "go.uber.org/mock/gomock"

	"github.com/cosmos/evm/testutil/integration/common/factory"
	testutils "github.com/cosmos/evm/testutil/integration/os/utils"
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

func (suite *KeeperTestSuite) TestConvertERC20NativeERC20() {
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
				err := testutils.UpdateERC20Params(
					testutils.UpdateParamsInput{
						Tf:      suite.factory,
						Network: suite.network,
						Pk:      suite.keyring.GetPrivKey(0),
						Params:  params,
					},
				)
				suite.Require().NoError(err)
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
			"fail - delayed malicious contract",
			10,
			10,
			func(common.Address) {},
			func() {},
			contractMaliciousDelayed,
			false,
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
				suite.network.App.Erc20Keeper = keeper.NewKeeper(
					suite.network.App.GetKey("erc20"), suite.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), suite.network.App.AccountKeeper,
					suite.network.App.BankKeeper, mockEVMKeeper, suite.network.App.StakingKeeper,
					&suite.network.App.TransferKeeper,
				)

				existingAcc := &statedb.Account{Nonce: uint64(1), Balance: uint256.NewInt(1)}
				balance := make([]uint8, 32)
				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil).Once()
				mockEVMKeeper.On("CallEVMWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return(nil, fmt.Errorf("forced ApplyMessage error"))
				mockEVMKeeper.On("GetAccountWithoutBalance", mock.Anything, mock.Anything).Return(existingAcc, nil)
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
				suite.network.App.Erc20Keeper = keeper.NewKeeper(
					suite.network.App.GetKey("erc20"), suite.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), suite.network.App.AccountKeeper,
					suite.network.App.BankKeeper, mockEVMKeeper, suite.network.App.StakingKeeper,
					&suite.network.App.TransferKeeper,
				)

				existingAcc := &statedb.Account{Nonce: uint64(1), Balance: uint256.NewInt(1)}
				balance := make([]uint8, 32)
				balance[31] = uint8(1)
				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil).Twice()
				mockEVMKeeper.On("CallEVMWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("forced balance error"))
				mockEVMKeeper.On("GetAccountWithoutBalance", mock.Anything, mock.Anything).Return(existingAcc, nil)
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
				suite.network.App.Erc20Keeper = keeper.NewKeeper(
					suite.network.App.GetKey("erc20"), suite.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), suite.network.App.AccountKeeper,
					suite.network.App.BankKeeper, mockEVMKeeper, suite.network.App.StakingKeeper,
					&suite.network.App.TransferKeeper,
				)

				existingAcc := &statedb.Account{Nonce: uint64(1), Balance: uint256.NewInt(1)}
				balance := make([]uint8, 32)
				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil).Once()
				mockEVMKeeper.On("CallEVMWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{}, nil)
				mockEVMKeeper.On("GetAccountWithoutBalance", mock.Anything, mock.Anything).Return(existingAcc, nil)
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
				suite.network.App.Erc20Keeper = keeper.NewKeeper(
					suite.network.App.GetKey("erc20"), suite.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), suite.network.App.AccountKeeper,
					suite.network.App.BankKeeper, mockEVMKeeper, suite.network.App.StakingKeeper,
					&suite.network.App.TransferKeeper,
				)

				existingAcc := &statedb.Account{Nonce: uint64(1), Balance: uint256.NewInt(1)}
				balance := make([]uint8, 32)
				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil).Once()
				mockEVMKeeper.On("CallEVMWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil)
				mockEVMKeeper.On("GetAccountWithoutBalance", mock.Anything, mock.Anything).Return(existingAcc, nil)
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
				ctrl := gomock.NewController(suite.T())
				mockBankKeeper := erc20mocks.NewMockBankKeeper(ctrl)

				suite.network.App.Erc20Keeper = keeper.NewKeeper(
					suite.network.App.GetKey("erc20"), suite.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), suite.network.App.AccountKeeper,
					mockBankKeeper, suite.network.App.EVMKeeper, suite.network.App.StakingKeeper,
					&suite.network.App.TransferKeeper,
				)

				mockBankKeeper.EXPECT().MintCoins(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to mint")).AnyTimes()
				mockBankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to unescrow")).AnyTimes()
				mockBankKeeper.EXPECT().BlockedAddr(gomock.Any()).Return(false).AnyTimes()
				mockBankKeeper.EXPECT().GetBalance(gomock.Any(), gomock.Any(), gomock.Any()).Return(sdk.Coin{Denom: "coin", Amount: math.OneInt()}).AnyTimes()
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
				ctrl := gomock.NewController(suite.T())
				mockBankKeeper := erc20mocks.NewMockBankKeeper(ctrl)

				suite.network.App.Erc20Keeper = keeper.NewKeeper(
					suite.network.App.GetKey("erc20"), suite.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), suite.network.App.AccountKeeper,
					mockBankKeeper, suite.network.App.EVMKeeper, suite.network.App.StakingKeeper,
					&suite.network.App.TransferKeeper,
				)

				mockBankKeeper.EXPECT().MintCoins(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockBankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to unescrow"))
				mockBankKeeper.EXPECT().BlockedAddr(gomock.Any()).Return(false)
				mockBankKeeper.EXPECT().GetBalance(gomock.Any(), gomock.Any(), gomock.Any()).Return(sdk.Coin{Denom: "coin", Amount: math.OneInt()})
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
				ctrl := gomock.NewController(suite.T())
				mockBankKeeper := erc20mocks.NewMockBankKeeper(ctrl)

				suite.network.App.Erc20Keeper = keeper.NewKeeper(
					suite.network.App.GetKey("erc20"), suite.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), suite.network.App.AccountKeeper,
					mockBankKeeper, suite.network.App.EVMKeeper, suite.network.App.StakingKeeper,
					&suite.network.App.TransferKeeper,
				)

				mockBankKeeper.EXPECT().MintCoins(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockBankKeeper.EXPECT().SendCoinsFromModuleToAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockBankKeeper.EXPECT().BlockedAddr(gomock.Any()).Return(false)
				mockBankKeeper.EXPECT().GetBalance(gomock.Any(), gomock.Any(), gomock.Any()).Return(sdk.Coin{Denom: coinName, Amount: math.OneInt()}).AnyTimes()
			},
			contractMinterBurner,
			false,
			false,
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			var err error
			suite.mintFeeCollector = true
			defer func() {
				suite.mintFeeCollector = false
			}()

			suite.SetupTest()

			contractAddr, err = suite.setupRegisterERC20Pair(tc.contractType)
			suite.Require().NoError(err)

			tc.malleate(contractAddr)
			suite.Require().NotNil(contractAddr)

			coinName = types.CreateDenom(contractAddr.String())
			sender := suite.keyring.GetAccAddr(0)

			_, err = suite.MintERC20Token(contractAddr, suite.keyring.GetAddr(0), big.NewInt(tc.mint))
			suite.Require().NoError(err)
			// update context with latest committed changes

			tc.extra()

			convertERC20Msg := types.NewMsgConvertERC20(
				math.NewInt(tc.transfer),
				sender,
				contractAddr,
				suite.keyring.GetAddr(0),
			)

			ctx := suite.network.GetContext()

			if tc.expPass {
				_, err = suite.factory.CommitCosmosTx(suite.keyring.GetPrivKey(0), factory.CosmosTxArgs{Msgs: []sdk.Msg{convertERC20Msg}})
				suite.Require().NoError(err, tc.name)

				cosmosBalance := suite.network.App.BankKeeper.GetBalance(ctx, sender, coinName)

				acc := suite.network.App.EVMKeeper.GetAccountWithoutBalance(ctx, contractAddr)
				if tc.selfdestructed {
					suite.Require().Nil(acc, "expected contract to be destroyed")
				} else {
					suite.Require().NotNil(acc)
				}

				if tc.selfdestructed || !acc.IsContract() {
					id := suite.network.App.Erc20Keeper.GetTokenPairID(ctx, contractAddr.String())
					_, found := suite.network.App.Erc20Keeper.GetTokenPair(ctx, id)
					suite.Require().False(found)
				} else {
					suite.Require().Equal(cosmosBalance.Amount, math.NewInt(tc.transfer))
				}
			} else {
				_, err = suite.network.App.Erc20Keeper.ConvertERC20(ctx, convertERC20Msg)
				suite.Require().Error(err, tc.name)
			}
		})
	}
	suite.mintFeeCollector = false
}

func (suite *KeeperTestSuite) TestConvertNativeERC20ToEVMERC20() {
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
				suite.network.App.Erc20Keeper = keeper.NewKeeper(
					suite.network.App.GetKey("erc20"), suite.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), suite.network.App.AccountKeeper,
					suite.network.App.BankKeeper, mockEVMKeeper, suite.network.App.StakingKeeper, &suite.network.App.TransferKeeper,
				)

				existingAcc := &statedb.Account{Nonce: uint64(1), Balance: uint256.NewInt(1)}
				balance := make([]uint8, 32)
				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, fmt.Errorf("forced ApplyMessage error")).Once()
				mockEVMKeeper.On("CallEVMWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return(nil, fmt.Errorf("forced ApplyMessage error"))
				mockEVMKeeper.On("GetAccountWithoutBalance", mock.Anything, mock.Anything).Return(existingAcc, nil)
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
				suite.network.App.Erc20Keeper = keeper.NewKeeper(
					suite.network.App.GetKey("erc20"), suite.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), suite.network.App.AccountKeeper,
					suite.network.App.BankKeeper, mockEVMKeeper, suite.network.App.StakingKeeper,
					&suite.network.App.TransferKeeper,
				)

				existingAcc := &statedb.Account{Nonce: uint64(1), Balance: uint256.NewInt(1)}
				balance := make([]uint8, 32)
				balance[31] = uint8(1)
				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil).Times(3)
				mockEVMKeeper.On("CallEVMWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("forced balance error"))
				mockEVMKeeper.On("GetAccountWithoutBalance", mock.Anything, mock.Anything).Return(existingAcc, nil)
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
				suite.network.App.Erc20Keeper = keeper.NewKeeper(
					suite.network.App.GetKey("erc20"), suite.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), suite.network.App.AccountKeeper,
					suite.network.App.BankKeeper, mockEVMKeeper, suite.network.App.StakingKeeper,
					&suite.network.App.TransferKeeper,
				)

				existingAcc := &statedb.Account{Nonce: uint64(1), Balance: uint256.NewInt(1)}
				balance := make([]uint8, 32)
				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil).Twice()
				mockEVMKeeper.On("CallEVMWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{}, nil)
				mockEVMKeeper.On("GetAccountWithoutBalance", mock.Anything, mock.Anything).Return(existingAcc, nil)
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
				suite.network.App.Erc20Keeper = keeper.NewKeeper(
					suite.network.App.GetKey("erc20"), suite.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), suite.network.App.AccountKeeper,
					suite.network.App.BankKeeper, mockEVMKeeper, suite.network.App.StakingKeeper,
					&suite.network.App.TransferKeeper,
				)

				existingAcc := &statedb.Account{Nonce: uint64(1), Balance: uint256.NewInt(1)}
				balance := make([]uint8, 32)
				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil).Twice()
				mockEVMKeeper.On("CallEVMWithData", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil)
				mockEVMKeeper.On("GetAccountWithoutBalance", mock.Anything, mock.Anything).Return(existingAcc, nil)
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
				ctrl := gomock.NewController(suite.T())
				mockBankKeeper := erc20mocks.NewMockBankKeeper(ctrl)

				suite.network.App.Erc20Keeper = keeper.NewKeeper(
					suite.network.App.GetKey("erc20"), suite.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), suite.network.App.AccountKeeper,
					mockBankKeeper, suite.network.App.EVMKeeper, suite.network.App.StakingKeeper,
					&suite.network.App.TransferKeeper,
				)

				mockBankKeeper.EXPECT().MintCoins(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to mint")).AnyTimes()
				mockBankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to unescrow")).AnyTimes()
				mockBankKeeper.EXPECT().BlockedAddr(gomock.Any()).Return(false).AnyTimes()
				mockBankKeeper.EXPECT().GetBalance(gomock.Any(), gomock.Any(), gomock.Any()).Return(sdk.Coin{Denom: "coin", Amount: math.OneInt()}).AnyTimes()
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
				ctrl := gomock.NewController(suite.T())
				mockBankKeeper := erc20mocks.NewMockBankKeeper(ctrl)

				suite.network.App.Erc20Keeper = keeper.NewKeeper(
					suite.network.App.GetKey("erc20"), suite.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), suite.network.App.AccountKeeper,
					mockBankKeeper, suite.network.App.EVMKeeper, suite.network.App.StakingKeeper,
					&suite.network.App.TransferKeeper,
				)

				mockBankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockBankKeeper.EXPECT().BurnCoins(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed to burn")).AnyTimes()
				mockBankKeeper.EXPECT().BlockedAddr(gomock.Any()).Return(false)
			},
			contractMinterBurner,
			false,
			false,
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			var err error
			suite.mintFeeCollector = true
			defer func() {
				suite.mintFeeCollector = false
			}()
			suite.SetupTest()

			contractAddr, err = suite.setupRegisterERC20Pair(tc.contractType)
			suite.Require().NoError(err)

			tc.malleate(contractAddr)
			suite.Require().NotNil(contractAddr)
			// update context with latest committed changes
			sender := suite.keyring.GetAccAddr(0)
			senderHex := suite.keyring.GetAddr(0)

			// mint tokens to sender
			_, err = suite.MintERC20Token(contractAddr, senderHex, big.NewInt(tc.mint))
			suite.Require().NoError(err)

			// convert tokens to native first
			convertERC20Msg := types.NewMsgConvertERC20(
				math.NewInt(tc.mint),
				sender,
				contractAddr,
				senderHex,
			)
			_, err = suite.factory.CommitCosmosTx(suite.keyring.GetPrivKey(0), factory.CosmosTxArgs{Msgs: []sdk.Msg{convertERC20Msg}})
			suite.Require().NoError(err)

			tc.extra()

			coinName = types.CreateDenom(contractAddr.String())

			evmTokenBalanceBefore, err := suite.BalanceOf(contractAddr, senderHex) // actual: 100, expected: 0
			suite.Require().NoError(err)
			suite.Require().Equal(big.NewInt(0).Int64(), evmTokenBalanceBefore.(*big.Int).Int64())

			// then convert native tokens back into EVM tokens
			convertNativeMsg := types.NewMsgConvertCoin(sdk.Coin{Denom: coinName, Amount: math.NewInt(tc.transfer)}, senderHex, sender)

			if tc.expPass {
				_, err = suite.factory.CommitCosmosTx(suite.keyring.GetPrivKey(0), factory.CosmosTxArgs{Msgs: []sdk.Msg{convertNativeMsg}})
				suite.Require().NoError(err, tc.name)
				cosmosBalance := suite.network.App.BankKeeper.GetBalance(suite.network.GetContext(), sender, coinName)
				evmTokenBalanceAfter, err := suite.BalanceOf(contractAddr, senderHex)
				suite.Require().NoError(err)

				acc := suite.network.App.EVMKeeper.GetAccountWithoutBalance(suite.network.GetContext(), contractAddr)
				if tc.selfdestructed {
					suite.Require().Nil(acc, "expected contract to be destroyed")
				} else {
					suite.Require().NotNil(acc)
				}

				if tc.selfdestructed || !acc.IsContract() {
					id := suite.network.App.Erc20Keeper.GetTokenPairID(suite.network.GetContext(), contractAddr.String())
					_, found := suite.network.App.Erc20Keeper.GetTokenPair(suite.network.GetContext(), id)
					suite.Require().False(found)
				} else {
					suite.Require().Equal(cosmosBalance.Amount, math.NewInt(tc.mint-tc.transfer))
					suite.Require().Equal(evmTokenBalanceAfter.(*big.Int).Int64(), math.NewInt(tc.transfer).Int64())
				}
			} else {
				_, err = suite.network.App.Erc20Keeper.ConvertCoin(suite.network.GetContext(), convertNativeMsg)
				suite.Require().Error(err, tc.name)
			}
		})
	}
	suite.mintFeeCollector = false
}

func (suite *KeeperTestSuite) TestUpdateParams() {
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
		suite.Run("MsgUpdateParams", func() {
			suite.SetupTest()
			_, err := suite.network.App.Erc20Keeper.UpdateParams(suite.network.GetContext(), tc.request)
			if tc.expectErr {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}
