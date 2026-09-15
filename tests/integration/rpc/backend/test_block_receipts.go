package backend

import (
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/mock"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/evm/rpc/backend/mocks"
	rpctypes "github.com/cosmos/evm/rpc/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/gogoproto/proto"

	"cosmossdk.io/math"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"google.golang.org/grpc/metadata"
)

// TestGetBlockReceiptsLatest checks that eth_getBlockReceipts("latest")
// fetches the block results of the exact block it resolved, even if CometBFT
// already committed a newer block in between.
func (s *TestSuite) TestGetBlockReceiptsLatest() {
	s.SetupTest()

	height := int64(10)
	latest := rpctypes.EthLatestBlockNumber
	msgEthereumTx, _ := s.buildEthereumTx()
	signedBz := s.signAndEncodeEthTx(msgEthereumTx)
	txHash := msgEthereumTx.AsTransaction().Hash()

	client := s.backend.ClientCtx.Client.(*mocks.Client)
	queryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)

	// eth_blockNumber resolves "latest" to height 10
	var header metadata.MD
	RegisterParams(queryClient, &header, height)

	// block 10 holds one eth tx
	block := cmttypes.MakeBlock(height, []cmttypes.Tx{signedBz}, nil, nil)
	block.ChainID = ChainID.ChainID
	resBlock := &cmtrpctypes.ResultBlock{Block: block}
	client.EXPECT().Block(mock.Anything, mock.MatchedBy(func(h *int64) bool { return h != nil && *h == height })).
		Return(resBlock, nil)

	// results of block 10
	anyResp, err := codectypes.NewAnyWithValue(&evmtypes.MsgEthereumTxResponse{Hash: txHash.Hex()})
	s.Require().NoError(err)
	txData, err := proto.Marshal(&sdk.TxMsgData{MsgResponses: []*codectypes.Any{anyResp}})
	s.Require().NoError(err)
	blockRes := &cmtrpctypes.ResultBlockResults{
		Height: height,
		TxsResults: []*abci.ExecTxResult{{
			Code:    0,
			GasUsed: 21000,
			Data:    txData,
			Events: []abci.Event{{
				Type: evmtypes.EventTypeEthereumTx,
				Attributes: []abci.EventAttribute{
					{Key: evmtypes.AttributeKeyEthereumTxHash, Value: txHash.Hex()},
					{Key: evmtypes.AttributeKeyTxIndex, Value: "0"},
					{Key: evmtypes.AttributeKeyTxGasUsed, Value: "21000"},
				},
			}},
		}},
	}
	client.EXPECT().BlockResults(mock.Anything, mock.MatchedBy(func(h *int64) bool { return h != nil && *h == height })).
		Return(blockRes, nil)

	// "latest" block results: an empty block 11 was committed meanwhile. The
	// backend must not ask for these when serving receipts of block 10.
	client.EXPECT().BlockResults(mock.Anything, mock.MatchedBy(func(h *int64) bool { return h == nil })).
		Return(&cmtrpctypes.ResultBlockResults{Height: height + 1, TxsResults: []*abci.ExecTxResult{}}, nil).
		Maybe()

	s.Require().NoError(s.backend.Indexer.IndexBlock(resBlock.Block, blockRes.TxsResults))
	RegisterBaseFee(queryClient, math.NewInt(1))

	receipts, err := s.backend.GetBlockReceipts(s.Ctx(), rpctypes.BlockNumberOrHash{BlockNumber: &latest})
	s.Require().NoError(err)
	s.Require().Len(receipts, 1)
	s.Require().Equal(txHash, receipts[0]["transactionHash"])
	s.Require().Equal(hexutil.Uint64(height), receipts[0]["blockNumber"])
	s.Require().Equal(hexutil.Uint64(21000), receipts[0]["gasUsed"])
	client.AssertNotCalled(s.T(), "BlockResults", mock.Anything, (*int64)(nil))
}
