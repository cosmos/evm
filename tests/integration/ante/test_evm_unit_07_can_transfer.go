package ante

import (
	"fmt"
	"math/big"

	"github.com/cosmos/evm/ante/evm"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

func (s *EvmUnitAnteTestSuite) TestCanTransfer() {
	keyring := testkeyring.New(1)
	unitNetwork := network.NewUnitTestNetwork(
		s.create,
		network.WithChainID(testconstants.ChainID{
			ChainID:    s.ChainID,
			EVMChainID: s.EvmChainID,
		}),
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
	)
	grpcHandler := grpc.NewIntegrationHandler(unitNetwork)
	txFactory := factory.New(unitNetwork, grpcHandler)
	senderKey := keyring.GetKey(0)

	testCases := []struct {
		name          string
		expectedError error
		isLondon      bool
		malleate      func(txArgs *evmtypes.EvmTxArgs)
	}{
		{
			name:          "fail: isLondon and insufficient fee",
			expectedError: errortypes.ErrInsufficientFee,
			isLondon:      true,
			malleate: func(txArgs *evmtypes.EvmTxArgs) {
				txArgs.GasFeeCap = big.NewInt(0)
			},
		},
		{
			name:          "fail: invalid tx with insufficient balance",
			expectedError: errortypes.ErrInsufficientFunds,
			isLondon:      true,
			malleate: func(txArgs *evmtypes.EvmTxArgs) {
				balanceResp, err := grpcHandler.GetBalanceFromEVM(senderKey.AccAddr)
				s.Require().NoError(err)

				balance, ok := math.NewIntFromString(balanceResp.Balance)
				s.Require().True(ok)
				invalidAmount := balance.Add(math.NewInt(1)).BigInt()
				txArgs.Amount = invalidAmount
			},
		},
		{
			name:          "success: valid tx and sufficient balance",
			expectedError: nil,
			isLondon:      true,
			malleate: func(*evmtypes.EvmTxArgs) {
			},
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("%v_%v_%v", evmtypes.GetTxTypeName(s.EthTxType), s.ChainID, tc.name), func() {
			baseFeeResp, err := grpcHandler.GetEvmBaseFee()
			s.Require().NoError(err)
			evmParams, err := grpcHandler.GetEvmParams()
			s.Require().NoError(err)
			txArgs, err := txFactory.GenerateDefaultTxTypeArgs(senderKey.Addr, s.EthTxType)
			s.Require().NoError(err)
			txArgs.Amount = big.NewInt(100)

			tc.malleate(&txArgs)

			msg := evmtypes.NewTx(&txArgs)
			msg.From = senderKey.Addr.Bytes()
			signMsg, err := txFactory.SignMsgEthereumTx(senderKey.Priv, *msg)
			s.Require().NoError(err)
			coreMsg, err := signMsg.AsMessage(baseFeeResp.BaseFee.BigInt())
			s.Require().NoError(err)

			// Function under test
			err = evm.CanTransfer(
				unitNetwork.GetContext(),
				unitNetwork.App.GetEVMKeeper(),
				*coreMsg,
				baseFeeResp.BaseFee.BigInt(),
				evmParams.Params,
				tc.isLondon,
			)

			if tc.expectedError != nil {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.expectedError.Error())

			} else {
				s.Require().NoError(err)
			}
		})
	}
}
