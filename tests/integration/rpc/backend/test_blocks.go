package backend

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/gogo/protobuf/proto"
	"google.golang.org/grpc/metadata"

	"github.com/cometbft/cometbft/abci/types"
	cmtrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/evm/rpc/backend/mocks"
	ethrpc "github.com/cosmos/evm/rpc/types"
	utiltx "github.com/cosmos/evm/testutil/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *TestSuite) TestBlockNumber() {
	testCases := []struct {
		name           string
		registerMock   func()
		expBlockNumber hexutil.Uint64
		expPass        bool
	}{
		{
			"fail - invalid block header height",
			func() {
				var header metadata.MD
				height := int64(1)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterParamsInvalidHeight(QueryClient, &header, height)
			},
			0x0,
			false,
		},
		{
			"fail - invalid block header",
			func() {
				var header metadata.MD
				height := int64(1)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterParamsInvalidHeader(QueryClient, &header, height)
			},
			0x0,
			false,
		},
		{
			"pass - app state header height 1",
			func() {
				var header metadata.MD
				height := int64(1)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterParams(QueryClient, &header, height)
			},
			0x1,
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset test and queries
			tc.registerMock()

			blockNumber, err := s.backend.BlockNumber()

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(tc.expBlockNumber, blockNumber)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestGetBlockByNumber() {
	var (
		blockRes *cmtrpctypes.ResultBlockResults
		resBlock *cmtrpctypes.ResultBlock
	)
	msgEthereumTx, _ := s.buildEthereumTx()
	// Produce a real Ethereum tx (with ExtensionOptions) for indexing-based lookups
	signedBz := s.signAndEncodeEthTx(msgEthereumTx)

	testCases := []struct {
		name         string
		blockNumber  ethrpc.BlockNumber
		fullTx       bool
		baseFee      *big.Int
		validator    sdk.AccAddress
		tx           *evmtypes.MsgEthereumTx
		txBz         []byte
		registerMock func(ethrpc.BlockNumber, math.Int, sdk.AccAddress, []byte)
		expNoop      bool
		expPass      bool
	}{
		{
			"pass - CometBFT block not found",
			ethrpc.BlockNumber(1),
			true,
			math.NewInt(1).BigInt(),
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			nil,
			nil,
			func(blockNum ethrpc.BlockNumber, _ math.Int, _ sdk.AccAddress, _ []byte) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockError(client, height)
			},
			true,
			true,
		},
		{
			"pass - block not found (e.g. request block height that is greater than current one)",
			ethrpc.BlockNumber(1),
			true,
			math.NewInt(1).BigInt(),
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			nil,
			nil,
			func(blockNum ethrpc.BlockNumber, _ math.Int, _ sdk.AccAddress, _ []byte) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resBlock = RegisterBlockNotFound(client, height)
			},
			true,
			true,
		},
		{
			"pass - block results error",
			ethrpc.BlockNumber(1),
			true,
			math.NewInt(1).BigInt(),
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			nil,
			nil,
			func(blockNum ethrpc.BlockNumber, _ math.Int, _ sdk.AccAddress, txBz []byte) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resBlock = RegisterBlock(client, height, txBz)
				RegisterBlockResultsError(client, blockNum.Int64())
			},
			true,
			true,
		},
		{
			"pass - without tx",
			ethrpc.BlockNumber(1),
			true,
			math.NewInt(1).BigInt(),
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			nil,
			nil,
			func(blockNum ethrpc.BlockNumber, baseFee math.Int, validator sdk.AccAddress, txBz []byte) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resBlock = RegisterBlock(client, height, txBz)
				blockRes = RegisterBlockResults(client, blockNum.Int64())
				RegisterConsensusParams(client, height)

				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterValidatorAccount(QueryClient, validator)
			},
			false,
			true,
		},
		{
			"pass - with tx",
			ethrpc.BlockNumber(1),
			true,
			math.NewInt(1).BigInt(),
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			msgEthereumTx,
			signedBz,
			func(blockNum ethrpc.BlockNumber, baseFee math.Int, validator sdk.AccAddress, txBz []byte) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resBlock = RegisterBlock(client, height, txBz)
				// Provide MsgEthereumTxResponse in Data for logs, and ethereum_tx events for indexer
				var err error
				blockRes, err = RegisterBlockResultsWithEventLog(client, height)
				s.Require().NoError(err)
				txHash := msgEthereumTx.AsTransaction().Hash()
				blockRes.TxsResults[0].Events = []types.Event{
					{Type: evmtypes.EventTypeEthereumTx, Attributes: []types.EventAttribute{
						{Key: evmtypes.AttributeKeyEthereumTxHash, Value: txHash.Hex()},
						{Key: evmtypes.AttributeKeyTxIndex, Value: "0"},
						{Key: evmtypes.AttributeKeyTxGasUsed, Value: "21000"},
					}},
				}
				// Index the block so GetTxByEthHash can find the tx when building receipts
				_ = s.backend.Indexer.IndexBlock(resBlock.Block, blockRes.TxsResults)
				RegisterConsensusParams(client, height)

				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterValidatorAccount(QueryClient, validator)
			},
			false,
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset test and queries
			tc.registerMock(tc.blockNumber, math.NewIntFromBigInt(tc.baseFee), tc.validator, tc.txBz)

			block, err := s.backend.GetBlockByNumber(tc.blockNumber, tc.fullTx)

			if tc.expPass {
				s.Require().NoError(err)
				if tc.expNoop {
					s.Require().Nil(block)
				} else {
					expBlock := s.buildFormattedBlock(
						blockRes,
						resBlock,
						tc.fullTx,
						tc.tx,
						tc.validator,
						tc.baseFee,
					)
					s.Require().Equal(expBlock, block)
				}
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestGetBlockByHash() {
	var (
		blockRes *cmtrpctypes.ResultBlockResults
		resBlock *cmtrpctypes.ResultBlock
	)
	msgEthereumTx, _ := s.buildEthereumTx()
	signedBz := s.signAndEncodeEthTx(msgEthereumTx)

	block := cmttypes.MakeBlock(1, []cmttypes.Tx{signedBz}, nil, nil)

	testCases := []struct {
		name         string
		hash         common.Hash
		fullTx       bool
		baseFee      *big.Int
		validator    sdk.AccAddress
		tx           *evmtypes.MsgEthereumTx
		txBz         []byte
		registerMock func(common.Hash, math.Int, sdk.AccAddress, []byte)
		expNoop      bool
		expPass      bool
	}{
		{
			"fail - CometBFT failed to get block",
			common.BytesToHash(block.Hash()),
			true,
			math.NewInt(1).BigInt(),
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			nil,
			nil,
			func(hash common.Hash, _ math.Int, _ sdk.AccAddress, txBz []byte) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockByHashError(client, hash, txBz)
			},
			false,
			false,
		},
		{
			"fail - CometBFT blockres not found",
			common.BytesToHash(block.Hash()),
			true,
			math.NewInt(1).BigInt(),
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			nil,
			nil,
			func(hash common.Hash, _ math.Int, _ sdk.AccAddress, txBz []byte) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockByHashNotFound(client, hash, txBz)
			},
			false,
			false,
		},
		{
			"noop - CometBFT failed to fetch block result",
			common.BytesToHash(block.Hash()),
			true,
			math.NewInt(1).BigInt(),
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			nil,
			nil,
			func(hash common.Hash, _ math.Int, _ sdk.AccAddress, txBz []byte) {
				height := int64(1)
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resBlock = RegisterBlockByHash(client, hash, txBz)

				RegisterBlockResultsError(client, height)
			},
			true,
			true,
		},
		{
			"pass - without tx",
			common.BytesToHash(block.Hash()),
			true,
			math.NewInt(1).BigInt(),
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			nil,
			nil,
			func(hash common.Hash, baseFee math.Int, validator sdk.AccAddress, txBz []byte) {
				height := int64(1)
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resBlock = RegisterBlockByHash(client, hash, txBz)
				blockRes = RegisterBlockResults(client, height)
				RegisterConsensusParams(client, height)

				err := s.backend.Indexer.IndexBlock(resBlock.Block, blockRes.TxsResults)
				s.Require().NoError(err)

				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterValidatorAccount(QueryClient, validator)
			},
			false,
			true,
		},
		{
			"pass - with tx",
			common.BytesToHash(block.Hash()),
			true,
			math.NewInt(1).BigInt(),
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			msgEthereumTx,
			signedBz,
			func(hash common.Hash, baseFee math.Int, validator sdk.AccAddress, txBz []byte) {
				height := int64(1)
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resBlock = RegisterBlockByHash(client, hash, txBz)

				// Provide MsgEthereumTxResponse in Data for logs, and ethereum_tx events for indexer
				var err error
				blockRes, err = RegisterBlockResultsWithEventLog(client, height)
				s.Require().NoError(err)
				txHash := msgEthereumTx.AsTransaction().Hash()
				blockRes.TxsResults[0].Events = []types.Event{
					{Type: evmtypes.EventTypeEthereumTx, Attributes: []types.EventAttribute{
						{Key: evmtypes.AttributeKeyEthereumTxHash, Value: txHash.Hex()},
						{Key: evmtypes.AttributeKeyTxIndex, Value: "0"},
						{Key: evmtypes.AttributeKeyTxGasUsed, Value: "21000"},
					}},
				}
				// Index the block so GetTxByEthHash can find the tx when building receipts
				err = s.backend.Indexer.IndexBlock(resBlock.Block, blockRes.TxsResults)
				s.Require().NoError(err)

				// blockRes = RegisterBlockResults(client, height)
				RegisterConsensusParams(client, height)

				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterValidatorAccount(QueryClient, validator)
			},
			false,
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset test and queries
			tc.registerMock(tc.hash, math.NewIntFromBigInt(tc.baseFee), tc.validator, tc.txBz)

			block, err := s.backend.GetBlockByHash(tc.hash, tc.fullTx)

			if tc.expPass {
				if tc.expNoop {
					s.Require().Nil(block)
				} else {
					expBlock := s.buildFormattedBlock(
						blockRes,
						resBlock,
						tc.fullTx,
						tc.tx,
						tc.validator,
						tc.baseFee,
					)
					s.Require().Equal(expBlock, block)
				}
				s.Require().NoError(err)

			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestGetBlockTransactionCountByHash() {
	ethereumTx, _ := s.buildEthereumTx()
	signedBz := s.signAndEncodeEthTx(ethereumTx)
	block := cmttypes.MakeBlock(1, []cmttypes.Tx{signedBz}, nil, nil)
	emptyBlock := cmttypes.MakeBlock(1, []cmttypes.Tx{}, nil, nil)

	testCases := []struct {
		name         string
		hash         common.Hash
		registerMock func(common.Hash)
		expCount     hexutil.Uint
		expPass      bool
	}{
		{
			"fail - header not found",
			common.BytesToHash(emptyBlock.Hash()),
			func(hash common.Hash) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockByHashError(client, hash, nil)
			},
			hexutil.Uint(0),
			false,
		},
		{
			"fail - CometBFT client failed to get block result",
			common.BytesToHash(emptyBlock.Hash()),
			func(hash common.Hash) {
				height := int64(1)
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockByHash(client, hash, nil)
				RegisterBlockResultsError(client, height)
			},
			hexutil.Uint(0),
			false,
		},
		{
			"pass - block without tx",
			common.BytesToHash(emptyBlock.Hash()),
			func(hash common.Hash) {
				height := int64(1)
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockByHash(client, hash, nil)
				RegisterBlockResults(client, height)
			},
			hexutil.Uint(0),
			true,
		},
		{
			"pass - block with tx",
			common.BytesToHash(block.Hash()),
			func(hash common.Hash) {
				height := int64(1)
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockByHash(client, hash, signedBz)
				RegisterBlockResults(client, height)
			},
			hexutil.Uint(1),
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset test and queries

			tc.registerMock(tc.hash)
			count := s.backend.GetBlockTransactionCountByHash(tc.hash)
			if tc.expPass {
				s.Require().Equal(tc.expCount, *count)
			} else {
				s.Require().Nil(count)
			}
		})
	}
}

func (s *TestSuite) TestGetBlockTransactionCountByNumber() {
	ethereumTx, _ := s.buildEthereumTx()
	signedBz := s.signAndEncodeEthTx(ethereumTx)
	block := cmttypes.MakeBlock(1, []cmttypes.Tx{signedBz}, nil, nil)
	emptyBlock := cmttypes.MakeBlock(1, []cmttypes.Tx{}, nil, nil)

	testCases := []struct {
		name         string
		blockNum     ethrpc.BlockNumber
		registerMock func(ethrpc.BlockNumber)
		expCount     hexutil.Uint
		expPass      bool
	}{
		{
			"fail - block not found",
			ethrpc.BlockNumber(emptyBlock.Height),
			func(blockNum ethrpc.BlockNumber) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockError(client, height)
			},
			hexutil.Uint(0),
			false,
		},
		{
			"fail - CometBFT client failed to get block result",
			ethrpc.BlockNumber(emptyBlock.Height),
			func(blockNum ethrpc.BlockNumber) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlock(client, height, nil)
				RegisterBlockResultsError(client, height)
			},
			hexutil.Uint(0),
			false,
		},
		{
			"pass - block without tx",
			ethrpc.BlockNumber(emptyBlock.Height),
			func(blockNum ethrpc.BlockNumber) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlock(client, height, nil)
				RegisterBlockResults(client, height)
			},
			hexutil.Uint(0),
			true,
		},
		{
			"pass - block with tx",
			ethrpc.BlockNumber(block.Height),
			func(blockNum ethrpc.BlockNumber) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlock(client, height, signedBz)
				RegisterBlockResults(client, height)
			},
			hexutil.Uint(1),
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset test and queries

			tc.registerMock(tc.blockNum)
			count := s.backend.GetBlockTransactionCountByNumber(tc.blockNum)
			if tc.expPass {
				s.Require().Equal(tc.expCount, *count)
			} else {
				s.Require().Nil(count)
			}
		})
	}
}

func (s *TestSuite) TestCometBlockByNumber() {
	var expResultHeader *cmtrpctypes.ResultBlock

	testCases := []struct {
		name         string
		blockNumber  ethrpc.BlockNumber
		registerMock func(ethrpc.BlockNumber)
		found        bool
		expPass      bool
	}{
		{
			"fail - client error",
			ethrpc.BlockNumber(1),
			func(blockNum ethrpc.BlockNumber) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockError(client, height)
			},
			false,
			false,
		},
		{
			"noop - header not found",
			ethrpc.BlockNumber(1),
			func(blockNum ethrpc.BlockNumber) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockNotFound(client, height)
			},
			false,
			true,
		},
		{
			"fail - blockNum < 0 with app state height error",
			ethrpc.BlockNumber(-1),
			func(_ ethrpc.BlockNumber) {
				var header metadata.MD
				appHeight := int64(1)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterParamsError(QueryClient, &header, appHeight)
			},
			false,
			false,
		},
		{
			"pass - blockNum < 0 with app state height >= 1",
			ethrpc.BlockNumber(-1),
			func(ethrpc.BlockNumber) {
				var header metadata.MD
				appHeight := int64(1)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterParams(QueryClient, &header, appHeight)

				tmHeight := appHeight
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				expResultHeader = RegisterBlock(client, tmHeight, nil)
			},
			true,
			true,
		},
		{
			"pass - blockNum = 0 (defaults to blockNum = 1 due to a difference between CometBFT heights and geth heights)",
			ethrpc.BlockNumber(0),
			func(blockNum ethrpc.BlockNumber) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				expResultHeader = RegisterBlock(client, height, nil)
			},
			true,
			true,
		},
		{
			"pass - blockNum = 1",
			ethrpc.BlockNumber(1),
			func(blockNum ethrpc.BlockNumber) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				expResultHeader = RegisterBlock(client, height, nil)
			},
			true,
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset test and queries

			tc.registerMock(tc.blockNumber)
			resultBlock, err := s.backend.CometBlockByNumber(tc.blockNumber)

			if tc.expPass {
				s.Require().NoError(err)

				if !tc.found {
					s.Require().Nil(resultBlock)
				} else {
					s.Require().Equal(expResultHeader, resultBlock)
					s.Require().Equal(expResultHeader.Block.Height, resultBlock.Block.Height)
				}
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestCometBlockResultByNumber() {
	var expBlockRes *cmtrpctypes.ResultBlockResults

	testCases := []struct {
		name         string
		blockNumber  int64
		registerMock func(int64)
		expPass      bool
	}{
		{
			"fail",
			1,
			func(blockNum int64) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockResultsError(client, blockNum)
			},
			false,
		},
		{
			"pass",
			1,
			func(blockNum int64) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockResults(client, blockNum)
				expBlockRes = &cmtrpctypes.ResultBlockResults{
					Height:     blockNum,
					TxsResults: []*types.ExecTxResult{{Code: 0, GasUsed: 0}},
				}
			},
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset test and queries
			tc.registerMock(tc.blockNumber)

			client := s.backend.ClientCtx.Client.(*mocks.Client)
			blockRes, err := client.BlockResults(s.backend.Ctx, &tc.blockNumber) //#nosec G601 -- fine for tests

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(expBlockRes, blockRes)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestBlockNumberFromComet() {
	var resHeader *cmtrpctypes.ResultHeader

	_, bz := s.buildEthereumTx()
	block := cmttypes.MakeBlock(1, []cmttypes.Tx{bz}, nil, nil)
	blockNum := ethrpc.NewBlockNumber(big.NewInt(block.Height))
	blockHash := common.BytesToHash(block.Hash())

	testCases := []struct {
		name         string
		blockNum     *ethrpc.BlockNumber
		hash         *common.Hash
		registerMock func(*common.Hash)
		expPass      bool
	}{
		{
			"error - without blockHash or blockNum",
			nil,
			nil,
			func(*common.Hash) {},
			false,
		},
		{
			"error - with blockHash, CometBFT client failed to get block",
			nil,
			&blockHash,
			func(hash *common.Hash) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterHeaderByHashError(client, *hash, bz)
			},
			false,
		},
		{
			"pass - with blockHash",
			nil,
			&blockHash,
			func(hash *common.Hash) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resHeader = RegisterHeaderByHash(client, *hash, bz)
			},
			true,
		},
		{
			"pass - without blockHash & with blockNumber",
			&blockNum,
			nil,
			func(*common.Hash) {},
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset test and queries

			blockNrOrHash := ethrpc.BlockNumberOrHash{
				BlockNumber: tc.blockNum,
				BlockHash:   tc.hash,
			}

			tc.registerMock(tc.hash)
			blockNum, err := s.backend.BlockNumberFromComet(blockNrOrHash)

			if tc.expPass {
				s.Require().NoError(err)
				if tc.hash == nil {
					s.Require().Equal(*tc.blockNum, blockNum)
				} else {
					expHeight := ethrpc.NewBlockNumber(big.NewInt(resHeader.Header.Height))
					s.Require().Equal(expHeight, blockNum)
				}
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestBlockNumberFromCometByHash() {
	var resHeader *cmtrpctypes.ResultHeader

	_, bz := s.buildEthereumTx()
	block := cmttypes.MakeBlock(1, []cmttypes.Tx{bz}, nil, nil)
	emptyBlock := cmttypes.MakeBlock(1, []cmttypes.Tx{}, nil, nil)

	testCases := []struct {
		name         string
		hash         common.Hash
		registerMock func(common.Hash)
		expPass      bool
	}{
		{
			"fail - CometBFT client failed to get block",
			common.BytesToHash(block.Hash()),
			func(hash common.Hash) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterHeaderByHashError(client, hash, bz)
			},
			false,
		},
		{
			"pass - block without tx",
			common.BytesToHash(emptyBlock.Hash()),
			func(hash common.Hash) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resHeader = RegisterHeaderByHash(client, hash, bz)
			},
			true,
		},
		{
			"pass - block with tx",
			common.BytesToHash(block.Hash()),
			func(hash common.Hash) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resHeader = RegisterHeaderByHash(client, hash, bz)
			},
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset test and queries

			tc.registerMock(tc.hash)
			blockNum, err := s.backend.BlockNumberFromCometByHash(tc.hash)
			if tc.expPass {
				expHeight := big.NewInt(resHeader.Header.Height)
				s.Require().NoError(err)
				s.Require().Equal(expHeight, blockNum)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestBlockBloomFromCometBlock() {
	testCases := []struct {
		name          string
		blockRes      *cmtrpctypes.ResultBlockResults
		expBlockBloom ethtypes.Bloom
		expPass       bool
	}{
		{
			"fail - empty block result",
			&cmtrpctypes.ResultBlockResults{},
			ethtypes.Bloom{},
			false,
		},
		{
			"fail - non block bloom event type",
			&cmtrpctypes.ResultBlockResults{
				FinalizeBlockEvents: []types.Event{{Type: evmtypes.EventTypeEthereumTx}},
			},
			ethtypes.Bloom{},
			false,
		},
		{
			"fail - nonblock bloom attribute key",
			&cmtrpctypes.ResultBlockResults{
				FinalizeBlockEvents: []types.Event{
					{
						Type: evmtypes.EventTypeBlockBloom,
						Attributes: []types.EventAttribute{
							{Key: evmtypes.AttributeKeyEthereumTxHash},
						},
					},
				},
			},
			ethtypes.Bloom{},
			false,
		},
		{
			"pass - block bloom attribute key",
			&cmtrpctypes.ResultBlockResults{
				FinalizeBlockEvents: []types.Event{
					{
						Type: evmtypes.EventTypeBlockBloom,
						Attributes: []types.EventAttribute{
							{Key: evmtypes.AttributeKeyEthereumBloom},
						},
					},
				},
			},
			ethtypes.Bloom{},
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			blockBloom, err := s.backend.BlockBloomFromCometBlock(tc.blockRes)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(tc.expBlockBloom, blockBloom)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestGetEthBlockFromComet() {
	msgEthereumTx, _ := s.buildEthereumTx()
	signedBz := s.signAndEncodeEthTx(msgEthereumTx)
	txHash := msgEthereumTx.AsTransaction().Hash()
	emptyBlock := cmttypes.MakeBlock(1, []cmttypes.Tx{}, nil, nil)

	anyValue, err := codectypes.NewAnyWithValue(&evmtypes.MsgEthereumTxResponse{
		Logs: []*evmtypes.Log{
			{Data: []byte("data")},
		},
	})
	s.Require().NoError(err)
	data, err := proto.Marshal(&sdk.TxMsgData{MsgResponses: []*codectypes.Any{anyValue}})
	s.Require().NoError(err)
	blockRes := &cmtrpctypes.ResultBlockResults{
		Height: 1,
		TxsResults: []*types.ExecTxResult{
			{
				Code:    0,
				GasUsed: 0,
				Data:    data,
				Events: []types.Event{
					{
						Type: evmtypes.EventTypeEthereumTx, Attributes: []types.EventAttribute{
							{Key: evmtypes.AttributeKeyEthereumTxHash, Value: txHash.Hex()},
							{Key: evmtypes.AttributeKeyTxIndex, Value: "0"},
							{Key: evmtypes.AttributeKeyTxGasUsed, Value: "21000"},
						},
					},
				},
			},
		},
	}

	testCases := []struct {
		name         string
		baseFee      *big.Int
		validator    sdk.AccAddress
		height       int64
		resBlock     *cmtrpctypes.ResultBlock
		blockRes     *cmtrpctypes.ResultBlockResults
		fullTx       bool
		registerMock func(math.Int, sdk.AccAddress, int64)
		expTxs       bool
		expPass      bool
	}{
		{
			"pass - block without tx",
			math.NewInt(1).BigInt(),
			sdk.AccAddress(common.Address{}.Bytes()),
			int64(1),
			&cmtrpctypes.ResultBlock{Block: emptyBlock},
			blockRes,
			false,
			func(baseFee math.Int, validator sdk.AccAddress, height int64) {
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterValidatorAccount(QueryClient, validator)

				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterConsensusParams(client, height)
			},
			false,
			true,
		},
		{
			"pass - block with tx - with BaseFee error",
			nil,
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			int64(1),
			&cmtrpctypes.ResultBlock{
				Block: cmttypes.MakeBlock(1, []cmttypes.Tx{signedBz}, nil, nil),
			},
			blockRes,
			true,
			func(_ math.Int, validator sdk.AccAddress, height int64) {
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFeeError(QueryClient)
				RegisterValidatorAccount(QueryClient, validator)

				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterConsensusParams(client, height)
			},
			true,
			true,
		},
		{
			"pass - block with tx - with ValidatorAccount error",
			math.NewInt(1).BigInt(),
			sdk.AccAddress(common.Address{}.Bytes()),
			int64(1),
			&cmtrpctypes.ResultBlock{
				Block: cmttypes.MakeBlock(1, []cmttypes.Tx{signedBz}, nil, nil),
			},
			blockRes,
			true,
			func(baseFee math.Int, _ sdk.AccAddress, height int64) {
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterValidatorAccountError(QueryClient)

				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterConsensusParams(client, height)
			},
			true,
			true,
		},
		{
			"pass - block with tx - with ConsensusParams error - BlockMaxGas defaults to max uint32",
			math.NewInt(1).BigInt(),
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			int64(1),
			&cmtrpctypes.ResultBlock{
				Block: cmttypes.MakeBlock(1, []cmttypes.Tx{signedBz}, nil, nil),
			},
			blockRes,
			true,
			func(baseFee math.Int, validator sdk.AccAddress, height int64) {
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterValidatorAccount(QueryClient, validator)

				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterConsensusParamsError(client, height)
			},
			true,
			true,
		},
		{
			"pass - block with tx - with ShouldIgnoreGasUsed - empty txs",
			math.NewInt(1).BigInt(),
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			int64(1),
			&cmtrpctypes.ResultBlock{
				Block: cmttypes.MakeBlock(1, []cmttypes.Tx{signedBz}, nil, nil),
			},
			&cmtrpctypes.ResultBlockResults{
				Height: 1,
				TxsResults: []*types.ExecTxResult{
					{
						Code:    11,
						GasUsed: 0,
						Log:     "no block gas left to run tx: out of gas",
					},
				},
			},
			true,
			func(baseFee math.Int, validator sdk.AccAddress, height int64) {
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterValidatorAccount(QueryClient, validator)

				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterConsensusParams(client, height)
			},
			false,
			true,
		},
		{
			"pass - block with tx - non fullTx",
			math.NewInt(1).BigInt(),
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			int64(1),
			&cmtrpctypes.ResultBlock{
				Block: cmttypes.MakeBlock(1, []cmttypes.Tx{signedBz}, nil, nil),
			},
			blockRes,
			false,
			func(baseFee math.Int, validator sdk.AccAddress, height int64) {
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterValidatorAccount(QueryClient, validator)

				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterConsensusParams(client, height)
			},
			true,
			true,
		},
		{
			"pass - block with tx",
			math.NewInt(1).BigInt(),
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			int64(1),
			&cmtrpctypes.ResultBlock{
				Block: cmttypes.MakeBlock(1, []cmttypes.Tx{signedBz}, nil, nil),
			},
			blockRes,
			true,
			func(baseFee math.Int, validator sdk.AccAddress, height int64) {
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterValidatorAccount(QueryClient, validator)

				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterConsensusParams(client, height)
			},
			true,
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset test and queries
			tc.registerMock(math.NewIntFromBigInt(tc.baseFee), tc.validator, tc.height)

			if len(tc.resBlock.Block.Txs) > 0 && len(tc.blockRes.TxsResults) > 0 {
				err := s.backend.Indexer.IndexBlock(tc.resBlock.Block, tc.blockRes.TxsResults)
				s.Require().NoError(err)
			}
			block, err := s.backend.RPCBlockFromCometBlock(tc.resBlock, tc.blockRes, tc.fullTx)

			var tx *evmtypes.MsgEthereumTx
			if tc.expTxs {
				tx = msgEthereumTx
			}
			expBlock := s.buildFormattedBlock(tc.blockRes, tc.resBlock, tc.fullTx, tx, tc.validator, tc.baseFee)

			if tc.expPass {
				s.Require().Equal(expBlock, block)
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestEthMsgsFromCometBlock() {
	msgEthereumTx, bz := s.buildEthereumTx()

	testCases := []struct {
		name     string
		resBlock *cmtrpctypes.ResultBlock
		blockRes *cmtrpctypes.ResultBlockResults
		expMsgs  []*evmtypes.MsgEthereumTx
	}{
		{
			"tx in not included in block - unsuccessful tx without ExceedBlockGasLimit error",
			&cmtrpctypes.ResultBlock{
				Block: cmttypes.MakeBlock(1, []cmttypes.Tx{bz}, nil, nil),
			},
			&cmtrpctypes.ResultBlockResults{
				TxsResults: []*types.ExecTxResult{
					{
						Code: 1,
					},
				},
			},
			[]*evmtypes.MsgEthereumTx(nil),
		},
		{
			"tx included in block - unsuccessful tx with ExceedBlockGasLimit error",
			&cmtrpctypes.ResultBlock{
				Block: cmttypes.MakeBlock(1, []cmttypes.Tx{bz}, nil, nil),
			},
			&cmtrpctypes.ResultBlockResults{
				TxsResults: []*types.ExecTxResult{
					{
						Code: 1,
						Log:  ethrpc.ExceedBlockGasLimitError,
					},
				},
			},
			[]*evmtypes.MsgEthereumTx{msgEthereumTx},
		},
		{
			"pass",
			&cmtrpctypes.ResultBlock{
				Block: cmttypes.MakeBlock(1, []cmttypes.Tx{bz}, nil, nil),
			},
			&cmtrpctypes.ResultBlockResults{
				TxsResults: []*types.ExecTxResult{
					{
						Code: 0,
						Log:  ethrpc.ExceedBlockGasLimitError,
					},
				},
			},
			[]*evmtypes.MsgEthereumTx{msgEthereumTx},
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset test and queries

			msgs := s.backend.EthMsgsFromCometBlock(tc.resBlock, tc.blockRes)
			for i, expMsg := range tc.expMsgs {
				expBytes, err := json.Marshal(expMsg)
				s.Require().Nil(err)
				bytes, err := json.Marshal(msgs[i])
				s.Require().Nil(err)
				s.Require().Equal(expBytes, bytes)
			}
		})
	}
}

func (s *TestSuite) TestHeaderByNumber() {
	var (
		blockRes *cmtrpctypes.ResultBlockResults
		resBlock *cmtrpctypes.ResultBlock
	)

	msgEthereumTx, _ := s.buildEthereumTx()
	signedBz := s.signAndEncodeEthTx(msgEthereumTx)
	validator := sdk.AccAddress(utiltx.GenerateAddress().Bytes())

	// Imports needed for added mocks
	// Note: file already imports these at top; ensure present

	testCases := []struct {
		name         string
		blockNumber  ethrpc.BlockNumber
		baseFee      *big.Int
		registerMock func(ethrpc.BlockNumber, math.Int)
		expPass      bool
	}{
		{
			"fail - CometBFT client failed to get block",
			ethrpc.BlockNumber(1),
			math.NewInt(1).BigInt(),
			func(blockNum ethrpc.BlockNumber, _ math.Int) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockError(client, height)
			},
			false,
		},
		{
			"fail - header not found for height",
			ethrpc.BlockNumber(1),
			math.NewInt(1).BigInt(),
			func(blockNum ethrpc.BlockNumber, _ math.Int) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlock(client, height, nil)
				RegisterBlockResultsError(client, height)
			},
			false,
		},
		{
			"pass - without Base Fee, failed to fetch from prunned block",
			ethrpc.BlockNumber(1),
			nil,
			func(blockNum ethrpc.BlockNumber, _ math.Int) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resBlock = RegisterBlock(client, height, nil)
				var err error
				blockRes, err = RegisterBlockResultsWithEventLog(client, height)
				s.Require().NoError(err)
				txHash := msgEthereumTx.AsTransaction().Hash()
				blockRes.TxsResults[0].Events = []types.Event{
					{Type: evmtypes.EventTypeEthereumTx, Attributes: []types.EventAttribute{
						{Key: evmtypes.AttributeKeyEthereumTxHash, Value: txHash.Hex()},
						{Key: evmtypes.AttributeKeyTxIndex, Value: "0"},
						{Key: evmtypes.AttributeKeyTxGasUsed, Value: "21000"},
					}},
				}

				RegisterBlockResults(client, height)
				RegisterConsensusParams(client, height)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFeeError(QueryClient)
				RegisterValidatorAccount(QueryClient, validator)
			},
			true,
		},
		{
			"pass - blockNum = 1, without tx",
			ethrpc.BlockNumber(1),
			math.NewInt(1).BigInt(),
			func(blockNum ethrpc.BlockNumber, baseFee math.Int) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resBlock = RegisterBlock(client, height, nil)
				var err error
				blockRes, err = RegisterBlockResultsWithEventLog(client, height)
				s.Require().NoError(err)
				txHash := msgEthereumTx.AsTransaction().Hash()
				blockRes.TxsResults[0].Events = []types.Event{
					{Type: evmtypes.EventTypeEthereumTx, Attributes: []types.EventAttribute{
						{Key: evmtypes.AttributeKeyEthereumTxHash, Value: txHash.Hex()},
						{Key: evmtypes.AttributeKeyTxIndex, Value: "0"},
						{Key: evmtypes.AttributeKeyTxGasUsed, Value: "21000"},
					}},
				}

				RegisterBlockResults(client, height)
				RegisterConsensusParams(client, height)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterValidatorAccount(QueryClient, validator)
			},
			true,
		},
		{
			"pass - blockNum = 1, with tx",
			ethrpc.BlockNumber(1),
			math.NewInt(1).BigInt(),
			func(blockNum ethrpc.BlockNumber, baseFee math.Int) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resBlock = RegisterBlock(client, height, signedBz)

				var err error
				blockRes, err = RegisterBlockResultsWithEventLog(client, height)
				s.Require().NoError(err)
				txHash := msgEthereumTx.AsTransaction().Hash()
				blockRes.TxsResults[0].Events = []types.Event{
					{Type: evmtypes.EventTypeEthereumTx, Attributes: []types.EventAttribute{
						{Key: evmtypes.AttributeKeyEthereumTxHash, Value: txHash.Hex()},
						{Key: evmtypes.AttributeKeyTxIndex, Value: "0"},
						{Key: evmtypes.AttributeKeyTxGasUsed, Value: "21000"},
					}},
				}
				_ = s.backend.Indexer.IndexBlock(resBlock.Block, blockRes.TxsResults)
				RegisterConsensusParams(client, height)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterValidatorAccount(QueryClient, validator)
			},
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset test and queries

			tc.registerMock(tc.blockNumber, math.NewIntFromBigInt(tc.baseFee))
			header, err := s.backend.HeaderByNumber(tc.blockNumber)

			if tc.expPass {
				msgs := s.backend.EthMsgsFromCometBlock(resBlock, blockRes)
				expHeader := s.buildEthBlock(blockRes, resBlock, msgs, validator, tc.baseFee).Header()

				s.Require().NoError(err)
				s.Require().Equal(expHeader, header)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestHeaderByHash() {
	var (
		blockRes *cmtrpctypes.ResultBlockResults
		resBlock *cmtrpctypes.ResultBlock
	)

	msgEthereumTx, _ := s.buildEthereumTx()
	signedBz := s.signAndEncodeEthTx(msgEthereumTx)
	validator := sdk.AccAddress(utiltx.GenerateAddress().Bytes())

	block := cmttypes.MakeBlock(1, []cmttypes.Tx{signedBz}, nil, nil)
	emptyBlock := cmttypes.MakeBlock(1, []cmttypes.Tx{}, nil, nil)

	testCases := []struct {
		name         string
		hash         common.Hash
		baseFee      *big.Int
		registerMock func(common.Hash, math.Int)
		expPass      bool
	}{
		{
			"fail - CometBFT client failed to get block",
			common.BytesToHash(block.Hash()),
			math.NewInt(1).BigInt(),
			func(hash common.Hash, _ math.Int) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockByHashError(client, hash, signedBz)
			},
			false,
		},
		{
			"pass - without Base Fee, failed to fetch from prunned block",
			common.BytesToHash(block.Hash()),
			nil,
			func(hash common.Hash, _ math.Int) {
				height := int64(1)
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resBlock = RegisterBlockByHash(client, hash, signedBz)
				var err error
				blockRes, err = RegisterBlockResultsWithEventLog(client, height)
				s.Require().NoError(err)
				txHash := msgEthereumTx.AsTransaction().Hash()
				blockRes.TxsResults[0].Events = []types.Event{
					{Type: evmtypes.EventTypeEthereumTx, Attributes: []types.EventAttribute{
						{Key: evmtypes.AttributeKeyEthereumTxHash, Value: txHash.Hex()},
						{Key: evmtypes.AttributeKeyTxIndex, Value: "0"},
						{Key: evmtypes.AttributeKeyTxGasUsed, Value: "21000"},
					}},
				}
				s.Require().NoError(s.backend.Indexer.IndexBlock(block, blockRes.TxsResults))
				RegisterConsensusParams(client, height)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFeeError(QueryClient)
				RegisterValidatorAccount(QueryClient, validator)
			},
			true,
		},
		{
			"pass - blockNum = 1, without tx",
			common.BytesToHash(emptyBlock.Hash()),
			math.NewInt(1).BigInt(),
			func(hash common.Hash, baseFee math.Int) {
				height := int64(1)
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resBlock = RegisterBlockByHash(client, hash, nil)
				blockRes = RegisterBlockResults(client, height)
				s.Require().NoError(s.backend.Indexer.IndexBlock(block, blockRes.TxsResults))
				RegisterConsensusParams(client, height)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterValidatorAccount(QueryClient, validator)
			},
			true,
		},
		{
			"pass - with tx",
			common.BytesToHash(block.Hash()),
			math.NewInt(1).BigInt(),
			func(hash common.Hash, baseFee math.Int) {
				height := int64(1)
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resBlock = RegisterBlockByHash(client, hash, signedBz)
				var err error
				blockRes, err = RegisterBlockResultsWithEventLog(client, height)
				s.Require().NoError(err)
				txHash := msgEthereumTx.AsTransaction().Hash()
				blockRes.TxsResults[0].Events = []types.Event{
					{Type: evmtypes.EventTypeEthereumTx, Attributes: []types.EventAttribute{
						{Key: evmtypes.AttributeKeyEthereumTxHash, Value: txHash.Hex()},
						{Key: evmtypes.AttributeKeyTxIndex, Value: "0"},
						{Key: evmtypes.AttributeKeyTxGasUsed, Value: "21000"},
					}},
				}
				s.Require().NoError(s.backend.Indexer.IndexBlock(resBlock.Block, blockRes.TxsResults))
				RegisterConsensusParams(client, height)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterValidatorAccount(QueryClient, validator)
			},
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset test and queries

			tc.registerMock(tc.hash, math.NewIntFromBigInt(tc.baseFee))
			header, err := s.backend.HeaderByHash(tc.hash)

			if tc.expPass {
				msgs := s.backend.EthMsgsFromCometBlock(resBlock, blockRes)
				expHeader := s.buildEthBlock(blockRes, resBlock, msgs, validator, tc.baseFee).Header()

				s.Require().NoError(err)
				s.Require().Equal(expHeader, header)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestEthBlockByNumber() {
	var (
		blockRes *cmtrpctypes.ResultBlockResults
		resBlock *cmtrpctypes.ResultBlock
	)

	msgEthereumTx, _ := s.buildEthereumTx()
	signedBz := s.signAndEncodeEthTx(msgEthereumTx)
	validator := sdk.AccAddress(utiltx.GenerateAddress().Bytes())
	baseFee := math.NewInt(1)

	testCases := []struct {
		name         string
		blockNumber  ethrpc.BlockNumber
		registerMock func(ethrpc.BlockNumber)
		expPass      bool
	}{
		{
			"fail - CometBFT client failed to get block",
			ethrpc.BlockNumber(1),
			func(blockNum ethrpc.BlockNumber) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockError(client, height)
			},
			false,
		},
		{
			"fail - block result not found for height",
			ethrpc.BlockNumber(1),
			func(blockNum ethrpc.BlockNumber) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resBlock = RegisterBlock(client, height, nil)
				RegisterBlockResultsError(client, blockNum.Int64())
			},
			false,
		},
		{
			"pass - block without tx",
			ethrpc.BlockNumber(1),
			func(blockNum ethrpc.BlockNumber) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resBlock = RegisterBlock(client, height, nil)
				blockRes = RegisterBlockResults(client, blockNum.Int64())
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterConsensusParams(client, height)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterValidatorAccount(QueryClient, validator)
			},
			true,
		},
		{
			"pass - block with tx",
			ethrpc.BlockNumber(1),
			func(blockNum ethrpc.BlockNumber) {
				height := blockNum.Int64()
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				resBlock = RegisterBlock(client, height, signedBz)
				var err error
				blockRes, err = RegisterBlockResultsWithEventLog(client, blockNum.Int64())
				s.Require().NoError(err)
				txHash := msgEthereumTx.AsTransaction().Hash()
				blockRes.TxsResults[0].Events = []types.Event{
					{Type: evmtypes.EventTypeEthereumTx, Attributes: []types.EventAttribute{
						{Key: evmtypes.AttributeKeyEthereumTxHash, Value: txHash.Hex()},
						{Key: evmtypes.AttributeKeyTxIndex, Value: "0"},
						{Key: evmtypes.AttributeKeyTxGasUsed, Value: "21000"},
					}},
				}
				s.Require().NoError(s.backend.Indexer.IndexBlock(resBlock.Block, blockRes.TxsResults))
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterConsensusParams(client, height)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterValidatorAccount(QueryClient, validator)
			},
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset test and queries
			tc.registerMock(tc.blockNumber)

			ethBlock, err := s.backend.EthBlockByNumber(tc.blockNumber)

			if tc.expPass {
				s.Require().NoError(err)

				msgs := s.backend.EthMsgsFromCometBlock(resBlock, blockRes)
				txs := make([]*ethtypes.Transaction, len(msgs))
				for i, m := range msgs {
					txs[i] = m.AsTransaction()
				}
				expEthBlock := s.buildEthBlock(blockRes, resBlock, msgs, validator, baseFee.BigInt())
				s.Require().Equal(expEthBlock.Header(), ethBlock.Header())
				s.Require().Equal(expEthBlock.Uncles(), ethBlock.Uncles())
				s.Require().Equal(expEthBlock.ReceiptHash(), ethBlock.ReceiptHash())
				for i, tx := range expEthBlock.Transactions() {
					s.Require().Equal(tx.Data(), ethBlock.Transactions()[i].Data())
				}

			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestEthBlockFromCometBlock() {
	msgEthereumTx, _ := s.buildEthereumTx()
	bz := s.signAndEncodeEthTx(msgEthereumTx)
	emptyBlock := cmttypes.MakeBlock(1, []cmttypes.Tx{}, nil, nil)

	testCases := []struct {
		name         string
		baseFee      *big.Int
		resBlock     *cmtrpctypes.ResultBlock
		blockRes     *cmtrpctypes.ResultBlockResults
		validator    sdk.AccAddress
		registerMock func(math.Int, int64, sdk.AccAddress)
		expPass      bool
	}{
		{
			"pass - block without tx",
			math.NewInt(1).BigInt(),
			&cmtrpctypes.ResultBlock{
				Block: emptyBlock,
			},
			&cmtrpctypes.ResultBlockResults{
				Height:     1,
				TxsResults: []*types.ExecTxResult{{Code: 0, GasUsed: 0}},
			},
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			func(baseFee math.Int, _ int64, validator sdk.AccAddress) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterConsensusParams(client, 1)
				RegisterValidatorAccount(QueryClient, validator)
			},
			true,
		},
		{
			"pass - block with tx",
			math.NewInt(1).BigInt(),
			&cmtrpctypes.ResultBlock{
				Block: cmttypes.MakeBlock(1, []cmttypes.Tx{bz}, nil, nil),
			},
			&cmtrpctypes.ResultBlockResults{
				Height:     1,
				TxsResults: []*types.ExecTxResult{{Code: 0, GasUsed: 0}},
				FinalizeBlockEvents: []types.Event{
					{
						Type: evmtypes.EventTypeBlockBloom,
						Attributes: []types.EventAttribute{
							{Key: evmtypes.AttributeKeyEthereumBloom},
						},
					},
				},
			},
			sdk.AccAddress(utiltx.GenerateAddress().Bytes()),
			func(baseFee math.Int, _ int64, validator sdk.AccAddress) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				RegisterBaseFee(QueryClient, baseFee)
				RegisterConsensusParams(client, 1)
				RegisterValidatorAccount(QueryClient, validator)
			},
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset test and queries
			tc.registerMock(math.NewIntFromBigInt(tc.baseFee), tc.blockRes.Height, tc.validator)

			// If the case includes an ethereum tx, ensure logs/data and indexer are prepared
			if len(tc.resBlock.Block.Txs) > 0 && len(tc.blockRes.TxsResults) > 0 {
				// Provide MsgEthereumTxResponse in Data
				anyValue, err := codectypes.NewAnyWithValue(&evmtypes.MsgEthereumTxResponse{})
				s.Require().NoError(err)
				data, err := proto.Marshal(&sdk.TxMsgData{MsgResponses: []*codectypes.Any{anyValue}})
				s.Require().NoError(err)
				tc.blockRes.TxsResults[0].Data = data

				// Inject ethereum_tx events for indexer parsing
				txHash := msgEthereumTx.AsTransaction().Hash()
				tc.blockRes.TxsResults[0].Events = []types.Event{
					{Type: evmtypes.EventTypeEthereumTx, Attributes: []types.EventAttribute{
						{Key: evmtypes.AttributeKeyEthereumTxHash, Value: txHash.Hex()},
						{Key: evmtypes.AttributeKeyTxIndex, Value: "0"},
						{Key: evmtypes.AttributeKeyTxGasUsed, Value: "21000"},
					}},
				}
				s.Require().NoError(s.backend.Indexer.IndexBlock(tc.resBlock.Block, tc.blockRes.TxsResults))
			}

			ethBlock, err := s.backend.EthBlockFromCometBlock(tc.resBlock, tc.blockRes)

			if tc.expPass {
				s.Require().NoError(err)

				msgs := s.backend.EthMsgsFromCometBlock(tc.resBlock, tc.blockRes)
				txs := make([]*ethtypes.Transaction, len(msgs))
				for i, m := range msgs {
					txs[i] = m.AsTransaction()
				}

				expBlock := s.buildEthBlock(tc.blockRes, tc.resBlock, msgs, tc.validator, tc.baseFee)
				s.Require().Equal(expBlock.Header(), ethBlock.Header())
				s.Require().Equal(expBlock.Uncles(), ethBlock.Uncles())
				s.Require().Equal(expBlock.ReceiptHash(), ethBlock.ReceiptHash())
				for i, tx := range expBlock.Transactions() {
					s.Require().Equal(tx.Data(), ethBlock.Transactions()[i].Data())
				}

			} else {
				s.Require().Error(err)
			}
		})
	}
}
