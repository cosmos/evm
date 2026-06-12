package backend

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm/indexer"
	rpctypes "github.com/cosmos/evm/rpc/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
)

func (suite *BackendTestSuite) TestTraceTransaction() {
	msgEthereumTx, _ := suite.buildEthereumTx()
	msgEthereumTx2, _ := suite.buildEthereumTx()

	txHash := msgEthereumTx.AsTransaction().Hash()
	txHash2 := msgEthereumTx2.AsTransaction().Hash()

	txBz := suite.signAndEncodeEthTx(msgEthereumTx)
	txBz2 := suite.signAndEncodeEthTx(msgEthereumTx2)

	// Recompute hashes after signing (From is set by signAndEncodeEthTx).
	txHash = msgEthereumTx.AsTransaction().Hash()
	txHash2 = msgEthereumTx2.AsTransaction().Hash()

	testCases := []struct {
		name          string
		registerMock  func()
		block         *types.Block
		responseBlock []*abci.ExecTxResult
		expPass       bool
	}{
		{
			"fail - tx not found",
			func() {
				client := suite.mockClient()
				query := fmt.Sprintf("%s.%s='%s'", evmtypes.TypeMsgEthereumTx, evmtypes.AttributeKeyEthereumTxHash, txHash.Hex())
				RegisterTxSearchEmpty(client, query)
			},
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
			false,
		},
		{
			"fail - block not found",
			func() {
				client := suite.mockClient()
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
			false,
		},
		{
			"pass - transaction found in a block with multiple transactions",
			func() {
				queryClient := suite.mockQueryClient()
				client := suite.mockClient()
				_, err := RegisterBlockMultipleTxs(client, 1, []types.Tx{txBz, txBz2})
				suite.Require().NoError(err)
				RegisterTraceTransactionWithPredecessors(queryClient, msgEthereumTx, nil)
				RegisterConsensusParams(client, 1)
			},
			&types.Block{Header: types.Header{Height: 1, ChainID: ChainID}, Data: types.Data{Txs: []types.Tx{txBz, txBz2}}},
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
			true,
		},
		{
			"pass - transaction found",
			func() {
				queryClient := suite.mockQueryClient()
				client := suite.mockClient()
				_, err := RegisterBlock(client, 1, txBz)
				suite.Require().NoError(err)
				RegisterTraceTransaction(queryClient, msgEthereumTx)
				RegisterConsensusParams(client, 1)
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
			true,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("case %s", tc.name), func() {
			suite.SetupTest()
			tc.registerMock()

			suite.backend.Indexer = indexer.NewKVIndexer(dbm.NewMemDB(), log.NewNopLogger(), suite.backend.ClientCtx)
			err := suite.backend.Indexer.IndexBlock(tc.block, tc.responseBlock)
			suite.Require().NoError(err)
			_, err = suite.backend.TraceTransaction(txHash, nil)

			if tc.expPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

// TestTraceTransactionEthTxIndex verifies that TraceTransaction correctly traces
// a transaction that is not the first in a multi-tx block, using EthTxIndex (not
// TxIndex) as the predecessor-loop bound after the index-domain fix.
func (suite *BackendTestSuite) TestTraceTransactionEthTxIndex() {
	suite.SetupTest()

	msgFirst, _ := suite.buildEthereumTx()
	txBzFirst := suite.signAndEncodeEthTx(msgFirst)
	txHashFirst := msgFirst.AsTransaction().Hash()

	msgTarget, _ := suite.buildEthereumTx()
	txBzTarget := suite.signAndEncodeEthTx(msgTarget)
	txHashTarget := msgTarget.AsTransaction().Hash()

	localBlock := types.MakeBlock(1, []types.Tx{txBzFirst, txBzTarget}, nil, nil)
	localBlock.ChainID = ChainID

	responseBlock := []*abci.ExecTxResult{
		{
			Code: 0,
			Events: []abci.Event{{
				Type: evmtypes.EventTypeEthereumTx,
				Attributes: []abci.EventAttribute{
					{Key: evmtypes.AttributeKeyEthereumTxHash, Value: txHashFirst.Hex()},
					{Key: evmtypes.AttributeKeyTxIndex, Value: "0"},
					{Key: evmtypes.AttributeKeyTxGasUsed, Value: "21000"},
				},
			}},
		},
		{
			Code: 0,
			Events: []abci.Event{{
				Type: evmtypes.EventTypeEthereumTx,
				Attributes: []abci.EventAttribute{
					{Key: evmtypes.AttributeKeyEthereumTxHash, Value: txHashTarget.Hex()},
					{Key: evmtypes.AttributeKeyTxIndex, Value: "1"},
					{Key: evmtypes.AttributeKeyTxGasUsed, Value: "21000"},
				},
			}},
		},
	}

	suite.backend.Indexer = indexer.NewKVIndexer(dbm.NewMemDB(), log.NewNopLogger(), suite.backend.ClientCtx)
	suite.Require().NoError(suite.backend.Indexer.IndexBlock(localBlock, responseBlock))

	queryClient := suite.mockQueryClient()
	client := suite.mockClient()

	_, err := RegisterBlockMultipleTxs(client, 1, []types.Tx{txBzFirst, txBzTarget})
	suite.Require().NoError(err)

	// EthTxIndex=1: the predecessor loop runs once (i=0) and fetches msgFirst.
	RegisterTraceTransactionWithPredecessors(queryClient, msgTarget, []*evmtypes.MsgEthereumTx{msgFirst})
	RegisterConsensusParams(client, 1)

	_, err = suite.backend.TraceTransaction(txHashTarget, nil)
	suite.Require().NoError(err)
}

// ethTxEvent returns a minimal EventTypeEthereumTx event suitable for IndexBlock.
func ethTxEvent(hash string, txIndex string) abci.Event {
	return abci.Event{
		Type: evmtypes.EventTypeEthereumTx,
		Attributes: []abci.EventAttribute{
			{Key: evmtypes.AttributeKeyEthereumTxHash, Value: hash},
			{Key: evmtypes.AttributeKeyTxIndex, Value: txIndex},
			{Key: evmtypes.AttributeKeyTxGasUsed, Value: "21000"},
		},
	}
}

// TestTraceTransactionMultiMsgSameCosmosTarget traces the second of two EVM messages
// packed into a single Cosmos tx slot. The same-Cosmos-tx guard fires on every outer-loop
// iteration, leaving the after-loop to supply the sole predecessor.
//
// Block layout:  slot0=[msg1, msg2=target]
// Expected predecessors: [msg1]
func (suite *BackendTestSuite) TestTraceTransactionMultiMsgSameCosmosTarget() {
	suite.SetupTest()

	msg1, _ := suite.buildEthereumTx()
	msgTarget, _ := suite.buildEthereumTx()
	txBzMulti := suite.buildAndEncodeMultiMsgEthTx(msg1, msgTarget)

	hash1 := msg1.AsTransaction().Hash()
	hashTarget := msgTarget.AsTransaction().Hash()

	localBlock := types.MakeBlock(1, []types.Tx{txBzMulti}, nil, nil)
	localBlock.ChainID = ChainID

	responseBlock := []*abci.ExecTxResult{{
		Code: 0,
		Events: []abci.Event{
			ethTxEvent(hash1.Hex(), "0"),
			ethTxEvent(hashTarget.Hex(), "1"),
		},
	}}

	suite.backend.Indexer = indexer.NewKVIndexer(dbm.NewMemDB(), log.NewNopLogger(), suite.backend.ClientCtx)
	suite.Require().NoError(suite.backend.Indexer.IndexBlock(localBlock, responseBlock))

	queryClient := suite.mockQueryClient()
	client := suite.mockClient()

	_, err := RegisterBlockMultipleTxs(client, 1, []types.Tx{txBzMulti})
	suite.Require().NoError(err)

	RegisterTraceTransactionWithPredecessors(queryClient, msgTarget, []*evmtypes.MsgEthereumTx{msg1})
	RegisterConsensusParams(client, 1)

	_, err = suite.backend.TraceTransaction(hashTarget, nil)
	suite.Require().NoError(err)
}

// TestTraceTransactionMultiMsgTargetIsThird traces the third of three EVM messages
// packed into a single Cosmos tx. The outer loop skips all same-slot entries; the
// after-loop adds both msg1 and msg2.
//
// Block layout:  slot0=[msg1, msg2, msg3=target]
// Expected predecessors: [msg1, msg2]
func (suite *BackendTestSuite) TestTraceTransactionMultiMsgTargetIsThird() {
	suite.SetupTest()

	msg1, _ := suite.buildEthereumTx()
	msg2, _ := suite.buildEthereumTx()
	msgTarget, _ := suite.buildEthereumTx()
	txBzMulti := suite.buildAndEncodeMultiMsgEthTx(msg1, msg2, msgTarget)

	hash1 := msg1.AsTransaction().Hash()
	hash2 := msg2.AsTransaction().Hash()
	hashTarget := msgTarget.AsTransaction().Hash()

	localBlock := types.MakeBlock(1, []types.Tx{txBzMulti}, nil, nil)
	localBlock.ChainID = ChainID

	responseBlock := []*abci.ExecTxResult{{
		Code: 0,
		Events: []abci.Event{
			ethTxEvent(hash1.Hex(), "0"),
			ethTxEvent(hash2.Hex(), "1"),
			ethTxEvent(hashTarget.Hex(), "2"),
		},
	}}

	suite.backend.Indexer = indexer.NewKVIndexer(dbm.NewMemDB(), log.NewNopLogger(), suite.backend.ClientCtx)
	suite.Require().NoError(suite.backend.Indexer.IndexBlock(localBlock, responseBlock))

	queryClient := suite.mockQueryClient()
	client := suite.mockClient()

	_, err := RegisterBlockMultipleTxs(client, 1, []types.Tx{txBzMulti})
	suite.Require().NoError(err)

	RegisterTraceTransactionWithPredecessors(queryClient, msgTarget, []*evmtypes.MsgEthereumTx{msg1, msg2})
	RegisterConsensusParams(client, 1)

	_, err = suite.backend.TraceTransaction(hashTarget, nil)
	suite.Require().NoError(err)
}

// TestTraceTransactionMultiMsgCosmosAsPredecessor traces a single-message target whose
// sole predecessor is a two-message Cosmos tx. Both messages must appear in the
// predecessor list — validates the fix that adds the message AT MsgIndex directly
// instead of the old inner loop that ran j<MsgIndex and missed the last message.
//
// Block layout:  slot0=[msg1, msg2], slot1=[msgTarget]
// Expected predecessors: [msg1, msg2]
func (suite *BackendTestSuite) TestTraceTransactionMultiMsgCosmosAsPredecessor() {
	suite.SetupTest()

	msg1, _ := suite.buildEthereumTx()
	msg2, _ := suite.buildEthereumTx()
	msgTarget, _ := suite.buildEthereumTx()

	txBzPred := suite.buildAndEncodeMultiMsgEthTx(msg1, msg2)
	txBzTarget := suite.signAndEncodeEthTx(msgTarget)

	hash1 := msg1.AsTransaction().Hash()
	hash2 := msg2.AsTransaction().Hash()
	hashTarget := msgTarget.AsTransaction().Hash()

	localBlock := types.MakeBlock(1, []types.Tx{txBzPred, txBzTarget}, nil, nil)
	localBlock.ChainID = ChainID

	responseBlock := []*abci.ExecTxResult{
		{
			Code:   0,
			Events: []abci.Event{ethTxEvent(hash1.Hex(), "0"), ethTxEvent(hash2.Hex(), "1")},
		},
		{
			Code:   0,
			Events: []abci.Event{ethTxEvent(hashTarget.Hex(), "2")},
		},
	}

	suite.backend.Indexer = indexer.NewKVIndexer(dbm.NewMemDB(), log.NewNopLogger(), suite.backend.ClientCtx)
	suite.Require().NoError(suite.backend.Indexer.IndexBlock(localBlock, responseBlock))

	queryClient := suite.mockQueryClient()
	client := suite.mockClient()

	_, err := RegisterBlockMultipleTxs(client, 1, []types.Tx{txBzPred, txBzTarget})
	suite.Require().NoError(err)

	RegisterTraceTransactionWithPredecessors(queryClient, msgTarget, []*evmtypes.MsgEthereumTx{msg1, msg2})
	RegisterConsensusParams(client, 1)

	_, err = suite.backend.TraceTransaction(hashTarget, nil)
	suite.Require().NoError(err)
}

// TestTraceTransactionThreeTxBlock exercises the full predecessor-assembly path across
// three Cosmos tx slots: a 1-msg slot, a 2-msg slot, and a 1-msg target slot.
//
// Block layout:  slot0=[msg1], slot1=[msg2, msg3], slot2=[msgTarget]
// Expected predecessors: [msg1, msg2, msg3]
func (suite *BackendTestSuite) TestTraceTransactionThreeTxBlock() {
	suite.SetupTest()

	msg1, _ := suite.buildEthereumTx()
	msg2, _ := suite.buildEthereumTx()
	msg3, _ := suite.buildEthereumTx()
	msgTarget, _ := suite.buildEthereumTx()

	txBz1 := suite.signAndEncodeEthTx(msg1)
	txBzPred2 := suite.buildAndEncodeMultiMsgEthTx(msg2, msg3)
	txBzTarget := suite.signAndEncodeEthTx(msgTarget)

	hash1 := msg1.AsTransaction().Hash()
	hash2 := msg2.AsTransaction().Hash()
	hash3 := msg3.AsTransaction().Hash()
	hashTarget := msgTarget.AsTransaction().Hash()

	localBlock := types.MakeBlock(1, []types.Tx{txBz1, txBzPred2, txBzTarget}, nil, nil)
	localBlock.ChainID = ChainID

	responseBlock := []*abci.ExecTxResult{
		{Code: 0, Events: []abci.Event{ethTxEvent(hash1.Hex(), "0")}},
		{Code: 0, Events: []abci.Event{ethTxEvent(hash2.Hex(), "1"), ethTxEvent(hash3.Hex(), "2")}},
		{Code: 0, Events: []abci.Event{ethTxEvent(hashTarget.Hex(), "3")}},
	}

	suite.backend.Indexer = indexer.NewKVIndexer(dbm.NewMemDB(), log.NewNopLogger(), suite.backend.ClientCtx)
	suite.Require().NoError(suite.backend.Indexer.IndexBlock(localBlock, responseBlock))

	queryClient := suite.mockQueryClient()
	client := suite.mockClient()

	_, err := RegisterBlockMultipleTxs(client, 1, []types.Tx{txBz1, txBzPred2, txBzTarget})
	suite.Require().NoError(err)

	RegisterTraceTransactionWithPredecessors(queryClient, msgTarget, []*evmtypes.MsgEthereumTx{msg1, msg2, msg3})
	RegisterConsensusParams(client, 1)

	_, err = suite.backend.TraceTransaction(hashTarget, nil)
	suite.Require().NoError(err)
}

// derivedTxEvt builds an EventTypeEthereumTx event for a derived transaction (txType=99).
func derivedTxEvt(hash string, txIndex int, sender string, recipient string, gasLimit uint64) abci.Event {
	return abci.Event{
		Type: evmtypes.EventTypeEthereumTx,
		Attributes: []abci.EventAttribute{
			{Key: evmtypes.AttributeKeyEthereumTxHash, Value: hash},
			{Key: evmtypes.AttributeKeyTxIndex, Value: fmt.Sprintf("%d", txIndex)},
			{Key: evmtypes.AttributeKeyTxGasUsed, Value: "21000"},
			{Key: evmtypes.AttributeKeyTxType, Value: fmt.Sprintf("%d", evmtypes.DerivedTxType)},
			{Key: rpctypes.SenderType, Value: sender},
			{Key: evmtypes.AttributeKeyRecipient, Value: recipient},
			{Key: evmtypes.AttributeKeyTxGasLimit, Value: fmt.Sprintf("%d", gasLimit)},
		},
	}
}

// TestTraceTransactionDerivedTxAsPredecessor verifies that a derived tx is correctly
// reconstructed and prepended to the predecessor list when it precedes the EVM target.
//
// Block layout:  slot0=[msg1_evm], slot1=[non-EVM → DerivedTx1], slot2=[msgTarget_evm]
// EthTxIndex:   msg1=0, DerivedTx1=1, msgTarget=2
// Expected predecessors: [msg1, DerivedTx1_synthetic]
func (suite *BackendTestSuite) TestTraceTransactionDerivedTxAsPredecessor() {
	suite.SetupTest()

	msg1, _ := suite.buildEthereumTx()
	txBz1 := suite.signAndEncodeEthTx(msg1)
	hash1 := msg1.AsTransaction().Hash()

	msgTarget, _ := suite.buildEthereumTx()
	txBzTarget := suite.signAndEncodeEthTx(msgTarget)
	hashTarget := msgTarget.AsTransaction().Hash()

	// Slot-1: non-EVM Cosmos tx (empty, no messages).
	dummyTxBz, err := suite.backend.ClientCtx.TxConfig.TxEncoder()(
		suite.backend.ClientCtx.TxConfig.NewTxBuilder().GetTx(),
	)
	suite.Require().NoError(err)

	hashDerived := common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
	senderAddr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	recipientAddr := common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12")
	gasLimitVal := uint64(50000)

	// Index only slots 0+1; msgTarget (slot2) is deliberately omitted so the KV
	// doesn't assign it a wrong EthTxIndex.
	localBlock := types.MakeBlock(1, []types.Tx{txBz1, dummyTxBz}, nil, nil)
	localBlock.ChainID = ChainID
	indexResults := []*abci.ExecTxResult{
		{Code: 0, Events: []abci.Event{ethTxEvent(hash1.Hex(), "0")}},
		{Code: 0, Events: []abci.Event{}},
	}
	suite.backend.Indexer = indexer.NewKVIndexer(dbm.NewMemDB(), log.NewNopLogger(), suite.backend.ClientCtx)
	suite.Require().NoError(suite.backend.Indexer.IndexBlock(localBlock, indexResults))

	queryClient := suite.mockQueryClient()
	client := suite.mockClient()

	// Actual block has 3 slots.
	_, err = RegisterBlockMultipleTxs(client, 1, []types.Tx{txBz1, dummyTxBz, txBzTarget})
	suite.Require().NoError(err)

	// GetTxByEthHash(hashTarget): KV miss → TxSearch. Returns EthTxIndex=2.
	targetHashQuery := fmt.Sprintf("%s.%s='%s'",
		evmtypes.TypeMsgEthereumTx, evmtypes.AttributeKeyEthereumTxHash, hashTarget.Hex())
	RegisterTxSearchWithResult(client, targetHashQuery, 1, 2, txBzTarget,
		[]abci.Event{ethTxEvent(hashTarget.Hex(), "2")})

	// GetTxByTxIndex(1, 1): KV miss → TxSearch by eth tx index. Returns DerivedTx1.
	derivedIdxQuery := fmt.Sprintf("tx.height=%d AND %s.%s=%d",
		1, evmtypes.TypeMsgEthereumTx, evmtypes.AttributeKeyTxIndex, 1)
	RegisterTxSearchWithResult(client, derivedIdxQuery, 1, 1, nil,
		[]abci.Event{derivedTxEvt(hashDerived.Hex(), 1, senderAddr.Hex(), recipientAddr.Hex(), gasLimitVal)})

	// Build the expected derived MsgEthereumTx that parseDerivedTxFromAdditionalFields produces.
	derivedAdditional := &rpctypes.TxResultAdditionalFields{
		Hash:      hashDerived,
		Sender:    senderAddr,
		Recipient: recipientAddr,
		GasUsed:   21000,
		GasLimit:  &gasLimitVal,
		Type:      evmtypes.DerivedTxType,
	}
	derivedMsg := suite.backend.parseDerivedTxFromAdditionalFields(derivedAdditional)
	suite.Require().NotNil(derivedMsg)

	RegisterTraceTransactionWithPredecessors(queryClient, msgTarget, []*evmtypes.MsgEthereumTx{msg1, derivedMsg})
	RegisterConsensusParams(client, 1)

	_, err = suite.backend.TraceTransaction(hashTarget, nil)
	suite.Require().NoError(err)
}

// TestTraceTransactionDerivedTxAsTarget verifies that TraceTransaction can trace a
// derived tx (the target is itself a derived EVM execution, not a Cosmos MsgEthereumTx).
//
// Block layout:  slot0=[msg1_evm], slot1=[non-EVM → DerivedTx1=target]
// EthTxIndex:   msg1=0, DerivedTx1=1
// Expected predecessors: [msg1]
func (suite *BackendTestSuite) TestTraceTransactionDerivedTxAsTarget() {
	suite.SetupTest()

	msg1, _ := suite.buildEthereumTx()
	txBz1 := suite.signAndEncodeEthTx(msg1)
	hash1 := msg1.AsTransaction().Hash()

	// Slot-1: non-EVM Cosmos tx that triggers the derived target.
	dummyTxBz, err := suite.backend.ClientCtx.TxConfig.TxEncoder()(
		suite.backend.ClientCtx.TxConfig.NewTxBuilder().GetTx(),
	)
	suite.Require().NoError(err)

	hashDerivedTarget := common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222")
	senderAddr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	recipientAddr := common.HexToAddress("0xabcdef1234567890abcdef1234567890abcdef12")
	gasLimitVal := uint64(50000)

	// KV index: only slot 0 (msg1); slot 1 is non-EVM and skipped.
	localBlock := types.MakeBlock(1, []types.Tx{txBz1, dummyTxBz}, nil, nil)
	localBlock.ChainID = ChainID
	indexResults := []*abci.ExecTxResult{
		{Code: 0, Events: []abci.Event{ethTxEvent(hash1.Hex(), "0")}},
		{Code: 0, Events: []abci.Event{}},
	}
	suite.backend.Indexer = indexer.NewKVIndexer(dbm.NewMemDB(), log.NewNopLogger(), suite.backend.ClientCtx)
	suite.Require().NoError(suite.backend.Indexer.IndexBlock(localBlock, indexResults))

	queryClient := suite.mockQueryClient()
	client := suite.mockClient()

	_, err = RegisterBlockMultipleTxs(client, 1, []types.Tx{txBz1, dummyTxBz})
	suite.Require().NoError(err)

	// GetTxByEthHash(hashDerivedTarget): KV miss → TxSearch. Returns derived tx with type=99.
	targetHashQuery := fmt.Sprintf("%s.%s='%s'",
		evmtypes.TypeMsgEthereumTx, evmtypes.AttributeKeyEthereumTxHash, hashDerivedTarget.Hex())
	RegisterTxSearchWithResult(client, targetHashQuery, 1, 1, nil,
		[]abci.Event{derivedTxEvt(hashDerivedTarget.Hex(), 1, senderAddr.Hex(), recipientAddr.Hex(), gasLimitVal)})

	// BlockResults for the after-loop derived-tx predecessor scan (slot1 contains target).
	RegisterBlockResultsWithTxs(client, 1, []*abci.ExecTxResult{
		{Code: 0, Events: []abci.Event{ethTxEvent(hash1.Hex(), "0")}},
		{Code: 0, Events: []abci.Event{derivedTxEvt(hashDerivedTarget.Hex(), 1, senderAddr.Hex(), recipientAddr.Hex(), gasLimitVal)}},
	})

	// Build the expected target Msg.
	derivedAdditional := &rpctypes.TxResultAdditionalFields{
		Hash:      hashDerivedTarget,
		Sender:    senderAddr,
		Recipient: recipientAddr,
		GasUsed:   21000,
		GasLimit:  &gasLimitVal,
		Type:      evmtypes.DerivedTxType,
	}
	derivedTargetMsg := suite.backend.parseDerivedTxFromAdditionalFields(derivedAdditional)
	suite.Require().NotNil(derivedTargetMsg)

	// Outer loop: i=0 → msg1 (KV hit, TxIndex=0 != target.TxIndex=1, added).
	// After-loop: derived scan finds target immediately and breaks — no extra predecessors.
	RegisterTraceTransactionWithPredecessors(queryClient, derivedTargetMsg, []*evmtypes.MsgEthereumTx{msg1})
	RegisterConsensusParams(client, 1)

	_, err = suite.backend.TraceTransaction(hashDerivedTarget, nil)
	suite.Require().NoError(err)
}
