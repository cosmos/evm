package erc20

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/mock"

	"github.com/cosmos/evm/contracts"
	utiltx "github.com/cosmos/evm/testutil/tx"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	"github.com/cosmos/evm/x/erc20/keeper"
	"github.com/cosmos/evm/x/erc20/types"
	erc20mocks "github.com/cosmos/evm/x/erc20/types/mocks"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

func (s *KeeperTestSuite) TestQueryERC20() {
	var (
		contract common.Address
		ctx      sdk.Context
	)
	testCases := []struct {
		name     string
		malleate func()
		res      bool
	}{
		{
			"erc20 not deployed",
			func() { contract = common.Address{} },
			false,
		},
		{
			"ok",
			func() {
				var err error
				contract, err = s.factory.DeployContract(
					s.keyring.GetPrivKey(0),
					evmtypes.EvmTxArgs{},
					testutiltypes.ContractDeploymentData{
						Contract:        contracts.ERC20MinterBurnerDecimalsContract,
						ConstructorArgs: []interface{}{"coin", "token", erc20Decimals},
					},
				)
				s.Require().NoError(err)
				s.Require().NoError(s.network.NextBlock())
				ctx = s.network.GetContext()
			},
			true,
		},
	}
	for _, tc := range testCases {
		s.SetupTest() // reset
		ctx = s.network.GetContext()

		tc.malleate()

		res, err := s.network.App.GetErc20Keeper().QueryERC20(ctx, contract)
		if tc.res {
			s.Require().NoError(err)
			s.Require().Equal(
				types.ERC20Data{Name: "coin", Symbol: "token", Decimals: erc20Decimals},
				res,
			)
		} else {
			s.Require().Error(err)
		}
	}
}

func (s *KeeperTestSuite) TestBalanceOf() {
	var mockEVMKeeper *erc20mocks.EVMKeeper
	contract := utiltx.GenerateAddress()
	testCases := []struct {
		name       string
		malleate   func()
		expBalance int64
		res        bool
	}{
		{
			"Failed to call Evm",
			func() {
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("forced ApplyMessage error"))
			},
			int64(0),
			false,
		},
		{
			"Incorrect res",
			func() {
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: []uint8{0, 0}}, nil).Once()
			},
			int64(0),
			false,
		},
		{
			"Correct Execution",
			func() {
				balance := make([]uint8, 32)
				balance[31] = uint8(10)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: balance}, nil).Once()
			},
			int64(10),
			true,
		},
	}
	for _, tc := range testCases {
		s.SetupTest() // reset
		mockEVMKeeper = &erc20mocks.EVMKeeper{}
		transferKeeper := s.network.App.GetTransferKeeper()
		erc20Keeper := keeper.NewKeeper(
			s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
			authtypes.NewModuleAddress(govtypes.ModuleName),
			s.network.App.GetAccountKeeper(), s.network.App.GetBankKeeper(),
			mockEVMKeeper, s.network.App.GetStakingKeeper(),
			&transferKeeper,
		)
		s.network.App.SetErc20Keeper(erc20Keeper)

		tc.malleate()

		abi := contracts.ERC20MinterBurnerDecimalsContract.ABI
		balance := s.network.App.GetErc20Keeper().BalanceOf(s.network.GetContext(), abi, contract, utiltx.GenerateAddress())
		if tc.res {
			s.Require().Equal(balance.Int64(), tc.expBalance)
		} else {
			s.Require().Nil(balance)
		}
	}
}

func (s *KeeperTestSuite) TestQueryERC20ForceFail() {
	var mockEVMKeeper *erc20mocks.EVMKeeper
	contract := utiltx.GenerateAddress()
	testCases := []struct {
		name     string
		malleate func()
		res      bool
	}{
		{
			"Failed to call Evm",
			func() {
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("forced ApplyMessage error"))
			},
			false,
		},
		{
			"Incorrect res",
			func() {
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: []uint8{0, 0}}, nil).Once()
			},
			false,
		},
		{
			"Correct res for name - incorrect for symbol",
			func() {
				ret := []uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 32, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 67, 111, 105, 110, 32, 84, 111, 107, 101, 110, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
				mockEVMKeeper.On("ApplyMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: ret}, nil).Once()
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{VmError: "Error"}, nil).Once()
			},
			false,
		},
		{
			"incorrect symbol res",
			func() {
				ret := []uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 32, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 67, 111, 105, 110, 32, 84, 111, 107, 101, 110, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: ret}, nil).Once()
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: []uint8{0, 0}}, nil).Once()
			},
			false,
		},
		{
			"Correct res for name - incorrect for symbol",
			func() {
				ret := []uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 32, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 67, 111, 105, 110, 32, 84, 111, 107, 101, 110, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
				retSymbol := []uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 32, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 67, 84, 75, 78, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: ret}, nil).Once()
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: retSymbol}, nil).Once()
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{VmError: "Error"}, nil).Once()
			},
			false,
		},
		{
			"incorrect symbol res",
			func() {
				ret := []uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 32, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 67, 111, 105, 110, 32, 84, 111, 107, 101, 110, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
				retSymbol := []uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 32, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 67, 84, 75, 78, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: ret}, nil).Once()
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: retSymbol}, nil).Once()
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
					mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.MsgEthereumTxResponse{Ret: []uint8{0, 0}}, nil).Once()
			},
			false,
		},
	}
	for _, tc := range testCases {
		s.SetupTest() // reset

		// TODO: what's the reason we are using mockEVMKeeper here? Instead of just passing the s.app.EVMKeeper?
		mockEVMKeeper = &erc20mocks.EVMKeeper{}
		transferKeeper := s.network.App.GetTransferKeeper()
		erc20Keeper := keeper.NewKeeper(
			s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
			authtypes.NewModuleAddress(govtypes.ModuleName),
			s.network.App.GetAccountKeeper(), s.network.App.GetBankKeeper(),
			mockEVMKeeper, s.network.App.GetStakingKeeper(),
			&transferKeeper,
		)
		s.network.App.SetErc20Keeper(erc20Keeper)

		tc.malleate()

		res, err := s.network.App.GetErc20Keeper().QueryERC20(s.network.GetContext(), contract)
		if tc.res {
			s.Require().NoError(err)
			s.Require().Equal(
				types.ERC20Data{Name: "coin", Symbol: "token", Decimals: erc20Decimals},
				res,
			)
		} else {
			s.Require().Error(err)
		}
	}
}

func (s *KeeperTestSuite) TestQueryERC20Bytes32Fallback() {
	var mockEVMKeeper *erc20mocks.EVMKeeper
	contract := utiltx.GenerateAddress()

	// Helper function to create bytes32 encoded data (for MKR-type tokens)
	createBytes32Data := func(text string) []byte {
		data := make([]byte, 32)
		copy(data, []byte(text))
		return data
	}

	// Helper function to create string encoded data (for standard ERC20 tokens)
	createStringData := func(text string) []byte {
		// ABI encoding for string: [offset][length][data_padded]
		textBytes := []byte(text)
		textLen := len(textBytes)

		// Pad to 32-byte boundary
		paddedLen := ((textLen + 31) / 32) * 32
		data := make([]byte, 64+paddedLen)

		// Offset (32 bytes) - points to start of string data
		data[31] = 32

		// Length (32 bytes)
		data[63] = byte(textLen)

		// String data (padded to 32-byte boundary)
		copy(data[64:64+textLen], textBytes)

		return data
	}

	testCases := []struct {
		name        string
		malleate    func()
		expectedRes types.ERC20Data
		shouldPass  bool
	}{
		{
			"Standard ERC20 - both name and symbol as string",
			func() {
				nameData := createStringData("Maker")
				symbolData := createStringData("MKR")
				decimalsData := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 18}

				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "name").
					Return(&evmtypes.MsgEthereumTxResponse{Ret: nameData}, nil).Once()
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "symbol").
					Return(&evmtypes.MsgEthereumTxResponse{Ret: symbolData}, nil).Once()
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "decimals").
					Return(&evmtypes.MsgEthereumTxResponse{Ret: decimalsData}, nil).Once()
			},
			types.ERC20Data{Name: "Maker", Symbol: "MKR", Decimals: 18},
			true,
		},
		{
			"MKR-type token - both name and symbol as bytes32",
			func() {
				nameData := createBytes32Data("Maker")
				symbolData := createBytes32Data("MKR")
				decimalsData := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 18}

				// First call tries string unpacking (will fail), then tries bytes32 (will succeed)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "name").
					Return(&evmtypes.MsgEthereumTxResponse{Ret: nameData}, nil).Once()
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "symbol").
					Return(&evmtypes.MsgEthereumTxResponse{Ret: symbolData}, nil).Once()
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "decimals").
					Return(&evmtypes.MsgEthereumTxResponse{Ret: decimalsData}, nil).Once()
			},
			types.ERC20Data{Name: "Maker", Symbol: "MKR", Decimals: 18},
			true,
		},
		{
			"Mixed - name as string, symbol as bytes32",
			func() {
				nameData := createStringData("Maker")
				symbolData := createBytes32Data("MKR")
				decimalsData := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 18}

				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "name").
					Return(&evmtypes.MsgEthereumTxResponse{Ret: nameData}, nil).Once()
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "symbol").
					Return(&evmtypes.MsgEthereumTxResponse{Ret: symbolData}, nil).Once()
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "decimals").
					Return(&evmtypes.MsgEthereumTxResponse{Ret: decimalsData}, nil).Once()
			},
			types.ERC20Data{Name: "Maker", Symbol: "MKR", Decimals: 18},
			true,
		},
		{
			"Bytes32 with null termination",
			func() {
				// Create bytes32 data with null bytes (like real MKR token)
				nameData := make([]byte, 32)
				copy(nameData[:5], []byte("Maker"))
				// Rest is already zero-filled

				symbolData := make([]byte, 32)
				copy(symbolData[:3], []byte("MKR"))
				// Rest is already zero-filled

				decimalsData := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 18}

				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "name").
					Return(&evmtypes.MsgEthereumTxResponse{Ret: nameData}, nil).Once()
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "symbol").
					Return(&evmtypes.MsgEthereumTxResponse{Ret: symbolData}, nil).Once()
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "decimals").
					Return(&evmtypes.MsgEthereumTxResponse{Ret: decimalsData}, nil).Once()
			},
			types.ERC20Data{Name: "Maker", Symbol: "MKR", Decimals: 18},
			true,
		},
		{
			"EVM call fails for name",
			func() {
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "name").
					Return(nil, fmt.Errorf("EVM call failed")).Once()
			},
			types.ERC20Data{},
			false,
		},
		{
			"Invalid data - both string and bytes32 unpacking fail for name",
			func() {
				invalidData := []byte{0xFF, 0xFF} // Invalid data that will fail both unpacking methods

				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "name").
					Return(&evmtypes.MsgEthereumTxResponse{Ret: invalidData}, nil).Once()
			},
			types.ERC20Data{},
			false,
		},
		{
			"EVM call succeeds for name but fails for symbol",
			func() {
				nameData := createStringData("Maker")

				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "name").
					Return(&evmtypes.MsgEthereumTxResponse{Ret: nameData}, nil).Once()
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "symbol").
					Return(nil, fmt.Errorf("EVM call failed")).Once()
			},
			types.ERC20Data{},
			false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset

			transferKeeper := s.network.App.GetTransferKeeper()
			mockEVMKeeper = &erc20mocks.EVMKeeper{}
			s.network.App.SetErc20Keeper(keeper.NewKeeper(
				s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
				authtypes.NewModuleAddress(govtypes.ModuleName),
				s.network.App.GetAccountKeeper(), s.network.App.GetBankKeeper(),
				mockEVMKeeper, s.network.App.GetStakingKeeper(),
				&transferKeeper,
			))

			tc.malleate()

			res, err := s.network.App.GetErc20Keeper().QueryERC20(s.network.GetContext(), contract)

			if tc.shouldPass {
				s.Require().NoError(err, "Test case should pass but got error: %v", err)
				s.Require().Equal(tc.expectedRes, res, "Expected result mismatch")
			} else {
				s.Require().Error(err, "Test case should fail but succeeded")
			}
		})
	}
}
