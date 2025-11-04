package types_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/encoding"
	utiltx "github.com/cosmos/evm/testutil/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	proto "github.com/cosmos/gogoproto/proto"

	"github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
)

func TestEvmDataEncoding(t *testing.T) {
	ret := []byte{0x5, 0x8}

	data := &evmtypes.MsgEthereumTxResponse{
		Hash: common.BytesToHash([]byte("hash")).String(),
		Logs: []*evmtypes.Log{{
			Data:        []byte{1, 2, 3, 4},
			BlockNumber: 17,
		}},
		Ret: ret,
	}

	anyData := codectypes.UnsafePackAny(data)
	txData := &sdk.TxMsgData{
		MsgResponses: []*codectypes.Any{anyData},
	}

	txDataBz, err := proto.Marshal(txData)
	require.NoError(t, err)

	res, err := evmtypes.DecodeTxResponse(txDataBz)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, data.Logs, res.Logs)
	require.Equal(t, ret, res.Ret)
}

func TestDecodeTxResponse(t *testing.T) {
	testCases := []struct {
		name        string
		setupData   func() []byte
		expectError bool
		expectEmpty bool
	}{
		{
			name: "valid single tx response",
			setupData: func() []byte {
				ret := []byte{0x1, 0x2, 0x3}
				data := &evmtypes.MsgEthereumTxResponse{
					Hash: common.BytesToHash([]byte("single_hash")).String(),
					Logs: []*evmtypes.Log{{
						Address:     common.HexToAddress("0x1234").String(),
						Topics:      []string{common.BytesToHash([]byte("topic1")).String()},
						Data:        []byte{5, 6, 7, 8},
						BlockNumber: 42,
						TxHash:      common.BytesToHash([]byte("single_hash")).String(),
						TxIndex:     0,
						Index:       0,
					}},
					Ret: ret,
				}
				anyData := codectypes.UnsafePackAny(data)
				txData := &sdk.TxMsgData{
					MsgResponses: []*codectypes.Any{anyData},
				}
				txDataBz, _ := proto.Marshal(txData)
				return txDataBz
			},
			expectError: false,
			expectEmpty: false,
		},
		{
			name: "empty tx response data",
			setupData: func() []byte {
				txData := &sdk.TxMsgData{
					MsgResponses: []*codectypes.Any{},
				}
				txDataBz, _ := proto.Marshal(txData)
				return txDataBz
			},
			expectError: false,
			expectEmpty: true,
		},
		{
			name: "invalid protobuf data",
			setupData: func() []byte {
				return []byte("invalid protobuf data")
			},
			expectError: true,
			expectEmpty: false,
		},
		{
			name: "nil input data",
			setupData: func() []byte {
				return nil
			},
			expectError: false,
			expectEmpty: true,
		},
		{
			name: "non-ethereum tx response",
			setupData: func() []byte {
				// Pack a different message type
				bankMsg := &codectypes.Any{
					TypeUrl: "/cosmos.bank.v1beta1.MsgSendResponse",
					Value:   []byte("some data"),
				}
				txData := &sdk.TxMsgData{
					MsgResponses: []*codectypes.Any{bankMsg},
				}
				txDataBz, _ := proto.Marshal(txData)
				return txDataBz
			},
			expectError: false,
			expectEmpty: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evmtypes.DecodeTxResponse(tc.setupData())
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.expectEmpty {
					require.Equal(t, &evmtypes.MsgEthereumTxResponse{}, result)
				} else {
					require.NotNil(t, result)
					require.Equal(t, common.BytesToHash([]byte("single_hash")).String(), result.Hash)
					require.Len(t, result.Logs, 1)
					require.Equal(t, []byte{1, 2, 3}, result.Ret)
				}
			}
		})
	}
}

func createTxResponseData(t *testing.T, key string) ([]byte, []*evmtypes.MsgEthereumTxResponse) {
	t.Helper()
	switch key {
	case "multiple":
		// 1st response
		data1 := &evmtypes.MsgEthereumTxResponse{
			Hash: common.BytesToHash([]byte("hash1")).String(),
			Logs: []*evmtypes.Log{createLog(t, testAddress, []string{testTopic}, 0, 0)},
			Ret:  []byte{0x1},
		}
		// 2nd response
		data2 := &evmtypes.MsgEthereumTxResponse{
			Hash: common.BytesToHash([]byte("hash2")).String(),
			Logs: []*evmtypes.Log{createLog(t, testAddress, []string{testTopic}, 0, 0)},
			Ret:  []byte{0x2},
		}
		anyData1 := codectypes.UnsafePackAny(data1)
		anyData2 := codectypes.UnsafePackAny(data2)
		txData := &sdk.TxMsgData{
			MsgResponses: []*codectypes.Any{anyData1, anyData2},
		}
		txDataBz, _ := proto.Marshal(txData)
		return txDataBz, []*evmtypes.MsgEthereumTxResponse{data1, data2}
	case "single":
		data := &evmtypes.MsgEthereumTxResponse{
			Hash: common.BytesToHash([]byte("hash")).String(),
			Logs: []*evmtypes.Log{createLog(t, testAddress, []string{testTopic}, 0, 0)},
			Ret:  []byte{0x5, 0x8},
		}
		anyData := codectypes.UnsafePackAny(data)
		txData := &sdk.TxMsgData{
			MsgResponses: []*codectypes.Any{anyData},
		}
		txDataBz, _ := proto.Marshal(txData)
		return txDataBz, []*evmtypes.MsgEthereumTxResponse{data}
	case "empty":
		txData := &sdk.TxMsgData{
			MsgResponses: []*codectypes.Any{},
		}
		txDataBz, _ := proto.Marshal(txData)
		return txDataBz, []*evmtypes.MsgEthereumTxResponse{}
	case "mixed":
		// EVM response
		evmData := &evmtypes.MsgEthereumTxResponse{
			Hash: common.BytesToHash([]byte("evm_hash")).String(),
			Ret:  []byte{0x99},
		}
		evmAnyData := codectypes.UnsafePackAny(evmData)
		// Non-EVM response
		bankData := &codectypes.Any{
			TypeUrl: "/cosmos.bank.v1beta1.MsgSendResponse",
			Value:   []byte("bank response"),
		}
		txData := &sdk.TxMsgData{
			MsgResponses: []*codectypes.Any{evmAnyData, bankData},
		}
		txDataBz, _ := proto.Marshal(txData)
		return txDataBz, []*evmtypes.MsgEthereumTxResponse{evmData}
	case "invalid":
		return []byte("invalid protobuf data"), nil
	case "nil":
		return nil, nil
	default:
		return []byte{}, nil
	}
}

func TestDecodeTxResponses(t *testing.T) {
	testCases := []struct {
		name         string
		txDataKey    string
		expectError  bool
		expectLength int
		expectNil    bool
	}{
		{
			name:         "multiple tx responses",
			txDataKey:    "multiple",
			expectError:  false,
			expectLength: 2,
			expectNil:    false,
		},
		{
			name:         "single tx response",
			txDataKey:    "single",
			expectError:  false,
			expectLength: 1,
			expectNil:    false,
		},
		{
			name:         "empty responses",
			txDataKey:    "empty",
			expectError:  false,
			expectLength: 0,
			expectNil:    false,
		},
		{
			name:         "mixed response types",
			txDataKey:    "mixed",
			expectError:  false,
			expectLength: 1, // Only EVM responses are included
			expectNil:    false,
		},
		{
			name:         "invalid protobuf data",
			txDataKey:    "invalid",
			expectError:  true,
			expectLength: 0,
			expectNil:    true,
		},
		{
			name:         "nil input",
			txDataKey:    "nil",
			expectError:  false,
			expectLength: 0,
			expectNil:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			txDataBz, _ := createTxResponseData(t, tc.txDataKey)
			results, err := evmtypes.DecodeTxResponses(txDataBz)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tc.expectNil {
				require.Nil(t, results)
			} else {
				require.NotNil(t, results)
				require.Len(t, results, tc.expectLength)

				// Verify specific content for known test cases
				if tc.name == "multiple tx responses" {
					require.Equal(t, common.BytesToHash([]byte("hash1")).String(), results[0].Hash)
					require.Equal(t, common.BytesToHash([]byte("hash2")).String(), results[1].Hash)
					require.Equal(t, []byte{0x1}, results[0].Ret)
					require.Equal(t, []byte{0x2}, results[1].Ret)
				}

				if tc.name == "single tx response" {
					require.Equal(t, common.BytesToHash([]byte("hash")).String(), results[0].Hash)
					require.Equal(t, []byte{0x5, 0x8}, results[0].Ret)
					require.Len(t, results[0].Logs, 1)
					require.Equal(t, []byte(testData), results[0].Logs[0].Data)
				}

				if tc.name == "mixed response types" {
					require.Equal(t, common.BytesToHash([]byte("evm_hash")).String(), results[0].Hash)
					require.Equal(t, []byte{0x99}, results[0].Ret)
				}
			}
		})
	}
}

const (
	testAddress = "0xc5570e6B97044960be06962E13248EC6b13107AE"
	testTopic   = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	testData    = "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAo="
)

func createLog(t *testing.T, address string, topics []string, txIndex, logIndex uint64) *evmtypes.Log {
	t.Helper()
	return &evmtypes.Log{
		Address:     address,
		Topics:      topics,
		Data:        []byte(testData),
		BlockNumber: uint64(3),
		TxHash:      "0x0eb002bd8fa02c0b0d549acfca70f7aab5fa745af118c76dda60a1f4329d0de1",
		TxIndex:     txIndex,
		BlockHash:   "0xa7a5ee692701bb2f971b9d1a1ab4bbf10599b0ce3814ea2b60c59a4a4a1d2e4c",
		Index:       logIndex,
		Removed:     false,
	}
}

func TestUnwrapEthererumMsg(t *testing.T) {
	chainID := big.NewInt(1)
	_, err := evmtypes.UnwrapEthereumMsg(nil, common.Hash{})
	require.NotNil(t, err)

	encodingConfig := encoding.MakeConfig(chainID.Uint64())
	clientCtx := client.Context{}.WithTxConfig(encodingConfig.TxConfig)
	builder, _ := clientCtx.TxConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)

	tx := builder.GetTx().(sdk.Tx)
	_, err = evmtypes.UnwrapEthereumMsg(&tx, common.Hash{})
	require.NotNil(t, err)

	evmTxParams := &evmtypes.EvmTxArgs{
		ChainID:  chainID,
		Nonce:    0,
		To:       &common.Address{},
		Amount:   big.NewInt(0),
		GasLimit: 0,
		GasPrice: big.NewInt(0),
		Input:    []byte{},
	}

	msg := evmtypes.NewTx(evmTxParams)
	err = builder.SetMsgs(msg)
	require.Nil(t, err)

	tx = builder.GetTx().(sdk.Tx)
	unwrappedMsg, err := evmtypes.UnwrapEthereumMsg(&tx, msg.AsTransaction().Hash())
	require.Nil(t, err)
	require.Equal(t, unwrappedMsg, msg)
}

func TestBinSearch(t *testing.T) {
	successExecutable := func(gas uint64) (bool, *evmtypes.MsgEthereumTxResponse, error) {
		target := uint64(21000)
		return gas < target, nil, nil
	}
	failedExecutable := func(_ uint64) (bool, *evmtypes.MsgEthereumTxResponse, error) {
		return true, nil, errors.New("contract failed")
	}

	gas, err := evmtypes.BinSearch(20000, 21001, successExecutable)
	require.NoError(t, err)
	// The helper allows a ~1.5% error ratio, so the upper bound of 21001 is returned here.
	// The RPC estimator still short-circuits native transfers to 21000 (see grpc_query.go).
	require.Equal(t, gas, uint64(21001))

	gas, err = evmtypes.BinSearch(20000, 21001, failedExecutable)
	require.Error(t, err)
	require.Equal(t, gas, uint64(0))
}

func TestTransactionLogsEncodeDecode(t *testing.T) {
	addr := utiltx.GenerateAddress().String()

	txLogs := evmtypes.TransactionLogs{
		Hash: common.BytesToHash([]byte("tx_hash")).String(),
		Logs: []*evmtypes.Log{
			{
				Address:     addr,
				Topics:      []string{common.BytesToHash([]byte("topic")).String()},
				Data:        []byte("data"),
				BlockNumber: 1,
				TxHash:      common.BytesToHash([]byte("tx_hash")).String(),
				TxIndex:     1,
				BlockHash:   common.BytesToHash([]byte("block_hash")).String(),
				Index:       1,
				Removed:     false,
			},
		},
	}

	txLogsEncoded, encodeErr := evmtypes.EncodeTransactionLogs(&txLogs)
	require.Nil(t, encodeErr)

	txLogsEncodedDecoded, decodeErr := evmtypes.DecodeTransactionLogs(txLogsEncoded)
	require.Nil(t, decodeErr)
	require.Equal(t, txLogs, txLogsEncodedDecoded)
}

func TestDecodeMsgLogs(t *testing.T) {
	testCases := []struct {
		name        string
		txDataKey   string
		msgIndex    int
		blockNum    uint64
		expectError bool
	}{
		{
			name:      "multiple tx responses, valid msgIndex 0",
			txDataKey: "multiple",
			msgIndex:  0,
			blockNum:  12,
		},
		{
			name:      "multiple tx responses, valid msgIndex 1",
			txDataKey: "multiple",
			msgIndex:  1,
			blockNum:  12,
		},
		{
			name:      "single tx response, valid msgIndex 0",
			txDataKey: "single",
			msgIndex:  0,
			blockNum:  34,
		},
		{
			name:        "single tx response, invalid msgIndex",
			txDataKey:   "single",
			msgIndex:    1,
			blockNum:    34,
			expectError: true,
		},
		{
			name:      "mixed response types, valid msgIndex 0",
			txDataKey: "mixed",
			msgIndex:  0,
			blockNum:  56,
		},
		{
			name:        "invalid protobuf data",
			txDataKey:   "invalid",
			msgIndex:    0,
			blockNum:    78,
			expectError: true,
		},
		{
			name:        "nil input",
			txDataKey:   "nil",
			msgIndex:    0,
			blockNum:    9,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			txMsgData, resps := createTxResponseData(t, tc.txDataKey)
			logsOut, err := evmtypes.DecodeMsgLogs(txMsgData, tc.msgIndex, tc.blockNum)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, logsOut)
				require.Greater(t, len(resps), tc.msgIndex)
				expResp := resps[tc.msgIndex]
				require.Len(t, logsOut, len(expResp.Logs))
				for i, log := range expResp.Logs {
					ethLog := log.ToEthereum()
					ethLog.TxHash = common.HexToHash(expResp.Hash)
					ethLog.BlockNumber = tc.blockNum
					require.Equal(t, ethLog.Address, logsOut[i].Address)
					require.Equal(t, ethLog.BlockNumber, logsOut[i].BlockNumber)
				}
			}
		})
	}
}

func TestDecodeTxLogs(t *testing.T) {
	testCases := []struct {
		name        string
		txDataKey   string
		blockNum    uint64
		expectError bool
	}{
		{
			name:      "multiple tx responses, valid msgIndex",
			txDataKey: "multiple",
			blockNum:  12,
		},
		{
			name:      "single tx response",
			txDataKey: "single",
			blockNum:  34,
		},
		{
			name:      "mixed response types",
			txDataKey: "mixed",
			blockNum:  56,
		},
		{
			name:        "invalid protobuf data",
			txDataKey:   "invalid",
			blockNum:    78,
			expectError: true,
		},
		{
			name:      "nil input",
			txDataKey: "nil",
			blockNum:  9,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			txMsgData, resps := createTxResponseData(t, tc.txDataKey)
			logsOut, err := evmtypes.DecodeTxLogs(txMsgData, tc.blockNum)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				expLogs := make([]*ethtypes.Log, 0)
				for _, resp := range resps {
					for _, log := range resp.Logs {
						ethLog := log.ToEthereum()
						ethLog.TxHash = common.HexToHash(resp.Hash)
						ethLog.BlockNumber = tc.blockNum
						expLogs = append(expLogs, ethLog)
					}
				}
				require.Equal(t, len(logsOut), len(expLogs))
				for i := range logsOut {
					require.Equal(t, expLogs[i].Address, logsOut[i].Address)
					require.Equal(t, expLogs[i].BlockNumber, logsOut[i].BlockNumber)
				}
			}
		})
	}
}
