package types_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"

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

func TestDecodeTxResponses(t *testing.T) {
	testCases := []struct {
		name         string
		setupData    func() []byte
		expectError  bool
		expectLength int
		expectNil    bool
	}{
		{
			name: "multiple tx responses",
			setupData: func() []byte {
				// 1st response
				data1 := &evmtypes.MsgEthereumTxResponse{
					Hash: common.BytesToHash([]byte("hash1")).String(),
					Logs: []*evmtypes.Log{{
						Address:     common.HexToAddress("0x1111").String(),
						Data:        []byte{1, 2},
						BlockNumber: 10,
					}},
					Ret: []byte{0x1},
				}
				// 2nd response
				data2 := &evmtypes.MsgEthereumTxResponse{
					Hash: common.BytesToHash([]byte("hash2")).String(),
					Logs: []*evmtypes.Log{{
						Address:     common.HexToAddress("0x2222").String(),
						Data:        []byte{3, 4},
						BlockNumber: 11,
					}},
					Ret: []byte{0x2},
				}
				anyData1 := codectypes.UnsafePackAny(data1)
				anyData2 := codectypes.UnsafePackAny(data2)
				txData := &sdk.TxMsgData{
					MsgResponses: []*codectypes.Any{anyData1, anyData2},
				}
				txDataBz, _ := proto.Marshal(txData)
				return txDataBz
			},
			expectError:  false,
			expectLength: 2,
			expectNil:    false,
		},
		{
			name: "single tx response",
			setupData: func() []byte {
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

				txDataBz, _ := proto.Marshal(txData)
				return txDataBz
			},
			expectError:  false,
			expectLength: 1,
			expectNil:    false,
		},
		{
			name: "empty responses",
			setupData: func() []byte {
				txData := &sdk.TxMsgData{
					MsgResponses: []*codectypes.Any{},
				}
				txDataBz, _ := proto.Marshal(txData)
				return txDataBz
			},
			expectError:  false,
			expectLength: 0,
			expectNil:    false,
		},
		{
			name: "mixed response types",
			setupData: func() []byte {
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
				return txDataBz
			},
			expectError:  false,
			expectLength: 1, // Only EVM responses are included
			expectNil:    false,
		},
		{
			name: "invalid protobuf data",
			setupData: func() []byte {
				return []byte("completely invalid data")
			},
			expectError:  true,
			expectLength: 0,
			expectNil:    true,
		},
		{
			name: "nil input",
			setupData: func() []byte {
				return nil
			},
			expectError:  false,
			expectLength: 0,
			expectNil:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := tc.setupData()

			results, err := evmtypes.DecodeTxResponses(data)

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
					require.Equal(t, []byte{1, 2, 3, 4}, results[0].Logs[0].Data)
				}

				if tc.name == "mixed response types" {
					require.Equal(t, common.BytesToHash([]byte("evm_hash")).String(), results[0].Hash)
					require.Equal(t, []byte{0x99}, results[0].Ret)
				}
			}
		})
	}
}

func TestTxLogsFromEvents(t *testing.T) {
	address := "0xc5570e6B97044960be06962E13248EC6b13107AE"
	topic := "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

	testCases := []struct {
		name              string
		events            []abci.Event
		msgIndex          int
		expectNotFoundErr bool
		expectParseErr    bool
		expectLogs        int
		validate          func(t *testing.T, logs []*ethtypes.Log)
	}{
		{
			name: "logs found at index 0",
			events: []abci.Event{
				{
					Type: evmtypes.EventTypeTxLog,
					Attributes: []abci.EventAttribute{
						{
							Key:   evmtypes.AttributeKeyTxLog,
							Value: createLogEventValue(t, address, []string{topic}, 0, 0),
						},
						{
							Key:   evmtypes.AttributeKeyTxLog,
							Value: createLogEventValue(t, "0xd09f7c8c4529cb5d387aa17e33d707c529a6f694", []string{"0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925"}, 0, 1),
						},
					},
				},
			},
			msgIndex:          0,
			expectNotFoundErr: false,
			expectLogs:        2,
			validate: func(t *testing.T, logs []*ethtypes.Log) {
				t.Helper()
				require.Equal(t, common.HexToAddress(address), logs[0].Address)
				require.Equal(t, common.HexToAddress("0xd09f7c8c4529cb5d387aa17e33d707c529a6f694"), logs[1].Address)
				require.Equal(t, common.HexToHash(topic), logs[0].Topics[0])
				require.Equal(t, testBlockNumber, logs[0].BlockNumber)
			},
		},
		{
			name: "logs found at index 1 - skips first event",
			events: []abci.Event{
				// First event (index 0) - should be skipped
				{
					Type: evmtypes.EventTypeTxLog,
					Attributes: []abci.EventAttribute{
						{
							Key:   evmtypes.AttributeKeyTxLog,
							Value: createLogEventValue(t, "0x1111111111111111111111111111111111111111", []string{"0x1111111111111111111111111111111111111111111111111111111111111111"}, 0, 0),
						},
					},
				},
				// Second event (index 1) - should be returned
				{
					Type: evmtypes.EventTypeTxLog,
					Attributes: []abci.EventAttribute{
						{
							Key:   evmtypes.AttributeKeyTxLog,
							Value: createLogEventValue(t, address, []string{topic}, 1, 0),
						},
					},
				},
			},
			msgIndex:          1,
			expectNotFoundErr: false,
			expectLogs:        1,
			validate: func(t *testing.T, logs []*ethtypes.Log) {
				t.Helper()
				require.Equal(t, common.HexToAddress(address), logs[0].Address)
				require.Equal(t, common.HexToHash(topic), logs[0].Topics[0])
				require.Equal(t, testBlockNumber, logs[0].BlockNumber)
				require.Equal(t, uint(1), logs[0].TxIndex)
			},
		},
		{
			name: "logs found at index 2 - skips multiple events",
			events: []abci.Event{
				// Event index 0
				{
					Type: evmtypes.EventTypeTxLog,
					Attributes: []abci.EventAttribute{
						{
							Key:   evmtypes.AttributeKeyTxLog,
							Value: createLogEventValue(t, "0x1111111111111111111111111111111111111111", []string{"0x1111111111111111111111111111111111111111111111111111111111111111"}, 0, 0),
						},
					},
				},
				// Event index 1
				{
					Type: evmtypes.EventTypeTxLog,
					Attributes: []abci.EventAttribute{
						{
							Key:   evmtypes.AttributeKeyTxLog,
							Value: createLogEventValue(t, "0x2222222222222222222222222222222222222222", []string{"0x2222222222222222222222222222222222222222222222222222222222222222"}, 1, 0),
						},
					},
				},
				// Event index 2 - target
				{
					Type: evmtypes.EventTypeTxLog,
					Attributes: []abci.EventAttribute{
						{
							Key:   evmtypes.AttributeKeyTxLog,
							Value: createLogEventValue(t, address, []string{topic}, 2, 0),
						},
					},
				},
			},
			msgIndex:          2,
			expectNotFoundErr: false,
			expectLogs:        1,
			validate: func(t *testing.T, logs []*ethtypes.Log) {
				t.Helper()
				require.Equal(t, common.HexToAddress(address), logs[0].Address)
				require.Equal(t, testBlockNumber, logs[0].BlockNumber)
				require.Equal(t, uint(2), logs[0].TxIndex)
			},
		},
		{
			name: "mixed event types - only tx log events count",
			events: []abci.Event{
				// Non-tx log event - should be ignored
				{
					Type: "other_event_type",
					Attributes: []abci.EventAttribute{
						{Key: "irrelevant", Value: "data"},
					},
				},
				// Tx log event index 0
				{
					Type: evmtypes.EventTypeTxLog,
					Attributes: []abci.EventAttribute{
						{
							Key:   evmtypes.AttributeKeyTxLog,
							Value: createLogEventValue(t, address, []string{topic}, 0, 0),
						},
					},
				},
				// Another non-tx log event - should be ignored
				{
					Type: "another_event",
					Attributes: []abci.EventAttribute{
						{Key: "more", Value: "irrelevant"},
					},
				},
			},
			msgIndex:          0,
			expectNotFoundErr: false,
			expectLogs:        1,
			validate: func(t *testing.T, logs []*ethtypes.Log) {
				t.Helper()
				require.Equal(t, common.HexToAddress(address), logs[0].Address)
				require.Equal(t, testBlockNumber, logs[0].BlockNumber)
			},
		},
		{
			name: "no tx log events found",
			events: []abci.Event{
				{
					Type: "other_event_type",
					Attributes: []abci.EventAttribute{
						{Key: "key", Value: "value"},
					},
				},
				{
					Type: "another_event_type",
					Attributes: []abci.EventAttribute{
						{Key: "key2", Value: "value2"},
					},
				},
			},
			msgIndex:          0,
			expectNotFoundErr: true,
			expectLogs:        0,
			validate:          func(t *testing.T, logs []*ethtypes.Log) { t.Helper() },
		},
		{
			name: "msg index out of range",
			events: []abci.Event{
				{
					Type: evmtypes.EventTypeTxLog,
					Attributes: []abci.EventAttribute{
						{
							Key:   evmtypes.AttributeKeyTxLog,
							Value: createLogEventValue(t, address, []string{topic}, 0, 0),
						},
					},
				},
				{
					Type: evmtypes.EventTypeTxLog,
					Attributes: []abci.EventAttribute{
						{
							Key:   evmtypes.AttributeKeyTxLog,
							Value: createLogEventValue(t, "0x2222222222222222222222222222222222222222", []string{"0x2222222222222222222222222222222222222222222222222222222222222222"}, 1, 0),
						},
					},
				},
			},
			msgIndex:          5, // Only 2 events available, asking for index 5
			expectNotFoundErr: true,
			expectLogs:        0,
			validate:          func(t *testing.T, logs []*ethtypes.Log) { t.Helper() },
		},
		{
			name:              "empty events slice",
			events:            []abci.Event{},
			msgIndex:          0,
			expectNotFoundErr: true,
			expectLogs:        0,
			validate:          func(t *testing.T, logs []*ethtypes.Log) { t.Helper() },
		},
		{
			name: "event with invalid JSON log - should propagate error from ParseTxLogsFromEvent",
			events: []abci.Event{
				{
					Type: evmtypes.EventTypeTxLog,
					Attributes: []abci.EventAttribute{
						{
							Key:   evmtypes.AttributeKeyTxLog,
							Value: "invalid json data",
						},
					},
				},
			},
			msgIndex:          0,
			expectNotFoundErr: true,
			expectParseErr:    true,
			expectLogs:        0,
			validate:          func(t *testing.T, logs []*ethtypes.Log) { t.Helper() },
		},
		{
			name: "event with empty topics and data",
			events: []abci.Event{
				{
					Type: evmtypes.EventTypeTxLog,
					Attributes: []abci.EventAttribute{
						{
							Key:   evmtypes.AttributeKeyTxLog,
							Value: createLogEventValue(t, address, []string{}, 0, 0),
						},
					},
				},
			},
			msgIndex:          0,
			expectNotFoundErr: false,
			expectLogs:        1,
			validate: func(t *testing.T, logs []*ethtypes.Log) {
				t.Helper()
				require.Equal(t, common.HexToAddress(address), logs[0].Address)
				require.Len(t, logs[0].Topics, 0)
				require.Equal(t, testBlockNumber, logs[0].BlockNumber)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logs, err := evmtypes.TxLogsFromEvents(tc.events, tc.msgIndex)
			if tc.expectNotFoundErr {
				require.Error(t, err)
				require.Nil(t, logs)
				if tc.expectParseErr {
					require.Contains(t, err.Error(), "invalid character")
				} else {
					require.Contains(t, err.Error(), fmt.Sprintf("eth tx logs not found for message index %d", tc.msgIndex))
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, logs)
				require.Len(t, logs, tc.expectLogs)
				for _, log := range logs {
					require.NotNil(t, log)
					require.IsType(t, &ethtypes.Log{}, log)
					require.NotNil(t, log.Address)
					require.NotNil(t, log.Topics)
					require.NotNil(t, log.Data)
				}
				tc.validate(t, logs)
			}
		})
	}
}

const testBlockNumber = uint64(3)

// createLogEventValue creates a JSON string representation of an EVM log event
func createLogEventValue(t *testing.T, address string, topics []string, txIndex, logIndex uint64) string {
	t.Helper()
	log := &evmtypes.Log{
		Address:     address,
		Topics:      topics,
		Data:        []byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAo="),
		BlockNumber: testBlockNumber,
		TxHash:      "0x0eb002bd8fa02c0b0d549acfca70f7aab5fa745af118c76dda60a1f4329d0de1",
		TxIndex:     txIndex,
		BlockHash:   "0xa7a5ee692701bb2f971b9d1a1ab4bbf10599b0ce3814ea2b60c59a4a4a1d2e4c",
		Index:       logIndex,
		Removed:     false,
	}
	jsonBytes, err := json.Marshal(log)
	require.NoError(t, err)
	return string(jsonBytes)
}

func TestParseTxLogsFromEvent(t *testing.T) {
	address := "0xc5570e6B97044960be06962E13248EC6b13107AE"
	topic := "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

	testCases := []struct {
		name        string
		event       abci.Event
		expectError bool
		expectLogs  int
		validate    func(t *testing.T, logs []*ethtypes.Log)
	}{
		{
			name: "single valid log",
			event: abci.Event{
				Type: evmtypes.EventTypeTxLog,
				Attributes: []abci.EventAttribute{
					{
						Key: evmtypes.AttributeKeyTxLog,
						Value: createLogEventValue(t, address, []string{
							topic,
							"0x000000000000000000000000378c50d9264c63f3f92b806d4ee56e9d86ffb3ec",
							"0x000000000000000000000000d09f7c8c4529cb5d387aa17e33d707c529a6f694",
						}, 0, 0),
					},
				},
			},
			expectError: false,
			expectLogs:  1,
			validate: func(t *testing.T, logs []*ethtypes.Log) {
				t.Helper()
				log := logs[0]
				require.Equal(t, common.HexToAddress(address), log.Address)
				require.Len(t, log.Topics, 3)
				require.Equal(t, common.HexToHash(topic), log.Topics[0])
				require.Equal(t, testBlockNumber, log.BlockNumber)
				require.Equal(t, uint(0), log.TxIndex)
				require.Equal(t, uint(0), log.Index)
				require.False(t, log.Removed)
			},
		},
		{
			name: "multiple valid logs",
			event: abci.Event{
				Type: evmtypes.EventTypeTxLog,
				Attributes: []abci.EventAttribute{
					{
						Key:   evmtypes.AttributeKeyTxLog,
						Value: createLogEventValue(t, address, []string{topic}, 0, 0),
					},
					{
						Key:   evmtypes.AttributeKeyTxLog,
						Value: createLogEventValue(t, "0xd09f7c8c4529cb5d387aa17e33d707c529a6f694", []string{"0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925"}, 1, 1),
					},
					{
						Key:   evmtypes.AttributeKeyTxLog,
						Value: createLogEventValue(t, "0x378c50d9264c63f3f92b806d4ee56e9d86ffb3ec", []string{"0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"}, 2, 2),
					},
				},
			},
			expectError: false,
			expectLogs:  3,
			validate: func(t *testing.T, logs []*ethtypes.Log) {
				t.Helper()
				require.Equal(t, common.HexToAddress(address), logs[0].Address)
				require.Equal(t, common.HexToAddress("0xd09f7c8c4529cb5d387aa17e33d707c529a6f694"), logs[1].Address)
				require.Equal(t, common.HexToAddress("0x378c50d9264c63f3f92b806d4ee56e9d86ffb3ec"), logs[2].Address)
				require.Equal(t, uint(0), logs[0].TxIndex)
				require.Equal(t, uint(1), logs[1].TxIndex)
				require.Equal(t, uint(2), logs[2].TxIndex)
			},
		},
		{
			name: "event with non-log attributes",
			event: abci.Event{
				Type: evmtypes.EventTypeTxLog,
				Attributes: []abci.EventAttribute{
					{Key: "other_attribute", Value: "some value"},
					{
						Key:   evmtypes.AttributeKeyTxLog,
						Value: createLogEventValue(t, address, []string{topic}, 0, 0),
					},
					{Key: "another_attribute", Value: "another value"},
				},
			},
			expectError: false,
			expectLogs:  1,
			validate: func(t *testing.T, logs []*ethtypes.Log) {
				t.Helper()
				require.Equal(t, common.HexToAddress(address), logs[0].Address)
				require.Equal(t, testBlockNumber, logs[0].BlockNumber)
			},
		},
		{
			name: "event with no log attributes",
			event: abci.Event{
				Type: evmtypes.EventTypeTxLog,
				Attributes: []abci.EventAttribute{
					{Key: "not_a_log", Value: "some value"},
					{Key: "another_non_log", Value: "another value"},
				},
			},
			expectError: false,
			expectLogs:  0,
			validate:    func(t *testing.T, logs []*ethtypes.Log) { t.Helper() }, // No validation needed for empty
		},
		{
			name: "invalid JSON in log attribute",
			event: abci.Event{
				Type: evmtypes.EventTypeTxLog,
				Attributes: []abci.EventAttribute{
					{Key: evmtypes.AttributeKeyTxLog, Value: "invalid json format"},
				},
			},
			expectError: true,
			expectLogs:  0,
			validate:    func(t *testing.T, logs []*ethtypes.Log) { t.Helper() }, // No validation for error case
		},
		{
			name: "malformed JSON structure",
			event: abci.Event{
				Type: evmtypes.EventTypeTxLog,
				Attributes: []abci.EventAttribute{
					{Key: evmtypes.AttributeKeyTxLog, Value: `{"address": "incomplete`},
				},
			},
			expectError: true,
			expectLogs:  0,
			validate:    func(t *testing.T, logs []*ethtypes.Log) { t.Helper() },
		},
		{
			name: "empty log attribute value",
			event: abci.Event{
				Type: evmtypes.EventTypeTxLog,
				Attributes: []abci.EventAttribute{
					{Key: evmtypes.AttributeKeyTxLog, Value: ""},
				},
			},
			expectError: true,
			expectLogs:  0,
			validate:    func(t *testing.T, logs []*ethtypes.Log) { t.Helper() },
		},
		{
			name: "log with empty topics array",
			event: abci.Event{
				Type: evmtypes.EventTypeTxLog,
				Attributes: []abci.EventAttribute{
					{
						Key:   evmtypes.AttributeKeyTxLog,
						Value: createLogEventValue(t, address, []string{}, 0, 0),
					},
				},
			},
			expectError: false,
			expectLogs:  1,
			validate: func(t *testing.T, logs []*ethtypes.Log) {
				t.Helper()
				require.Len(t, logs[0].Topics, 0)
				require.Equal(t, testBlockNumber, logs[0].BlockNumber)
			},
		},
		{
			name: "mixed valid and invalid log attributes",
			event: abci.Event{
				Type: evmtypes.EventTypeTxLog,
				Attributes: []abci.EventAttribute{
					{
						Key:   evmtypes.AttributeKeyTxLog,
						Value: createLogEventValue(t, address, []string{topic}, 0, 0),
					},
					{Key: evmtypes.AttributeKeyTxLog, Value: "invalid json"},
				},
			},
			expectError: true, // Should fail on the invalid JSON
			expectLogs:  0,
			validate:    func(t *testing.T, logs []*ethtypes.Log) { t.Helper() },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logs, err := evmtypes.ParseTxLogsFromEvent(tc.event)
			if tc.expectError {
				require.Error(t, err)
				require.Nil(t, logs)
				return
			}
			require.NoError(t, err)
			if tc.expectLogs == 0 {
				require.Empty(t, logs)
			} else {
				require.NotNil(t, logs)
				require.Len(t, logs, tc.expectLogs)
				for _, log := range logs {
					require.NotNil(t, log)
					require.IsType(t, &ethtypes.Log{}, log)
					require.NotNil(t, log.Address)
					require.NotNil(t, log.Topics)
					require.NotNil(t, log.Data)
				}
				tc.validate(t, logs)
			}
		})
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
	require.Equal(t, gas, uint64(21000))

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
