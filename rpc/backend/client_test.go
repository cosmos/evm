package backend

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/bytes"
	cmtversion "github.com/cometbft/cometbft/proto/tendermint/version"
	cmtrpcclient "github.com/cometbft/cometbft/rpc/client"
	cmtrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/types"
	"github.com/cometbft/cometbft/version"

	"github.com/cosmos/evm/rpc/backend/mocks"
	rpc "github.com/cosmos/evm/rpc/types"
	"github.com/cosmos/evm/testutil/constants"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

var _ cmtrpcclient.Client = &mocks.Client{}

// ChainID is the chain ID string used by block-level mock helpers.
var ChainID = constants.ExampleChainID.ChainID

// Tx Search

func RegisterTxSearch(client *mocks.Client, query string, txBz []byte) {
	resulTxs := []*cmtrpctypes.ResultTx{{Tx: txBz}}
	client.On("TxSearch", rpc.ContextWithHeight(1), query, false, (*int)(nil), (*int)(nil), "").
		Return(&cmtrpctypes.ResultTxSearch{Txs: resulTxs, TotalCount: 1}, nil)
}

func RegisterTxSearchEmpty(client *mocks.Client, query string) {
	client.On("TxSearch", rpc.ContextWithHeight(1), query, false, (*int)(nil), (*int)(nil), "").
		Return(&cmtrpctypes.ResultTxSearch{}, nil)
}

// RegisterTxSearchWithResult registers a TxSearch mock that returns a single ResultTx
// with explicit raw tx bytes (nil for derived txs with no Cosmos envelope), block height,
// Cosmos-tx-slot index, and ABCI events. Needed when the KV indexer has no entry and
// code falls through to CometBFT TxSearch.
func RegisterTxSearchWithResult(
	client *mocks.Client,
	query string,
	height int64,
	txSlot uint32,
	txBz types.Tx,
	events []abci.Event,
) {
	resultTx := &cmtrpctypes.ResultTx{
		Height: height,
		Index:  txSlot,
		Tx:     txBz,
		TxResult: abci.ExecTxResult{
			Code:   0,
			Events: events,
		},
	}
	client.On("TxSearch", rpc.ContextWithHeight(1), query, false, (*int)(nil), (*int)(nil), "").
		Return(&cmtrpctypes.ResultTxSearch{Txs: []*cmtrpctypes.ResultTx{resultTx}, TotalCount: 1}, nil)
}

func RegisterTxSearchError(client *mocks.Client, query string) {
	client.On("TxSearch", rpc.ContextWithHeight(1), query, false, (*int)(nil), (*int)(nil), "").
		Return(nil, errortypes.ErrInvalidRequest)
}

// Block

func RegisterBlockMultipleTxs(
	client *mocks.Client,
	height int64,
	txs []types.Tx,
) (*cmtrpctypes.ResultBlock, error) {
	block := types.MakeBlock(height, txs, nil, nil)
	block.ChainID = ChainID
	resBlock := &cmtrpctypes.ResultBlock{Block: block}
	client.On("Block", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).Return(resBlock, nil)
	return resBlock, nil
}

func RegisterBlock(
	client *mocks.Client,
	height int64,
	tx []byte,
) (*cmtrpctypes.ResultBlock, error) {
	if tx == nil {
		emptyBlock := types.MakeBlock(height, []types.Tx{}, nil, nil)
		emptyBlock.ChainID = ChainID
		resBlock := &cmtrpctypes.ResultBlock{Block: emptyBlock}
		client.On("Block", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).Return(resBlock, nil)
		return resBlock, nil
	}
	block := types.MakeBlock(height, []types.Tx{tx}, nil, nil)
	block.ChainID = ChainID
	resBlock := &cmtrpctypes.ResultBlock{Block: block}
	client.On("Block", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).Return(resBlock, nil)
	return resBlock, nil
}

func RegisterBlockError(client *mocks.Client, height int64) {
	client.On("Block", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).
		Return(nil, errortypes.ErrInvalidRequest)
}

func TestRegisterBlock(t *testing.T) {
	client := mocks.NewClient(t)
	height := rpc.BlockNumber(1).Int64()
	_, err := RegisterBlock(client, height, nil)
	require.NoError(t, err)

	res, err := client.Block(rpc.ContextWithHeight(height), &height)

	emptyBlock := types.MakeBlock(height, []types.Tx{}, nil, nil)
	emptyBlock.ChainID = ChainID
	resBlock := &cmtrpctypes.ResultBlock{Block: emptyBlock}
	require.Equal(t, resBlock, res)
	require.NoError(t, err)
}

// ConsensusParams

func RegisterConsensusParams(client *mocks.Client, height int64) {
	consensusParams := types.DefaultConsensusParams()
	client.On("ConsensusParams", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).
		Return(&cmtrpctypes.ResultConsensusParams{ConsensusParams: *consensusParams}, nil)
}

func RegisterConsensusParamsError(client *mocks.Client, height int64) {
	client.On("ConsensusParams", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).
		Return(nil, errortypes.ErrInvalidRequest)
}

func TestRegisterConsensusParams(t *testing.T) {
	client := mocks.NewClient(t)
	height := int64(1)
	RegisterConsensusParams(client, height)

	res, err := client.ConsensusParams(rpc.ContextWithHeight(height), &height)
	consensusParams := types.DefaultConsensusParams()
	require.Equal(t, &cmtrpctypes.ResultConsensusParams{ConsensusParams: *consensusParams}, res)
	require.NoError(t, err)
}

// BlockResults

func RegisterBlockResultsWithEventLog(client *mocks.Client, height int64) (*cmtrpctypes.ResultBlockResults, error) {
	res := &cmtrpctypes.ResultBlockResults{
		Height: height,
		TxsResults: []*abci.ExecTxResult{
			{Code: 0, GasUsed: 0, Events: []abci.Event{{
				Type: evmtypes.EventTypeTxLog,
				Attributes: []abci.EventAttribute{{
					Key:   evmtypes.AttributeKeyTxLog,
					Value: "{\"test\": \"hello\"}",
					Index: true,
				}},
			}}},
		},
	}
	client.On("BlockResults", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).
		Return(res, nil)
	return res, nil
}

func RegisterBlockResults(
	client *mocks.Client,
	height int64,
) (*cmtrpctypes.ResultBlockResults, error) {
	res := &cmtrpctypes.ResultBlockResults{
		Height:     height,
		TxsResults: []*abci.ExecTxResult{{Code: 0, GasUsed: 0}},
	}
	client.On("BlockResults", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).
		Return(res, nil)
	return res, nil
}

// RegisterBlockResultsWithTxs registers a BlockResults mock with custom per-slot ABCI
// results. Required when the after-loop derived-tx section in TraceTransaction fetches
// BlockResults to scan for intra-slot derived-tx predecessor ordering.
func RegisterBlockResultsWithTxs(
	client *mocks.Client,
	height int64,
	txResults []*abci.ExecTxResult,
) *cmtrpctypes.ResultBlockResults {
	res := &cmtrpctypes.ResultBlockResults{
		Height:     height,
		TxsResults: txResults,
	}
	client.On("BlockResults", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).
		Return(res, nil)
	return res
}

func RegisterBlockResultsError(client *mocks.Client, height int64) {
	client.On("BlockResults", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).
		Return(nil, errortypes.ErrInvalidRequest)
}

func TestRegisterBlockResults(t *testing.T) {
	client := mocks.NewClient(t)
	height := int64(1)
	_, err := RegisterBlockResults(client, height)
	require.NoError(t, err)

	res, err := client.BlockResults(rpc.ContextWithHeight(height), &height)
	expRes := &cmtrpctypes.ResultBlockResults{
		Height:     height,
		TxsResults: []*abci.ExecTxResult{{Code: 0, GasUsed: 0}},
	}
	require.Equal(t, expRes, res)
	require.NoError(t, err)
}

// BlockByHash

func RegisterBlockByHash(
	client *mocks.Client,
	_ common.Hash,
	tx []byte,
) (*cmtrpctypes.ResultBlock, error) {
	block := types.MakeBlock(1, []types.Tx{tx}, nil, nil)
	resBlock := &cmtrpctypes.ResultBlock{Block: block}
	client.On("BlockByHash", rpc.ContextWithHeight(1), []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}).
		Return(resBlock, nil)
	return resBlock, nil
}

func RegisterBlockByHashError(client *mocks.Client, _ common.Hash, _ []byte) {
	client.On("BlockByHash", rpc.ContextWithHeight(1), []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}).
		Return(nil, errortypes.ErrInvalidRequest)
}

// HeaderByHash

func RegisterHeaderByHash(
	client *mocks.Client,
	_ common.Hash,
	_ []byte,
) (*cmtrpctypes.ResultHeader, error) {
	header := &types.Header{
		Version: cmtversion.Consensus{Block: version.BlockProtocol, App: 0},
		Height:  1,
	}
	resHeader := &cmtrpctypes.ResultHeader{Header: header}
	client.On("HeaderByHash", rpc.ContextWithHeight(1), bytes.HexBytes{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}).
		Return(resHeader, nil)
	return resHeader, nil
}

func RegisterHeaderByHashError(client *mocks.Client, _ common.Hash, _ []byte) {
	client.On("HeaderByHash", rpc.ContextWithHeight(1), bytes.HexBytes{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}).
		Return(nil, errortypes.ErrInvalidRequest)
}

// ABCIQuery

func RegisterABCIQueryWithOptions(client *mocks.Client, height int64, path string, data bytes.HexBytes, opts cmtrpcclient.ABCIQueryOptions) {
	client.On("ABCIQueryWithOptions", context.Background(), path, data, opts).
		Return(&cmtrpctypes.ResultABCIQuery{
			Response: abci.ResponseQuery{
				Value:  []byte{2},
				Height: height,
			},
		}, nil)
}

func RegisterABCIQueryWithOptionsError(clients *mocks.Client, path string, data bytes.HexBytes, opts cmtrpcclient.ABCIQueryOptions) {
	clients.On("ABCIQueryWithOptions", context.Background(), path, data, opts).
		Return(nil, errortypes.ErrInvalidRequest)
}

func RegisterABCIQueryAccount(clients *mocks.Client, data bytes.HexBytes, opts cmtrpcclient.ABCIQueryOptions, acc client.Account) {
	baseAccount := authtypes.NewBaseAccount(acc.GetAddress(), acc.GetPubKey(), acc.GetAccountNumber(), acc.GetSequence())
	accAny, _ := codectypes.NewAnyWithValue(baseAccount)
	accResponse := authtypes.QueryAccountResponse{Account: accAny}
	respBz, _ := accResponse.Marshal()
	clients.On("ABCIQueryWithOptions", context.Background(), "/cosmos.auth.v1beta1.Query/Account", data, opts).
		Return(&cmtrpctypes.ResultABCIQuery{
			Response: abci.ResponseQuery{
				Value:  respBz,
				Height: 1,
			},
		}, nil)
}

// EVM query client helpers for tracing tests

func RegisterTraceTransactionWithPredecessors(queryClient *mocks.EVMQueryClient, _ *evmtypes.MsgEthereumTx, _ []*evmtypes.MsgEthereumTx) {
	data, _ := json.Marshal(map[string]interface{}{"test": "hello"})
	queryClient.On("TraceTx", rpc.ContextWithHeight(1), mock.Anything).
		Return(&evmtypes.QueryTraceTxResponse{Data: data}, nil)
}

func RegisterTraceTransaction(queryClient *mocks.EVMQueryClient, msgEthTx *evmtypes.MsgEthereumTx) {
	RegisterTraceTransactionWithPredecessors(queryClient, msgEthTx, nil)
}
