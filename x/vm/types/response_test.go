package types_test

import (
	"fmt"
	"math/big"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	abci "github.com/cometbft/cometbft/abci/types"

	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/gogoproto/proto"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func createEthTxResult(t *testing.T, hash string, numLogs int, code uint32) *abci.ExecTxResult {
	t.Helper()
	logs := make([]*evmtypes.Log, numLogs)
	for i := 0; i < numLogs; i++ {
		logs[i] = &evmtypes.Log{Data: []byte{byte(i)}}
	}
	response := &evmtypes.MsgEthereumTxResponse{
		Hash: common.BytesToHash([]byte(hash)).String(),
		Logs: logs,
	}
	anyRsp, _ := codectypes.NewAnyWithValue(response)
	txMsgData := &sdk.TxMsgData{
		MsgResponses: []*codectypes.Any{anyRsp},
	}
	data, _ := proto.Marshal(txMsgData)
	return &abci.ExecTxResult{
		Code: code,
		Data: data,
	}
}

func unmarshalTxResponse(t *testing.T, result *abci.ExecTxResult) *evmtypes.MsgEthereumTxResponse {
	t.Helper()
	var txMsgData sdk.TxMsgData
	err := proto.Unmarshal(result.Data, &txMsgData)
	require.NoError(t, err)
	var response evmtypes.MsgEthereumTxResponse
	err = proto.Unmarshal(txMsgData.MsgResponses[0].Value, &response)
	require.NoError(t, err)
	return &response
}

func requireEventTxIndex(t *testing.T, result *abci.ExecTxResult, expectedIdx string) {
	t.Helper()
	require.Len(t, result.Events, 1)
	require.Equal(t, evmtypes.EventTypeEthereumTx, result.Events[0].Type)
	require.Equal(t, expectedIdx, result.Events[0].Attributes[1].Value)
}

func TestPatchTxResponses(t *testing.T) {
	testCases := []struct {
		name     string
		input    []*abci.ExecTxResult
		validate func(t *testing.T, result []*abci.ExecTxResult)
	}{
		{
			name:  "empty input",
			input: []*abci.ExecTxResult{},
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				require.Empty(t, result)
			},
		},
		{
			name:  "single tx with no logs",
			input: []*abci.ExecTxResult{createEthTxResult(t, "hash1", 0, 0)},
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				require.Len(t, result, 1)
				requireEventTxIndex(t, result[0], "0")
			},
		},
		{
			name:  "single tx with logs",
			input: []*abci.ExecTxResult{createEthTxResult(t, "hash1", 2, 0)},
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				require.Len(t, result, 1)
				requireEventTxIndex(t, result[0], "0")
				response := unmarshalTxResponse(t, result[0])
				require.Len(t, response.Logs, 2)
				require.Equal(t, uint64(0), response.Logs[0].TxIndex)
				require.Equal(t, uint64(0), response.Logs[0].Index)
				require.Equal(t, uint64(0), response.Logs[1].TxIndex)
				require.Equal(t, uint64(1), response.Logs[1].Index)
			},
		},
		{
			name: "multiple txs with logs",
			input: []*abci.ExecTxResult{
				createEthTxResult(t, "hash1", 2, 0),
				createEthTxResult(t, "hash2", 3, 0),
			},
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				require.Len(t, result, 2)
				requireEventTxIndex(t, result[0], "0")
				response1 := unmarshalTxResponse(t, result[0])
				require.Len(t, response1.Logs, 2)
				require.Equal(t, uint64(0), response1.Logs[0].TxIndex)
				require.Equal(t, uint64(0), response1.Logs[0].Index)
				require.Equal(t, uint64(0), response1.Logs[1].TxIndex)
				require.Equal(t, uint64(1), response1.Logs[1].Index)

				requireEventTxIndex(t, result[1], "1")
				response2 := unmarshalTxResponse(t, result[1])
				require.Len(t, response2.Logs, 3)
				require.Equal(t, uint64(1), response2.Logs[0].TxIndex)
				require.Equal(t, uint64(2), response2.Logs[0].Index)
				require.Equal(t, uint64(1), response2.Logs[1].TxIndex)
				require.Equal(t, uint64(3), response2.Logs[1].Index)
				require.Equal(t, uint64(1), response2.Logs[2].TxIndex)
				require.Equal(t, uint64(4), response2.Logs[2].Index)
			},
		},
		{
			name:  "failed tx should be skipped",
			input: []*abci.ExecTxResult{createEthTxResult(t, "hash1", 1, 1)},
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				require.Len(t, result, 1)
				require.Empty(t, result[0].Events)
			},
		},
		{
			name: "mixed success and failed txs",
			input: []*abci.ExecTxResult{
				createEthTxResult(t, "hash1", 1, 0), // Success
				createEthTxResult(t, "hash2", 1, 1), // Failed
				createEthTxResult(t, "hash3", 1, 0), // Success
			},
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				require.Len(t, result, 3)
				requireEventTxIndex(t, result[0], "0")
				require.Empty(t, result[1].Events)
				requireEventTxIndex(t, result[2], "1")
				response3 := unmarshalTxResponse(t, result[2])
				require.Equal(t, uint64(1), response3.Logs[0].TxIndex)
				require.Equal(t, uint64(1), response3.Logs[0].Index)
			},
		},
		{
			name: "tx with existing events",
			input: func() []*abci.ExecTxResult {
				result := createEthTxResult(t, "hash1", 1, 0)
				result.Events = []abci.Event{
					{Type: "existing_event", Attributes: []abci.EventAttribute{{Key: "key", Value: "value"}}},
				}
				return []*abci.ExecTxResult{result}
			}(),
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				require.Len(t, result, 1)
				require.Len(t, result[0].Events, 2)
				require.Equal(t, evmtypes.EventTypeEthereumTx, result[0].Events[0].Type)
				require.Equal(t, "existing_event", result[0].Events[1].Type)
			},
		},
		{
			name: "non-ethereum tx msg should be ignored",
			input: func() []*abci.ExecTxResult {
				anyRsp, _ := codectypes.NewAnyWithValue(&sdk.TxMsgData{})
				txMsgData := &sdk.TxMsgData{MsgResponses: []*codectypes.Any{anyRsp}}
				data, _ := proto.Marshal(txMsgData)
				return []*abci.ExecTxResult{{Code: 0, Data: data}}
			}(),
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				require.Len(t, result, 1)
				require.Empty(t, result[0].Events)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := evmtypes.PatchTxResponses(tc.input, nil, nil)
			tc.validate(t, result)
		})
	}
}

func TestPatchTxResponses_EventAttributes(t *testing.T) {
	txHash := common.BytesToHash([]byte("test_hash"))
	input := []*abci.ExecTxResult{createEthTxResult(t, txHash.Hex(), 0, 0)}
	result := evmtypes.PatchTxResponses(input, nil, nil)

	require.Len(t, result, 1)
	require.Len(t, result[0].Events, 1)

	event := result[0].Events[0]
	require.Equal(t, evmtypes.EventTypeEthereumTx, event.Type)
	require.Len(t, event.Attributes, 2)
	require.Equal(t, evmtypes.AttributeKeyEthereumTxHash, event.Attributes[0].Key)
	require.Equal(t, evmtypes.AttributeKeyTxIndex, event.Attributes[1].Key)
	require.Equal(t, "0", event.Attributes[1].Value)
}

func TestPatchTxResponses_LogIndex(t *testing.T) {
	input := []*abci.ExecTxResult{
		createEthTxResult(t, "hash1", 2, 0), // Logs 0, 1
		createEthTxResult(t, "hash2", 3, 0), // Logs 2, 3, 4
		createEthTxResult(t, "hash3", 1, 0), // Log 5
	}
	result := evmtypes.PatchTxResponses(input, nil, nil)
	expectedLogIndexes := [][]uint64{
		{0, 1},
		{2, 3, 4},
		{5},
	}
	for txIdx, expectedIndexes := range expectedLogIndexes {
		response := unmarshalTxResponse(t, result[txIdx])
		require.Len(t, response.Logs, len(expectedIndexes))
		for logIdx, expectedIndex := range expectedIndexes {
			require.Equal(t, expectedIndex, response.Logs[logIdx].Index)
			require.Equal(t, uint64(txIdx), response.Logs[logIdx].TxIndex) //#nosec G115
		}
		eventTxIndex, err := strconv.ParseUint(result[txIdx].Events[0].Attributes[1].Value, 10, 64)
		require.NoError(t, err)
		require.Equal(t, uint64(txIdx), eventTxIndex) //#nosec G115
	}
}

// mockTx implements sdk.Tx for testing PatchTxResponses expected-failure path.
type mockTx struct {
	msgs []sdk.Msg
}

func (m mockTx) GetMsgs() []sdk.Msg                    { return m.msgs }
func (m mockTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

// createTestEthMsg creates a MsgEthereumTx for testing and returns the msg and its hash.
func createTestEthMsg(to common.Address) (*evmtypes.MsgEthereumTx, common.Hash) {
	msg := evmtypes.NewTx(&evmtypes.EvmTxArgs{
		Nonce:    0,
		To:       &to,
		Amount:   big.NewInt(0),
		GasLimit: 21000,
		GasPrice: big.NewInt(1),
		ChainID:  big.NewInt(1),
	})
	return msg, msg.AsTransaction().Hash()
}

// mockTxDecoder returns a TxDecoder that maps raw tx bytes to pre-built mock txs.
func mockTxDecoder(mapping map[string]sdk.Tx) sdk.TxDecoder {
	return func(rawTx []byte) (sdk.Tx, error) {
		if tx, ok := mapping[string(rawTx)]; ok {
			return tx, nil
		}
		return nil, fmt.Errorf("unknown tx")
	}
}

func TestPatchTxResponses_ExpectedFailure(t *testing.T) {
	to := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	ethMsg, expectedHash := createTestEthMsg(to)
	rawTx := []byte("expected-failure-tx")
	txDecoder := mockTxDecoder(map[string]sdk.Tx{
		string(rawTx): mockTx{msgs: []sdk.Msg{ethMsg}},
	})

	testCases := []struct {
		name     string
		input    []*abci.ExecTxResult
		rawTxs   [][]byte
		decoder  sdk.TxDecoder
		validate func(t *testing.T, result []*abci.ExecTxResult)
	}{
		{
			name:    "ExceedBlockGasLimit gets events and correct txIndex",
			input:   []*abci.ExecTxResult{{Code: 11, Log: evmtypes.ExceedBlockGasLimitError}},
			rawTxs:  [][]byte{rawTx},
			decoder: txDecoder,
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				requireEventTxIndex(t, result[0], "0")
				require.Equal(t, expectedHash.Hex(), result[0].Events[0].Attributes[0].Value)
			},
		},
		{
			name:    "StateDBCommitError gets events and correct txIndex",
			input:   []*abci.ExecTxResult{{Code: 11, Log: evmtypes.StateDBCommitError}},
			rawTxs:  [][]byte{rawTx},
			decoder: txDecoder,
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				requireEventTxIndex(t, result[0], "0")
				require.Equal(t, expectedHash.Hex(), result[0].Events[0].Attributes[0].Value)
			},
		},
		{
			name: "mixed success then expected-failure keeps txIndex aligned",
			input: []*abci.ExecTxResult{
				createEthTxResult(t, "hash1", 1, 0),
				{Code: 11, Log: evmtypes.ExceedBlockGasLimitError},
				createEthTxResult(t, "hash3", 1, 0),
			},
			rawTxs:  [][]byte{nil, rawTx, nil},
			decoder: txDecoder,
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				requireEventTxIndex(t, result[0], "0")
				requireEventTxIndex(t, result[1], "1")
				require.Equal(t, expectedHash.Hex(), result[1].Events[0].Attributes[0].Value)
				requireEventTxIndex(t, result[2], "2") // not 1 since we count expected failure tx
			},
		},
		{
			name: "true failure is skipped and does not increment txIndex",
			input: []*abci.ExecTxResult{
				{Code: 11, Log: "some other error"},
				createEthTxResult(t, "hash1", 1, 0),
			},
			rawTxs:  [][]byte{rawTx, nil},
			decoder: txDecoder,
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				require.Empty(t, result[0].Events)
				requireEventTxIndex(t, result[1], "0")
			},
		},
		{
			name:    "nil txDecoder is safe",
			input:   []*abci.ExecTxResult{{Code: 11, Log: evmtypes.ExceedBlockGasLimitError}},
			rawTxs:  [][]byte{rawTx},
			decoder: nil,
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				require.Empty(t, result[0].Events)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := evmtypes.PatchTxResponses(tc.input, tc.rawTxs, tc.decoder)
			tc.validate(t, result)
		})
	}
}
