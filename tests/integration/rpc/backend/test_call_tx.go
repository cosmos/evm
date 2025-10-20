package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"google.golang.org/grpc/metadata"

	"github.com/cosmos/evm/rpc/backend/mocks"
	rpctypes "github.com/cosmos/evm/rpc/types"
	"github.com/cosmos/evm/testutil/constants"
	utiltx "github.com/cosmos/evm/testutil/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

func (s *TestSuite) TestResend() {
	txNonce := (hexutil.Uint64)(1)
	baseFee := math.NewInt(1)
	gasPrice := new(hexutil.Big)
	toAddr := utiltx.GenerateAddress()
	evmChainID := (*hexutil.Big)(s.backend.EvmChainID)
	height := int64(1)
	callArgs := evmtypes.TransactionArgs{
		From:                 nil,
		To:                   &toAddr,
		Gas:                  nil,
		GasPrice:             nil,
		MaxFeePerGas:         gasPrice,
		MaxPriorityFeePerGas: gasPrice,
		Value:                gasPrice,
		Nonce:                &txNonce,
		Input:                nil,
		Data:                 nil,
		AccessList:           nil,
		ChainID:              evmChainID,
	}

	testCases := []struct {
		name         string
		registerMock func()
		args         evmtypes.TransactionArgs
		gasPrice     *hexutil.Big
		gasLimit     *hexutil.Uint64
		expHash      common.Hash
		expPass      bool
	}{
		{
			"fail - Missing transaction nonce",
			func() {},
			evmtypes.TransactionArgs{
				Nonce: nil,
			},
			nil,
			nil,
			common.Hash{},
			false,
		},
		{
			"pass - Can't set Tx defaults BaseFee disabled",
			func() {
				var header metadata.MD
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterParams(QueryClient, &header, height)
				RegisterBlock(client, height, nil)
				RegisterBlockResults(client, 1)
				RegisterConsensusParams(client, height)
				RegisterValidatorAccount(QueryClient, sdk.AccAddress(utiltx.GenerateAddress().Bytes()))
				RegisterBaseFeeDisabled(QueryClient)
			},
			evmtypes.TransactionArgs{
				Nonce:   &txNonce,
				ChainID: callArgs.ChainID,
			},
			nil,
			nil,
			common.Hash{},
			true,
		},
		{
			"pass - Can't set Tx defaults",
			func() {
				var header metadata.MD
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				feeMarketClient := s.backend.QueryClient.FeeMarket.(*mocks.FeeMarketQueryClient)
				RegisterParams(QueryClient, &header, height)
				RegisterFeeMarketParams(feeMarketClient, 1)
				RegisterBlock(client, height, nil)
				RegisterBlockResults(client, 1)
				RegisterConsensusParams(client, height)
				RegisterValidatorAccount(QueryClient, sdk.AccAddress(utiltx.GenerateAddress().Bytes()))
				RegisterBaseFee(QueryClient, baseFee)
			},
			evmtypes.TransactionArgs{
				Nonce: &txNonce,
			},
			nil,
			nil,
			common.Hash{},
			true,
		},
		{
			"pass - MaxFeePerGas is nil",
			func() {
				var header metadata.MD
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterParams(QueryClient, &header, height)
				RegisterBlock(client, height, nil)
				RegisterBlockResults(client, 1)
				RegisterConsensusParams(client, height)
				RegisterValidatorAccount(QueryClient, sdk.AccAddress(utiltx.GenerateAddress().Bytes()))
				RegisterBaseFeeDisabled(QueryClient)
			},
			evmtypes.TransactionArgs{
				Nonce:                &txNonce,
				MaxPriorityFeePerGas: nil,
				GasPrice:             nil,
				MaxFeePerGas:         nil,
			},
			nil,
			nil,
			common.Hash{},
			true,
		},
		{
			"fail - GasPrice and (MaxFeePerGas or MaxPriorityPerGas specified)",
			func() {},
			evmtypes.TransactionArgs{
				Nonce:                &txNonce,
				MaxPriorityFeePerGas: nil,
				GasPrice:             gasPrice,
				MaxFeePerGas:         gasPrice,
			},
			nil,
			nil,
			common.Hash{},
			false,
		},
		{
			"fail - Block error",
			func() {
				var header metadata.MD
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterParams(QueryClient, &header, height)
				RegisterBlockError(client, height)
			},
			evmtypes.TransactionArgs{
				Nonce: &txNonce,
			},
			nil,
			nil,
			common.Hash{},
			false,
		},
		{
			"pass - MaxFeePerGas is nil",
			func() {
				var header metadata.MD
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterParams(QueryClient, &header, height)
				RegisterBlock(client, height, nil)
				RegisterBlockResults(client, 1)
				RegisterConsensusParams(client, height)
				RegisterValidatorAccount(QueryClient, sdk.AccAddress(utiltx.GenerateAddress().Bytes()))
				RegisterBaseFee(QueryClient, baseFee)
			},
			evmtypes.TransactionArgs{
				Nonce:                &txNonce,
				GasPrice:             nil,
				MaxPriorityFeePerGas: gasPrice,
				MaxFeePerGas:         gasPrice,
				ChainID:              callArgs.ChainID,
			},
			nil,
			nil,
			common.Hash{},
			true,
		},
		{
			"pass - Chain Id is nil",
			func() {
				var header metadata.MD
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterParams(QueryClient, &header, height)
				RegisterBlock(client, height, nil)
				RegisterBlockResults(client, 1)
				RegisterConsensusParams(client, height)
				RegisterValidatorAccount(QueryClient, sdk.AccAddress(utiltx.GenerateAddress().Bytes()))
				RegisterBaseFee(QueryClient, baseFee)
			},
			evmtypes.TransactionArgs{
				Nonce:                &txNonce,
				MaxPriorityFeePerGas: gasPrice,
				ChainID:              nil,
			},
			nil,
			nil,
			common.Hash{},
			true,
		},
		{
			"fail - Pending transactions error",
			func() {
				var header metadata.MD
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBlock(client, height, nil)
				RegisterHeader(client, &height, nil)
				RegisterBlockResults(client, 1)
				RegisterConsensusParams(client, height)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterEstimateGas(QueryClient, callArgs)
				RegisterParams(QueryClient, &header, height)
				RegisterValidatorAccount(QueryClient, sdk.AccAddress(utiltx.GenerateAddress().Bytes()))
				RegisterUnconfirmedTxsError(client, nil)
			},
			evmtypes.TransactionArgs{
				Nonce:                &txNonce,
				To:                   &toAddr,
				MaxFeePerGas:         gasPrice,
				MaxPriorityFeePerGas: gasPrice,
				Value:                gasPrice,
				Gas:                  nil,
				ChainID:              callArgs.ChainID,
			},
			gasPrice,
			nil,
			common.Hash{},
			false,
		},
		{
			"fail - Not Ethereum txs",
			func() {
				var header metadata.MD
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBlock(client, height, nil)
				RegisterHeader(client, &height, nil)
				RegisterBlockResults(client, 1)
				RegisterConsensusParams(client, height)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterEstimateGas(QueryClient, callArgs)
				RegisterParams(QueryClient, &header, height)
				RegisterValidatorAccount(QueryClient, sdk.AccAddress(utiltx.GenerateAddress().Bytes()))
				RegisterUnconfirmedTxsEmpty(client, nil)
			},
			evmtypes.TransactionArgs{
				Nonce:                &txNonce,
				To:                   &toAddr,
				MaxFeePerGas:         gasPrice,
				MaxPriorityFeePerGas: gasPrice,
				Value:                gasPrice,
				Gas:                  nil,
				ChainID:              callArgs.ChainID,
			},
			gasPrice,
			nil,
			common.Hash{},
			false,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("case %s", tc.name), func() {
			s.SetupTest() // reset test and queries
			tc.registerMock()

			hash, err := s.backend.Resend(tc.args, tc.gasPrice, tc.gasLimit)

			if tc.expPass {
				s.Require().Equal(tc.expHash, hash)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestSendRawTransaction() {
	ethTx, bz := s.buildEthereumTx()

	emptyEvmChainIDTx := s.buildEthereumTxWithChainID(nil)
	invalidChainID := big.NewInt(1)
	// Sign the ethTx
	ethSigner := ethtypes.LatestSigner(s.backend.ChainConfig())
	err := ethTx.Sign(ethSigner, s.signer)
	s.Require().NoError(err)

	rlpEncodedBz, err := ethTx.AsTransaction().MarshalBinary()
	s.Require().NoError(err)
	evmDenom := evmtypes.GetEVMCoinDenom()

	testCases := []struct {
		name         string
		registerMock func()
		rawTx        func() []byte
		expHash      common.Hash
		expError     string
		expPass      bool
	}{
		{
			"fail - empty bytes",
			func() {},
			func() []byte { return []byte{} },
			common.Hash{},
			"",
			false,
		},
		{
			"fail - no RLP encoded bytes",
			func() {},
			func() []byte { return bz },
			common.Hash{},
			"",
			false,
		},
		{
			"fail - invalid chain-id",
			func() {
				s.backend.AllowUnprotectedTxs = false
			},
			func() []byte {
				from, priv := utiltx.NewAddrKey()
				signer := utiltx.NewSigner(priv)
				invalidEvmChainIDTx := s.buildEthereumTxWithChainID(invalidChainID)
				invalidEvmChainIDTx.From = from.Bytes()
				err := invalidEvmChainIDTx.Sign(ethtypes.LatestSignerForChainID(invalidChainID), signer)
				s.Require().NoError(err)
				bytes, _ := rlp.EncodeToBytes(invalidEvmChainIDTx.AsTransaction())
				return bytes
			},
			common.Hash{},
			fmt.Errorf("incorrect chain-id; expected %d, got %d", constants.ExampleChainID.EVMChainID, invalidChainID).Error(),
			false,
		},
		{
			"fail - unprotected tx",
			func() {
				s.backend.AllowUnprotectedTxs = false
			},
			func() []byte {
				bytes, _ := rlp.EncodeToBytes(emptyEvmChainIDTx.AsTransaction())
				return bytes
			},
			common.Hash{},
			errors.New("only replay-protected (EIP-155) transactions allowed over RPC").Error(),
			false,
		},
		{
			"fail - failed to broadcast transaction",
			func() {
				cosmosTx, _ := ethTx.BuildTx(s.backend.ClientCtx.TxConfig.NewTxBuilder(), evmDenom)
				txBytes, _ := s.backend.ClientCtx.TxConfig.TxEncoder()(cosmosTx)

				client := s.backend.ClientCtx.Client.(*mocks.Client)
				s.backend.AllowUnprotectedTxs = true
				RegisterBroadcastTxError(client, txBytes)
			},
			func() []byte {
				bytes, _ := rlp.EncodeToBytes(ethTx.AsTransaction())
				return bytes
			},
			ethTx.Hash(),
			errortypes.ErrInvalidRequest.Error(),
			false,
		},
		{
			"pass - Gets the correct transaction hash of the eth transaction",
			func() {
				cosmosTx, _ := ethTx.BuildTx(s.backend.ClientCtx.TxConfig.NewTxBuilder(), evmDenom)
				txBytes, _ := s.backend.ClientCtx.TxConfig.TxEncoder()(cosmosTx)

				client := s.backend.ClientCtx.Client.(*mocks.Client)
				s.backend.AllowUnprotectedTxs = true
				RegisterBroadcastTx(client, txBytes)
			},
			func() []byte { return rlpEncodedBz },
			ethTx.Hash(),
			"",
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("case %s", tc.name), func() {
			s.SetupTest() // reset test and queries
			tc.registerMock()

			hash, err := s.backend.SendRawTransaction(tc.rawTx())

			if tc.expPass {
				s.Require().Equal(tc.expHash, hash)
			} else {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.expError)
			}
		})
	}
}

func (s *TestSuite) registerMock(bz []byte, expectedRequest *evmtypes.EthCallRequest, shouldError bool) func() {
	return func() {
		client := s.backend.ClientCtx.Client.(*mocks.Client)
		QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
		height := int64(1)
		RegisterHeader(client, &height, bz)
		// Register precompile for any address that might be called
		RegisterPrecompile(QueryClient, "")
		if shouldError {
			RegisterEthCallError(QueryClient, expectedRequest)
		} else {
			RegisterEthCall(QueryClient, expectedRequest)
		}
	}
}

func (s *TestSuite) TestDoCall() {
	_, bz := s.buildEthereumTx()
	gasPrice := (*hexutil.Big)(big.NewInt(1))
	toAddr := utiltx.GenerateAddress()
	evmChainID := (*hexutil.Big)(s.backend.EvmChainID)
	callArgs := evmtypes.TransactionArgs{
		From:                 nil,
		To:                   &toAddr,
		Gas:                  nil,
		GasPrice:             nil,
		MaxFeePerGas:         gasPrice,
		MaxPriorityFeePerGas: gasPrice,
		Value:                gasPrice,
		Input:                nil,
		Data:                 nil,
		AccessList:           nil,
		ChainID:              evmChainID,
	}

	var err error
	argsBz, err := json.Marshal(callArgs)
	s.Require().NoError(err)

	evmOverrides := json.RawMessage(`{
        "` + toAddr.Hex() + `": {
            "balance": "0x1000000000000000000",
            "nonce": "0x1",
            "code": "0x608060405234801561001057600080fd5b50600436106100365760003560e01c8063c6888fa11461003b578063c8e7ca2e14610057575b600080fd5b610055600480360381019061005091906100a3565b610075565b005b61005f61007f565b60405161006c91906100e1565b60405180910390f35b8060008190555050565b60008054905090565b600080fd5b6000819050919050565b61009d8161008a565b81146100a857600080fd5b50565b6000813590506100ba81610094565b92915050565b6000602082840312156100d6576100d5610085565b5b60006100e4848285016100ab565b91505092915050565b6100f68161008a565b82525050565b600060208201905061011160008301846100ed565b9291505056fea2646970667358221220c7d2d7c0b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b264736f6c634300080a0033",
            "storage": {
                "0x0000000000000000000000000000000000000000000000000000000000000000": "0x123"
            }
        }
    }`)
	evmOverridesParsed, _, err := rpctypes.ParseOverrides(&evmOverrides, false)
	s.Require().NoError(err)
	var bzOverrides []byte
	if evmOverridesParsed != nil {
		bzOverrides, err = json.Marshal(evmOverridesParsed)
		s.Require().NoError(err)
	}

	invalidOverrides := json.RawMessage(`{"invalid": json}`)
	emptyOverrides := json.RawMessage(`{}`)
	baseRequest := &evmtypes.EthCallRequest{Args: argsBz, ChainId: s.backend.EvmChainID.Int64()}
	testCases := []struct {
		name         string
		registerMock func()
		blockNum     rpctypes.BlockNumber
		callArgs     evmtypes.TransactionArgs
		overrides    *json.RawMessage
		expEthTx     *evmtypes.MsgEthereumTxResponse
		expPass      bool
	}{
		{
			"fail - Invalid request",
			s.registerMock(bz, baseRequest, true),
			rpctypes.BlockNumber(1),
			callArgs,
			nil,
			&evmtypes.MsgEthereumTxResponse{},
			false,
		},
		{
			"pass - Returned transaction response",
			s.registerMock(bz, baseRequest, false),
			rpctypes.BlockNumber(1),
			callArgs,
			nil,
			&evmtypes.MsgEthereumTxResponse{},
			true,
		},
		{
			"pass - With EVM state overrides",
			func() {
				_, expectedCosmosOverrides, err := rpctypes.ParseOverrides(&evmOverrides, false)
				s.Require().NoError(err)
				expectedRequest := &evmtypes.EthCallRequest{
					Args:           argsBz,
					ChainId:        s.backend.EvmChainID.Int64(),
					Overrides:      bzOverrides,
					StateOverrides: expectedCosmosOverrides,
				}
				s.registerMock(bz, expectedRequest, false)()
			},
			rpctypes.BlockNumber(1),
			callArgs,
			&evmOverrides,
			&evmtypes.MsgEthereumTxResponse{},
			true,
		},
		{
			"fail - Invalid state overrides JSON",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				height := int64(1)
				RegisterHeader(client, &height, bz)
				RegisterPrecompile(QueryClient, "")
			},
			rpctypes.BlockNumber(1),
			callArgs,
			&invalidOverrides,
			&evmtypes.MsgEthereumTxResponse{},
			false,
		},
		{
			"pass - Empty state overrides",
			func() {
				_, expectedCosmosOverrides, err := rpctypes.ParseOverrides(&emptyOverrides, false)
				s.Require().NoError(err)
				expectedRequest := &evmtypes.EthCallRequest{
					Args:           argsBz,
					ChainId:        s.backend.EvmChainID.Int64(),
					Overrides:      nil,
					StateOverrides: expectedCosmosOverrides,
				}
				s.registerMock(bz, expectedRequest, false)()
			},
			rpctypes.BlockNumber(1),
			callArgs,
			&emptyOverrides,
			&evmtypes.MsgEthereumTxResponse{},
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("case %s", tc.name), func() {
			s.SetupTest() // reset test and queries
			tc.registerMock()

			msgEthTx, err := s.backend.DoCall(tc.callArgs, tc.blockNum, tc.overrides)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(tc.expEthTx, msgEthTx)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestGasPrice() {
	defaultGasPrice := (*hexutil.Big)(big.NewInt(1))
	height := int64(1)
	testCases := []struct {
		name         string
		registerMock func()
		expGas       *hexutil.Big
		expPass      bool
	}{
		{
			"pass - get the default gas price",
			func() {
				var header metadata.MD
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				feeMarketClient := s.backend.QueryClient.FeeMarket.(*mocks.FeeMarketQueryClient)
				RegisterFeeMarketParams(feeMarketClient, 1)
				RegisterParams(QueryClient, &header, height)
				RegisterGlobalMinGasPrice(QueryClient, 1)
				RegisterBlock(client, height, nil)
				RegisterBlockResults(client, 1)
				RegisterConsensusParams(client, height)
				RegisterValidatorAccount(QueryClient, sdk.AccAddress(utiltx.GenerateAddress().Bytes()))
				RegisterBaseFee(QueryClient, math.NewInt(1))
			},
			defaultGasPrice,
			true,
		},
		{
			"fail - can't get gasFee, FeeMarketParams error",
			func() {
				var header metadata.MD
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				feeMarketClient := s.backend.QueryClient.FeeMarket.(*mocks.FeeMarketQueryClient)
				RegisterFeeMarketParamsError(feeMarketClient, 1)
				RegisterParams(QueryClient, &header, height)
				RegisterBlock(client, height, nil)
				RegisterBlockResults(client, 1)
				RegisterConsensusParams(client, height)
				RegisterValidatorAccount(QueryClient, sdk.AccAddress(utiltx.GenerateAddress().Bytes()))
				RegisterBaseFee(QueryClient, math.NewInt(1))
			},
			defaultGasPrice,
			false,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("case %s", tc.name), func() {
			s.SetupTest() // reset test and queries
			tc.registerMock()

			gasPrice, err := s.backend.GasPrice()
			if tc.expPass {
				s.Require().Equal(tc.expGas, gasPrice)
			} else {
				s.Require().Error(err)
			}
		})
	}
}
