package types_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

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
			name:  "single tx with no logs is a no-op",
			input: []*abci.ExecTxResult{createEthTxResult(t, "hash1", 0, 0)},
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				require.Len(t, result, 1)
				require.Empty(t, result[0].Events)
			},
		},
		{
			name:  "single tx with logs: log.Index + log.TxIndex rewritten",
			input: []*abci.ExecTxResult{createEthTxResult(t, "hash1", 2, 0)},
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				require.Len(t, result, 1)
				response := unmarshalTxResponse(t, result[0])
				require.Len(t, response.Logs, 2)
				require.Equal(t, uint64(0), response.Logs[0].TxIndex)
				require.Equal(t, uint64(0), response.Logs[0].Index)
				require.Equal(t, uint64(0), response.Logs[1].TxIndex)
				require.Equal(t, uint64(1), response.Logs[1].Index)
			},
		},
		{
			name: "multiple txs with logs: indices monotonic across block",
			input: []*abci.ExecTxResult{
				createEthTxResult(t, "hash1", 2, 0),
				createEthTxResult(t, "hash2", 3, 0),
			},
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				require.Len(t, result, 2)
				response1 := unmarshalTxResponse(t, result[0])
				require.Len(t, response1.Logs, 2)
				require.Equal(t, uint64(0), response1.Logs[0].TxIndex)
				require.Equal(t, uint64(0), response1.Logs[0].Index)
				require.Equal(t, uint64(0), response1.Logs[1].TxIndex)
				require.Equal(t, uint64(1), response1.Logs[1].Index)

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
			name:  "failed tx is skipped (no index increments)",
			input: []*abci.ExecTxResult{createEthTxResult(t, "hash1", 1, 1)},
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				require.Len(t, result, 1)
				require.Empty(t, result[0].Events)
			},
		},
		{
			name: "mixed success and failed txs: eth tx counter only advances on success",
			input: []*abci.ExecTxResult{
				createEthTxResult(t, "hash1", 1, 0),
				createEthTxResult(t, "hash2", 1, 1),
				createEthTxResult(t, "hash3", 1, 0),
			},
			validate: func(t *testing.T, result []*abci.ExecTxResult) {
				t.Helper()
				require.Len(t, result, 3)

				response1 := unmarshalTxResponse(t, result[0])
				require.Equal(t, uint64(0), response1.Logs[0].TxIndex)
				require.Equal(t, uint64(0), response1.Logs[0].Index)

				require.Empty(t, result[1].Events)

				response3 := unmarshalTxResponse(t, result[2])
				require.Equal(t, uint64(1), response3.Logs[0].TxIndex)
				require.Equal(t, uint64(1), response3.Logs[0].Index)
			},
		},
		{
			name: "existing events are preserved",
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
				require.Len(t, result[0].Events, 1)
				require.Equal(t, "existing_event", result[0].Events[0].Type)
			},
		},
		{
			name: "non-ethereum tx msg response is ignored",
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
			result, err := evmtypes.PatchTxResponses(tc.input)
			require.NoError(t, err)
			tc.validate(t, result)
		})
	}
}

func TestPatchTxResponses_LogIndex(t *testing.T) {
	input := []*abci.ExecTxResult{
		createEthTxResult(t, "hash1", 2, 0),
		createEthTxResult(t, "hash2", 3, 0),
		createEthTxResult(t, "hash3", 1, 0),
	}
	result, err := evmtypes.PatchTxResponses(input)
	require.NoError(t, err)
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
	}
}
