package backend

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/libs/bytes"
	cmtversion "github.com/cometbft/cometbft/proto/tendermint/version"
	cmtrpcclient "github.com/cometbft/cometbft/rpc/client"
	cmtrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/types"
	"github.com/cometbft/cometbft/version"

	"github.com/cosmos/evm/rpc/backend/mocks"
	rpc "github.com/cosmos/evm/rpc/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// Client defines a mocked object that implements the CometBFT JSON-RPC Client
// interface. It allows for performing Client queries without having to run a
// CometBFT RPC Client server.
//
// To use a mock method it has to be registered in a given test.
var _ cmtrpcclient.Client = &mocks.Client{}

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

func RegisterTxSearchError(client *mocks.Client, query string) {
	client.On("TxSearch", rpc.ContextWithHeight(1), query, false, (*int)(nil), (*int)(nil), "").
		Return(nil, errortypes.ErrInvalidRequest)
}

// Broadcast Tx

func RegisterBroadcastTx(client *mocks.Client, tx types.Tx) {
	client.On("BroadcastTxSync", context.Background(), tx).
		Return(&cmtrpctypes.ResultBroadcastTx{}, nil)
}

func RegisterBroadcastTxError(client *mocks.Client, tx types.Tx) {
	client.On("BroadcastTxSync", context.Background(), tx).
		Return(nil, errortypes.ErrInvalidRequest)
}

// Unconfirmed Transactions

func RegisterUnconfirmedTxs(client *mocks.Client, limit *int, txs []types.Tx) {
	client.On("UnconfirmedTxs", rpc.ContextWithHeight(1), limit).
		Return(&cmtrpctypes.ResultUnconfirmedTxs{Txs: txs}, nil)
}

func RegisterUnconfirmedTxsEmpty(client *mocks.Client, limit *int) {
	client.On("UnconfirmedTxs", rpc.ContextWithHeight(1), limit).
		Return(&cmtrpctypes.ResultUnconfirmedTxs{
			Txs: make([]types.Tx, 2),
		}, nil)
}

func RegisterUnconfirmedTxsError(client *mocks.Client, limit *int) {
	client.On("UnconfirmedTxs", rpc.ContextWithHeight(1), limit).
		Return(nil, errortypes.ErrInvalidRequest)
}

// Status

func RegisterStatus(client *mocks.Client) {
	client.On("Status", rpc.ContextWithHeight(1)).
		Return(&cmtrpctypes.ResultStatus{}, nil)
}

func RegisterStatusError(client *mocks.Client) {
	client.On("Status", rpc.ContextWithHeight(1)).
		Return(nil, errortypes.ErrInvalidRequest)
}

// Block

func RegisterBlockMultipleTxs(
	client *mocks.Client,
	height int64,
	txs []types.Tx,
) *cmtrpctypes.ResultBlock {
	block := types.MakeBlock(height, txs, nil, nil)
	block.ChainID = ChainID.ChainID
	resBlock := &cmtrpctypes.ResultBlock{Block: block}
	client.On("Block", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).Return(resBlock, nil)
	return resBlock
}

func RegisterBlock(
	client *mocks.Client,
	height int64,
	tx []byte,
) *cmtrpctypes.ResultBlock {
	// without tx
	if tx == nil {
		emptyBlock := types.MakeBlock(height, []types.Tx{}, nil, nil)
		emptyBlock.ChainID = ChainID.ChainID
		resBlock := &cmtrpctypes.ResultBlock{Block: emptyBlock}
		client.On("Block", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).Return(resBlock, nil)
		return resBlock
	}

	// with tx
	block := types.MakeBlock(height, []types.Tx{tx}, nil, nil)
	block.ChainID = ChainID.ChainID
	resBlock := &cmtrpctypes.ResultBlock{Block: block}
	client.On("Block", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).Return(resBlock, nil)
	return resBlock
}

// Block returns error

func RegisterBlockError(client *mocks.Client, height int64) {
	client.On("Block", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).
		Return(nil, errortypes.ErrInvalidRequest)
}

// Block not found

func RegisterBlockNotFound(
	client *mocks.Client,
	height int64,
) *cmtrpctypes.ResultBlock {
	client.On("Block", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).
		Return(&cmtrpctypes.ResultBlock{Block: nil}, nil)

	return &cmtrpctypes.ResultBlock{Block: nil}
}

// Block panic

func RegisterBlockPanic(client *mocks.Client, height int64) {
	client.On("Block", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).
		Return(func(context.Context, *int64) *cmtrpctypes.ResultBlock {
			panic("Block call panic")
		}, nil)
}

func TestRegisterBlock(t *testing.T) {
	client := mocks.NewClient(t)
	height := rpc.BlockNumber(1).Int64()
	RegisterBlock(client, height, nil)

	res, err := client.Block(rpc.ContextWithHeight(height), &height)

	emptyBlock := types.MakeBlock(height, []types.Tx{}, nil, nil)
	emptyBlock.ChainID = ChainID.ChainID
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
	anyValue, err := codectypes.NewAnyWithValue(&evmtypes.MsgEthereumTxResponse{
		Logs: []*evmtypes.Log{
			{Data: []byte("data")},
		},
	})
	if err != nil {
		return nil, err
	}
	data, err := proto.Marshal(&sdk.TxMsgData{MsgResponses: []*codectypes.Any{anyValue}})
	if err != nil {
		return nil, err
	}
	res := &cmtrpctypes.ResultBlockResults{
		Height: height,
		TxsResults: []*abci.ExecTxResult{
			{Code: 0, GasUsed: 0, Data: data},
		},
	}
	client.On("BlockResults", rpc.ContextWithHeight(height), mock.AnythingOfType("*int64")).
		Return(res, nil)
	return res, nil
}

func RegisterBlockResults(
	client *mocks.Client,
	height int64,
) *cmtrpctypes.ResultBlockResults {
	return RegisterBlockResultsWithTxs(client, height, []*abci.ExecTxResult{{Code: 0, GasUsed: 0}})
}

func RegisterBlockResultsWithTxs(
	client *mocks.Client,
	height int64,
	txsResults []*abci.ExecTxResult,
) *cmtrpctypes.ResultBlockResults {
	res := &cmtrpctypes.ResultBlockResults{
		Height:     height,
		TxsResults: txsResults,
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
	RegisterBlockResults(client, height)

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
) *cmtrpctypes.ResultBlock {
	block := types.MakeBlock(1, []types.Tx{tx}, nil, nil)
	resBlock := &cmtrpctypes.ResultBlock{Block: block}

	client.On("BlockByHash", rpc.ContextWithHeight(1), []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}).
		Return(resBlock, nil)
	return resBlock
}

func RegisterBlockByHashError(client *mocks.Client, _ common.Hash, _ []byte) {
	client.On("BlockByHash", rpc.ContextWithHeight(1), []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}).
		Return(nil, errortypes.ErrInvalidRequest)
}

func RegisterBlockByHashNotFound(client *mocks.Client, _ common.Hash, _ []byte) {
	client.On("BlockByHash", rpc.ContextWithHeight(1), []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}).
		Return(nil, nil)
}

// HeaderByHash

func RegisterHeaderByHash(
	client *mocks.Client,
	_ common.Hash,
	_ []byte,
) *cmtrpctypes.ResultHeader {
	header := &types.Header{
		Version: cmtversion.Consensus{Block: version.BlockProtocol, App: 0},
		Height:  1,
	}
	resHeader := &cmtrpctypes.ResultHeader{
		Header: header,
	}

	client.On("HeaderByHash", rpc.ContextWithHeight(1), bytes.HexBytes{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}).
		Return(resHeader, nil)
	return resHeader
}

func RegisterHeaderByHashError(client *mocks.Client, _ common.Hash, _ []byte) {
	client.On("HeaderByHash", rpc.ContextWithHeight(1), bytes.HexBytes{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}).
		Return(nil, errortypes.ErrInvalidRequest)
}

func RegisterHeaderByHashNotFound(client *mocks.Client, hash common.Hash, tx []byte) {
	client.On("HeaderByHash", rpc.ContextWithHeight(1), bytes.HexBytes(hash.Bytes())).
		Return(&coretypes.ResultHeader{Header: nil}, nil)
}

// Header

func RegisterHeader(client *mocks.Client, height *int64, tx []byte) *coretypes.ResultHeader {
	block := types.MakeBlock(*height, []types.Tx{tx}, nil, nil)
	resHeader := &coretypes.ResultHeader{Header: &block.Header}
	client.On("Header", rpc.ContextWithHeight(*height), mock.AnythingOfType("*int64")).Return(resHeader, nil)
	return resHeader
}

func RegisterHeaderError(client *mocks.Client, height *int64) {
	client.On("Header", rpc.ContextWithHeight(*height), height).Return(nil, errortypes.ErrInvalidRequest)
}

// Header not found

func RegisterHeaderNotFound(client *mocks.Client, height int64) {
	client.On("Header", rpc.ContextWithHeight(height), mock.MatchedBy(func(arg *int64) bool {
		return arg != nil && height == *arg
	})).Return(&coretypes.ResultHeader{Header: nil}, nil)
}

func RegisterABCIQueryWithOptions(client *mocks.Client, height int64, path string, data bytes.HexBytes, opts cmtrpcclient.ABCIQueryOptions) {
	client.On("ABCIQueryWithOptions", context.Background(), path, data, opts).
		Return(&cmtrpctypes.ResultABCIQuery{
			Response: abci.ResponseQuery{
				Value:  []byte{2}, // TODO replace with data.Bytes(),
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
