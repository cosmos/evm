package runner_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm/x/vm/runner"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
)

type fakeRunner struct {
	results []*abci.ExecTxResult
	err     error
	calls   int
}

func (r *fakeRunner) Run(
	_ context.Context,
	_ storetypes.MultiStore,
	_ [][]byte,
	_ sdk.DeliverTxFunc,
) ([]*abci.ExecTxResult, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	return r.results, nil
}

func encodeEthResponse(t *testing.T, hash string, logs []*evmtypes.Log) []byte {
	t.Helper()
	resp := &evmtypes.MsgEthereumTxResponse{Hash: hash, Logs: logs}
	anyResp, err := codectypes.NewAnyWithValue(resp)
	require.NoError(t, err)
	txMsgData := &sdk.TxMsgData{MsgResponses: []*codectypes.Any{anyResp}}
	data, err := proto.Marshal(txMsgData)
	require.NoError(t, err)
	return data
}

func TestWrap_PatchesLogIndices(t *testing.T) {
	inner := &fakeRunner{
		results: []*abci.ExecTxResult{
			{
				Code: 0,
				Data: encodeEthResponse(t, "0xaa", []*evmtypes.Log{
					{TxIndex: 0, Index: 0},
					{TxIndex: 0, Index: 1},
				}),
			},
			{
				Code: 0,
				Data: encodeEthResponse(t, "0xbb", []*evmtypes.Log{
					{TxIndex: 1, Index: 0},
				}),
			},
		},
	}

	wrapped := runner.Wrap(inner)
	out, err := wrapped.Run(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, inner.calls)
	require.Len(t, out, 2)

	gotLogs := func(data []byte) []*evmtypes.Log {
		var txMsgData sdk.TxMsgData
		require.NoError(t, proto.Unmarshal(data, &txMsgData))
		require.Len(t, txMsgData.MsgResponses, 1)
		var resp evmtypes.MsgEthereumTxResponse
		require.NoError(t, proto.Unmarshal(txMsgData.MsgResponses[0].Value, &resp))
		return resp.Logs
	}

	logs0 := gotLogs(out[0].Data)
	require.Len(t, logs0, 2)
	require.Equal(t, uint64(0), logs0[0].TxIndex)
	require.Equal(t, uint64(0), logs0[0].Index)
	require.Equal(t, uint64(0), logs0[1].TxIndex)
	require.Equal(t, uint64(1), logs0[1].Index)

	logs1 := gotLogs(out[1].Data)
	require.Len(t, logs1, 1)
	require.Equal(t, uint64(1), logs1[0].TxIndex)
	require.Equal(t, uint64(2), logs1[0].Index)
}

func TestWrap_PropagatesInnerError(t *testing.T) {
	want := errors.New("inner boom")
	wrapped := runner.Wrap(&fakeRunner{err: want})
	out, err := wrapped.Run(context.Background(), nil, nil, nil)
	require.ErrorIs(t, err, want)
	require.Nil(t, out)
}

func TestWrap_NoEthResponses(t *testing.T) {
	raw := []byte{0x01, 0x02, 0x03}
	inner := &fakeRunner{
		results: []*abci.ExecTxResult{
			{Code: 0, Data: nil},
			{Code: 1, Data: raw},
		},
	}
	wrapped := runner.Wrap(inner)
	out, err := wrapped.Run(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, out, 2)
	require.Nil(t, out[0].Data)
	require.Equal(t, raw, out[1].Data)
}
