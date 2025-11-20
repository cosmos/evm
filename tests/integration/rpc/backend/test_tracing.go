package backend

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	mock "github.com/stretchr/testify/mock"

	abci "github.com/cometbft/cometbft/abci/types"
	tmrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/indexer"
	"github.com/cosmos/evm/rpc/backend/mocks"
	rpctypes "github.com/cosmos/evm/rpc/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/crypto"
)

func (s *TestSuite) TestTraceTransaction() {
	msgEthereumTx, _ := s.buildEthereumTx()
	msgEthereumTx2, _ := s.buildEthereumTx()

	txHash := msgEthereumTx.AsTransaction().Hash()
	txHash2 := msgEthereumTx2.AsTransaction().Hash()

	priv, _ := ethsecp256k1.GenerateKey()
	from := common.BytesToAddress(priv.PubKey().Address().Bytes())
	armor := crypto.EncryptArmorPrivKey(priv, "", "eth_secp256k1")
	_ = s.backend.ClientCtx.Keyring.ImportPrivKey("test_key", armor, "")

	ethSigner := ethtypes.LatestSigner(s.backend.ChainConfig())

	txEncoder := s.backend.ClientCtx.TxConfig.TxEncoder()

	msgEthereumTx.From = from.Bytes()
	_ = msgEthereumTx.Sign(ethSigner, s.signer)

	baseDenom := evmtypes.GetEVMCoinDenom()

	tx, _ := msgEthereumTx.BuildTx(s.backend.ClientCtx.TxConfig.NewTxBuilder(), baseDenom)
	txBz, _ := txEncoder(tx)

	msgEthereumTx2.From = from.Bytes()
	_ = msgEthereumTx2.Sign(ethSigner, s.signer)

	tx2, _ := msgEthereumTx.BuildTx(s.backend.ClientCtx.TxConfig.NewTxBuilder(), baseDenom)
	txBz2, _ := txEncoder(tx2)

	testCases := []struct {
		name          string
		registerMock  func()
		block         *types.Block
		responseBlock []*abci.ExecTxResult
		expResult     interface{}
		expPass       bool
	}{
		{
			"fail - tx not found",
			func() {},
			&types.Block{Header: types.Header{Height: 1}, Data: types.Data{Txs: []types.Tx{}}},
			[]*abci.ExecTxResult{
				{
					Code: 0,
					Events: []abci.Event{
						{Type: evmtypes.EventTypeEthereumTx, Attributes: []abci.EventAttribute{
							{Key: "ethereumTxHash", Value: txHash.Hex()},
							{Key: "txIndex", Value: "0"},
							{Key: "amount", Value: "1000"},
							{Key: "txGasUsed", Value: "21000"},
							{Key: "txHash", Value: ""},
							{Key: "recipient", Value: "0x775b87ef5D82ca211811C1a02CE0fE0CA3a455d7"},
						}},
					},
				},
			},
			nil,
			false,
		},
		{
			"fail - block not found",
			func() {
				// var header metadata.MD
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockError(client, 1)
			},
			&types.Block{Header: types.Header{Height: 1}, Data: types.Data{Txs: []types.Tx{txBz}}},
			[]*abci.ExecTxResult{
				{
					Code: 0,
					Events: []abci.Event{
						{Type: evmtypes.EventTypeEthereumTx, Attributes: []abci.EventAttribute{
							{Key: "ethereumTxHash", Value: txHash.Hex()},
							{Key: "txIndex", Value: "0"},
							{Key: "amount", Value: "1000"},
							{Key: "txGasUsed", Value: "21000"},
							{Key: "txHash", Value: ""},
							{Key: "recipient", Value: "0x775b87ef5D82ca211811C1a02CE0fE0CA3a455d7"},
						}},
					},
				},
			},
			map[string]interface{}{"test": "hello"},
			false,
		},
		{
			"pass - transaction found in a block with multiple transactions",
			func() {
				var (
					QueryClient       = s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
					client            = s.backend.ClientCtx.Client.(*mocks.Client)
					height      int64 = 1
				)
				RegisterBlockMultipleTxs(client, height, []types.Tx{txBz, txBz2})
				RegisterTraceTransactionWithPredecessors(QueryClient, msgEthereumTx, []*evmtypes.MsgEthereumTx{msgEthereumTx})
				RegisterConsensusParams(client, height)
			},
			&types.Block{Header: types.Header{Height: 1, ChainID: ChainID.ChainID}, Data: types.Data{Txs: []types.Tx{txBz, txBz2}}},
			[]*abci.ExecTxResult{
				{
					Code: 0,
					Events: []abci.Event{
						{Type: evmtypes.EventTypeEthereumTx, Attributes: []abci.EventAttribute{
							{Key: "ethereumTxHash", Value: txHash.Hex()},
							{Key: "txIndex", Value: "0"},
							{Key: "amount", Value: "1000"},
							{Key: "txGasUsed", Value: "21000"},
							{Key: "txHash", Value: ""},
							{Key: "recipient", Value: "0x775b87ef5D82ca211811C1a02CE0fE0CA3a455d7"},
						}},
					},
				},
				{
					Code: 0,
					Events: []abci.Event{
						{Type: evmtypes.EventTypeEthereumTx, Attributes: []abci.EventAttribute{
							{Key: "ethereumTxHash", Value: txHash2.Hex()},
							{Key: "txIndex", Value: "1"},
							{Key: "amount", Value: "1000"},
							{Key: "txGasUsed", Value: "21000"},
							{Key: "txHash", Value: ""},
							{Key: "recipient", Value: "0x775b87ef5D82ca211811C1a02CE0fE0CA3a455d7"},
						}},
					},
				},
			},
			map[string]interface{}{"test": "hello"},
			true,
		},
		{
			"pass - transaction found",
			func() {
				var (
					QueryClient       = s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
					client            = s.backend.ClientCtx.Client.(*mocks.Client)
					height      int64 = 1
				)
				RegisterBlock(client, height, txBz)
				RegisterTraceTransaction(QueryClient, msgEthereumTx)
				RegisterConsensusParams(client, height)
			},
			&types.Block{Header: types.Header{Height: 1}, Data: types.Data{Txs: []types.Tx{txBz}}},
			[]*abci.ExecTxResult{
				{
					Code: 0,
					Events: []abci.Event{
						{Type: evmtypes.EventTypeEthereumTx, Attributes: []abci.EventAttribute{
							{Key: "ethereumTxHash", Value: txHash.Hex()},
							{Key: "txIndex", Value: "0"},
							{Key: "amount", Value: "1000"},
							{Key: "txGasUsed", Value: "21000"},
							{Key: "txHash", Value: ""},
							{Key: "recipient", Value: "0x775b87ef5D82ca211811C1a02CE0fE0CA3a455d7"},
						}},
					},
				},
			},
			map[string]interface{}{"test": "hello"},
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("case %s", tc.name), func() {
			s.SetupTest() // reset test and queries
			tc.registerMock()

			db := dbm.NewMemDB()
			s.backend.Indexer = indexer.NewKVIndexer(db, log.NewNopLogger(), s.backend.ClientCtx)

			err := s.backend.Indexer.IndexBlock(tc.block, tc.responseBlock)
			s.Require().NoError(err)
			txResult, err := s.backend.TraceTransaction(txHash, nil)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(tc.expResult, txResult)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestTraceCall() {
	msgEthTx, _ := s.buildEthereumTx()

	testCases := []struct {
		name          string
		registerMock  func()
		args          evmtypes.TransactionArgs
		blockNrOrHash rpctypes.BlockNumberOrHash
		config        *rpctypes.TraceConfig
		expResult     interface{}
		expPass       bool
	}{
		{
			"pass - trace call with block number",
			func() {
				var (
					QueryClient = s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
					client      = s.backend.ClientCtx.Client.(*mocks.Client)
					height      = int64(1)
				)
				RegisterHeader(client, &height, nil)
				RegisterTraceCall(QueryClient, msgEthTx)
			},
			evmtypes.TransactionArgs{
				From: &common.Address{0x1},
				To:   &common.Address{0x2},
			},
			rpctypes.BlockNumberOrHash{
				BlockNumber: func() *rpctypes.BlockNumber {
					bn := rpctypes.BlockNumber(1)
					return &bn
				}(),
			},
			&rpctypes.TraceConfig{},
			map[string]interface{}{"test": "trace_call"},
			true,
		},
		{
			"pass - trace call with block hash",
			func() {
				var (
					QueryClient = s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
					client      = s.backend.ClientCtx.Client.(*mocks.Client)
					height      = int64(1)
				)
				blockHash := common.Hash{} // Use zero hash to match RegisterHeaderByHash
				_ = RegisterHeaderByHash(client, blockHash, blockHash.Bytes())
				RegisterHeader(client, &height, nil)
				RegisterTraceCall(QueryClient, msgEthTx)
			},
			evmtypes.TransactionArgs{
				From: &common.Address{0x1},
				To:   &common.Address{0x2},
			},
			rpctypes.BlockNumberOrHash{
				BlockHash: func() *common.Hash {
					h := common.Hash{} // Use zero hash to match RegisterHeaderByHash
					return &h
				}(),
			},
			&rpctypes.TraceConfig{},
			map[string]interface{}{"test": "trace_call"},
			true,
		},
		{
			"pass - trace call with callTracer",
			func() {
				var (
					QueryClient = s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
					client      = s.backend.ClientCtx.Client.(*mocks.Client)
					height      = int64(1)
				)
				RegisterHeader(client, &height, nil)
				RegisterTraceCallWithTracer(QueryClient, msgEthTx, "callTracer")
			},
			evmtypes.TransactionArgs{
				From: &common.Address{0x1},
				To:   &common.Address{0x2},
			},
			rpctypes.BlockNumberOrHash{
				BlockNumber: func() *rpctypes.BlockNumber {
					bn := rpctypes.BlockNumber(1)
					return &bn
				}(),
			},
			&rpctypes.TraceConfig{
				TraceConfig: evmtypes.TraceConfig{
					Tracer: "callTracer",
				},
			},
			map[string]interface{}{"type": "CALL"},
			true,
		},
		{
			"fail - invalid block number or hash",
			func() {},
			evmtypes.TransactionArgs{
				From: &common.Address{0x1},
				To:   &common.Address{0x2},
			},
			rpctypes.BlockNumberOrHash{
				BlockNumber: nil,
				BlockHash:   nil,
			},
			nil,
			nil,
			false,
		},
		{
			"fail - block not found",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				height := int64(1)
				RegisterHeaderError(client, &height)
			},
			evmtypes.TransactionArgs{
				From: &common.Address{0x1},
				To:   &common.Address{0x2},
			},
			rpctypes.BlockNumberOrHash{
				BlockNumber: func() *rpctypes.BlockNumber {
					bn := rpctypes.BlockNumber(1)
					return &bn
				}(),
			},
			nil,
			nil,
			false,
		},
		{
			"fail - query client error",
			func() {
				var (
					QueryClient = s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
					client      = s.backend.ClientCtx.Client.(*mocks.Client)
					height      = int64(1)
				)
				RegisterHeader(client, &height, nil)
				RegisterTraceCallError(QueryClient)
			},
			evmtypes.TransactionArgs{
				From: &common.Address{0x1},
				To:   &common.Address{0x2},
			},
			rpctypes.BlockNumberOrHash{
				BlockNumber: func() *rpctypes.BlockNumber {
					bn := rpctypes.BlockNumber(1)
					return &bn
				}(),
			},
			nil,
			nil,
			false,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("case %s", tc.name), func() {
			s.SetupTest() // reset test and queries
			tc.registerMock()

			result, err := s.backend.TraceCall(tc.args, tc.blockNrOrHash, tc.config)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(tc.expResult, result)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestTraceBlock() {
	msgEthTx, bz := s.buildEthereumTx()
	emptyBlock := types.MakeBlock(1, []types.Tx{}, nil, nil)
	emptyBlock.ChainID = ChainID.ChainID
	filledBlock := types.MakeBlock(1, []types.Tx{bz}, nil, nil)
	filledBlock.ChainID = ChainID.ChainID
	resBlockEmpty := tmrpctypes.ResultBlock{Block: emptyBlock, BlockID: emptyBlock.LastBlockID}
	resBlockFilled := tmrpctypes.ResultBlock{Block: filledBlock, BlockID: filledBlock.LastBlockID}

	testCases := []struct {
		name            string
		registerMock    func()
		expTraceResults []*evmtypes.TxTraceResult
		resBlock        *tmrpctypes.ResultBlock
		config          *rpctypes.TraceConfig
		expPass         bool
	}{
		{
			"pass - no transaction returning empty array",
			func() {},
			[]*evmtypes.TxTraceResult{},
			&resBlockEmpty,
			&rpctypes.TraceConfig{},
			true,
		},
		{
			"fail - cannot unmarshal data",
			func() {
				QueryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterTraceBlock(QueryClient, []*evmtypes.MsgEthereumTx{msgEthTx})
				RegisterConsensusParams(client, 1)
				RegisterBlockResults(client, 1)
			},
			[]*evmtypes.TxTraceResult{},
			&resBlockFilled,
			&rpctypes.TraceConfig{},
			false,
		},
		{
			"fail - TendermintBlockResultByNumber returns error",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockResultsError(client, 1)
			},
			nil,
			&resBlockFilled,
			&rpctypes.TraceConfig{},
			true,
		},
		{
			"skip invalid tx result code - transaction failed",
			func() {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockResultsWithTxs(client, 1, []*abci.ExecTxResult{{Code: 0}, {Code: 1}, {Code: 0}})
				RegisterConsensusParams(client, 1)
				traceResult := &evmtypes.QueryTraceBlockResponse{
					Data: []byte(`[{"result": "trace1"}, {"result": "trace2"}]`),
				}
				queryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
				queryClient.On("TraceBlock", mock.Anything, mock.AnythingOfType("*types.QueryTraceBlockRequest")).
					Return(traceResult, nil).
					Once()
			},
			[]*evmtypes.TxTraceResult{{Result: "trace1"}, {Result: "trace2"}},
			&resBlockFilled,
			&rpctypes.TraceConfig{},
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("case %s", tc.name), func() {
			s.SetupTest() // reset test and queries
			tc.registerMock()

			traceResults, err := s.backend.TraceBlock(1, tc.config, tc.resBlock)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(tc.expTraceResults, traceResults)
			} else {
				s.Require().Error(err)
			}
		})
	}
}
