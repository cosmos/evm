package indexer

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"

	dbm "github.com/cosmos/cosmos-db"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/client"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/indexer"
	"github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/vm/types"
)

// TestTransformerCosmosEventOnly tests that FinalizeBlockEvents create synthetic txs per phase.
// Events in the same phase are bundled into a single synthetic tx with multiple logs.
func TestTransformerCosmosEventOnly(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	nw := network.New(create, options...)
	encodingConfig := nw.GetEncodingConfig()
	clientCtx := client.Context{}.WithTxConfig(encodingConfig.TxConfig).WithCodec(encodingConfig.Codec)

	tokenAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	validReceiverAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())

	testCases := []struct {
		name           string
		finalizeEvents []abci.Event
		expTxCount     int // number of synthetic txs (1 per phase with transformable events)
		expLogCount    int // total number of logs in the receipt
	}{
		{
			name: "single event creates single synthetic tx with 1 log",
			finalizeEvents: []abci.Event{
				{
					Type: banktypes.EventTypeCoinReceived,
					Attributes: []abci.EventAttribute{
						{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
						{Key: sdk.AttributeKeyAmount, Value: "1000stake"},
					},
				},
			},
			expTxCount:  1,
			expLogCount: 1,
		},
		{
			name: "multiple events in same phase create single tx with multiple logs",
			finalizeEvents: []abci.Event{
				{
					Type: banktypes.EventTypeCoinReceived,
					Attributes: []abci.EventAttribute{
						{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
						{Key: sdk.AttributeKeyAmount, Value: "1000stake"},
					},
				},
				{
					Type: banktypes.EventTypeCoinSpent,
					Attributes: []abci.EventAttribute{
						{Key: banktypes.AttributeKeySpender, Value: validReceiverAddr.String()},
						{Key: sdk.AttributeKeyAmount, Value: "500stake"},
					},
				},
			},
			expTxCount:  1, // single phase = single tx
			expLogCount: 2, // 2 events = 2 logs in the receipt
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := dbm.NewMemDB()
			idxer := indexer.NewKVIndexer(db, log.NewNopLogger(), clientCtx)

			bankTransformer := NewBankTransferTransformer(tokenAddress)
			idxer.RegisterTransformer(bankTransformer)

			block := &cmttypes.Block{
				Header: cmttypes.Header{Height: 1},
				Data:   cmttypes.Data{Txs: []cmttypes.Tx{}},
			}

			err := idxer.IndexBlock(block, []*abci.ExecTxResult{}, tc.finalizeEvents)
			require.NoError(t, err)

			first, err := idxer.FirstIndexedBlock()
			require.NoError(t, err)
			require.Equal(t, int64(1), first)

			// Verify expected number of synthetic txs
			res, err := idxer.GetByBlockAndIndex(1, 0)
			require.NoError(t, err)
			require.NotNil(t, res)
			require.Equal(t, int64(1), res.Height)
			require.Equal(t, int32(0), res.EthTxIndex)

			// Verify no additional txs beyond expTxCount
			if tc.expTxCount == 1 {
				_, err = idxer.GetByBlockAndIndex(1, 1)
				require.Error(t, err, "should not have tx at index 1")
			}

			// Verify receipt has expected number of logs
			ethTxHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhasePreBlock), block.Hash())
			receiptJSON, err := idxer.GetEthReceipt(ethTxHash)
			require.NoError(t, err)

			var receipt ethtypes.Receipt
			err = json.Unmarshal(receiptJSON, &receipt)
			require.NoError(t, err)
			require.Len(t, receipt.Logs, tc.expLogCount, "receipt should have expected number of logs")

			// Verify log indices are sequential
			for i, log := range receipt.Logs {
				require.Equal(t, uint(i), log.Index, "log index should be sequential")
			}
		})
	}
}

func TestTransformerNoTransformerSkipsEvent(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	nw := network.New(create, options...)
	encodingConfig := nw.GetEncodingConfig()
	clientCtx := client.Context{}.WithTxConfig(encodingConfig.TxConfig).WithCodec(encodingConfig.Codec)

	db := dbm.NewMemDB()
	idxer := indexer.NewKVIndexer(db, log.NewNopLogger(), clientCtx)

	// No transformer registered

	validReceiverAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())

	block := &cmttypes.Block{
		Header: cmttypes.Header{Height: 1},
		Data:   cmttypes.Data{Txs: []cmttypes.Tx{}},
	}

	finalizeEvents := []abci.Event{
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "1000stake"},
			},
		},
	}

	err := idxer.IndexBlock(block, []*abci.ExecTxResult{}, finalizeEvents)
	require.NoError(t, err)

	first, err := idxer.FirstIndexedBlock()
	require.NoError(t, err)
	require.Equal(t, int64(-1), first)

	last, err := idxer.LastIndexedBlock()
	require.NoError(t, err)
	require.Equal(t, int64(-1), last)
}

func TestTransformerStakingDelegate(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	nw := network.New(create, options...)
	encodingConfig := nw.GetEncodingConfig()
	clientCtx := client.Context{}.WithTxConfig(encodingConfig.TxConfig).WithCodec(encodingConfig.Codec)

	stakingPrecompileAddr := common.HexToAddress("0x0000000000000000000000000000000000000800")
	validDelegatorAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())
	validValidatorAddr := sdk.ValAddress(common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd").Bytes())

	db := dbm.NewMemDB()
	idxer := indexer.NewKVIndexer(db, log.NewNopLogger(), clientCtx)

	stakingTransformer := NewStakingDelegateTransformer(stakingPrecompileAddr)
	idxer.RegisterTransformer(stakingTransformer)

	block := &cmttypes.Block{
		Header: cmttypes.Header{Height: 1},
		Data:   cmttypes.Data{Txs: []cmttypes.Tx{}},
	}

	finalizeEvents := []abci.Event{
		{
			Type: EventTypeDelegate,
			Attributes: []abci.EventAttribute{
				{Key: AttributeKeyDelegator, Value: validDelegatorAddr.String()},
				{Key: AttributeKeyValidator, Value: validValidatorAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "5000stake"},
			},
		},
	}

	err := idxer.IndexBlock(block, []*abci.ExecTxResult{}, finalizeEvents)
	require.NoError(t, err)

	first, err := idxer.FirstIndexedBlock()
	require.NoError(t, err)
	require.Equal(t, int64(1), first)

	res, err := idxer.GetByBlockAndIndex(1, 0)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, int64(1), res.Height)
	require.Equal(t, int32(0), res.EthTxIndex)
}

// TestTransformerMultipleTransformers tests that multiple transformers can handle
// different event types in the same phase, combining their logs into a single receipt.
func TestTransformerMultipleTransformers(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	nw := network.New(create, options...)
	encodingConfig := nw.GetEncodingConfig()
	clientCtx := client.Context{}.WithTxConfig(encodingConfig.TxConfig).WithCodec(encodingConfig.Codec)

	tokenAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	stakingPrecompileAddr := common.HexToAddress("0x0000000000000000000000000000000000000800")
	validReceiverAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())
	validDelegatorAddr := sdk.AccAddress(common.HexToAddress("0x2234567890123456789012345678901234567890").Bytes())
	validValidatorAddr := sdk.ValAddress(common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd").Bytes())

	db := dbm.NewMemDB()
	idxer := indexer.NewKVIndexer(db, log.NewNopLogger(), clientCtx)

	bankTransformer := NewBankTransferTransformer(tokenAddress)
	stakingTransformer := NewStakingDelegateTransformer(stakingPrecompileAddr)
	idxer.RegisterTransformer(bankTransformer)
	idxer.RegisterTransformer(stakingTransformer)

	block := &cmttypes.Block{
		Header: cmttypes.Header{Height: 1},
		Data:   cmttypes.Data{Txs: []cmttypes.Tx{}},
	}

	// Two different event types in the same phase (PreBlock - no mode attribute)
	finalizeEvents := []abci.Event{
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "1000stake"},
			},
		},
		{
			Type: EventTypeDelegate,
			Attributes: []abci.EventAttribute{
				{Key: AttributeKeyDelegator, Value: validDelegatorAddr.String()},
				{Key: AttributeKeyValidator, Value: validValidatorAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "5000stake"},
			},
		},
	}

	err := idxer.IndexBlock(block, []*abci.ExecTxResult{}, finalizeEvents)
	require.NoError(t, err)

	// Should have 1 synthetic tx (single phase with 2 events)
	res, err := idxer.GetByBlockAndIndex(1, 0)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, int64(1), res.Height)
	require.Equal(t, int32(0), res.EthTxIndex)

	// Verify no second tx
	_, err = idxer.GetByBlockAndIndex(1, 1)
	require.Error(t, err, "should not have tx at index 1")

	// Verify receipt has 2 logs (one from each transformer)
	ethTxHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhasePreBlock), block.Hash())
	receiptJSON, err := idxer.GetEthReceipt(ethTxHash)
	require.NoError(t, err)

	var receipt ethtypes.Receipt
	err = json.Unmarshal(receiptJSON, &receipt)
	require.NoError(t, err)
	require.Len(t, receipt.Logs, 2, "receipt should have 2 logs from different transformers")

	// Verify log indices are sequential
	require.Equal(t, uint(0), receipt.Logs[0].Index)
	require.Equal(t, uint(1), receipt.Logs[1].Index)
}

// TestTransformerLogIndexOrdering tests that multiple events in a phase
// produce logs with sequential indices in a single receipt.
func TestTransformerLogIndexOrdering(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	nw := network.New(create, options...)
	encodingConfig := nw.GetEncodingConfig()
	clientCtx := client.Context{}.WithTxConfig(encodingConfig.TxConfig).WithCodec(encodingConfig.Codec)

	tokenAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	validReceiverAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())

	db := dbm.NewMemDB()
	idxer := indexer.NewKVIndexer(db, log.NewNopLogger(), clientCtx)

	bankTransformer := NewBankTransferTransformer(tokenAddress)
	idxer.RegisterTransformer(bankTransformer)

	block := &cmttypes.Block{
		Header: cmttypes.Header{Height: 5},
		Data:   cmttypes.Data{Txs: []cmttypes.Tx{}},
	}

	// 3 events in the same phase
	finalizeEvents := []abci.Event{
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "100stake"},
			},
		},
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "200stake"},
			},
		},
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "300stake"},
			},
		},
	}

	err := idxer.IndexBlock(block, []*abci.ExecTxResult{}, finalizeEvents)
	require.NoError(t, err)

	// Should have 1 synthetic tx (single phase)
	res, err := idxer.GetByBlockAndIndex(5, 0)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, int64(5), res.Height)
	require.Equal(t, int32(0), res.EthTxIndex)

	// Verify no additional txs
	_, err = idxer.GetByBlockAndIndex(5, 1)
	require.Error(t, err, "should not have tx at index 1")

	// Verify receipt has 3 logs with sequential indices
	ethTxHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhasePreBlock), block.Hash())
	receiptJSON, err := idxer.GetEthReceipt(ethTxHash)
	require.NoError(t, err)

	var receipt ethtypes.Receipt
	err = json.Unmarshal(receiptJSON, &receipt)
	require.NoError(t, err)
	require.Len(t, receipt.Logs, 3, "receipt should have 3 logs")

	for i, log := range receipt.Logs {
		require.Equal(t, uint(i), log.Index, "log index should be sequential")
	}
}

// TestTransformerMultiplePhases tests that PreBlock, BeginBlock, and EndBlock
// each create separate synthetic txs with correct phase-based hashes.
func TestTransformerMultiplePhases(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	nw := network.New(create, options...)
	encodingConfig := nw.GetEncodingConfig()
	clientCtx := client.Context{}.WithTxConfig(encodingConfig.TxConfig).WithCodec(encodingConfig.Codec)

	tokenAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	validReceiverAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())

	db := dbm.NewMemDB()
	idxer := indexer.NewKVIndexer(db, log.NewNopLogger(), clientCtx)

	bankTransformer := NewBankTransferTransformer(tokenAddress)
	idxer.RegisterTransformer(bankTransformer)

	block := &cmttypes.Block{
		Header: cmttypes.Header{Height: 1},
		Data:   cmttypes.Data{Txs: []cmttypes.Tx{}},
	}

	// Events in 3 different phases (distinguished by mode attribute)
	finalizeEvents := []abci.Event{
		// PreBlock event (no mode attribute)
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "100stake"},
			},
		},
		// BeginBlock event
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "200stake"},
				{Key: "mode", Value: "BeginBlock"},
			},
		},
		// EndBlock event
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "300stake"},
				{Key: "mode", Value: "EndBlock"},
			},
		},
	}

	err := idxer.IndexBlock(block, []*abci.ExecTxResult{}, finalizeEvents)
	require.NoError(t, err)

	// Should have 3 synthetic txs (one per phase)
	for i := int32(0); i < 3; i++ {
		res, err := idxer.GetByBlockAndIndex(1, i)
		require.NoError(t, err, "should have tx at index %d", i)
		require.NotNil(t, res)
		require.Equal(t, int64(1), res.Height)
		require.Equal(t, i, res.EthTxIndex)
	}

	// Verify no 4th tx
	_, err = idxer.GetByBlockAndIndex(1, 3)
	require.Error(t, err, "should not have tx at index 3")

	// Verify each phase has correct receipt with different hash
	preBlockHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhasePreBlock), block.Hash())
	beginBlockHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhaseBeginBlock), block.Hash())
	endBlockHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhaseEndBlock), block.Hash())

	// All hashes should be different
	require.NotEqual(t, preBlockHash, beginBlockHash)
	require.NotEqual(t, beginBlockHash, endBlockHash)
	require.NotEqual(t, preBlockHash, endBlockHash)

	// Each phase should have a receipt
	for _, hash := range []common.Hash{preBlockHash, beginBlockHash, endBlockHash} {
		receiptJSON, err := idxer.GetEthReceipt(hash)
		require.NoError(t, err)
		require.NotEmpty(t, receiptJSON)

		var receipt ethtypes.Receipt
		err = json.Unmarshal(receiptJSON, &receipt)
		require.NoError(t, err)
		require.Len(t, receipt.Logs, 1, "each phase receipt should have 1 log")
	}
}

// TestTransformerPhaseEthTxIndexOrdering tests that EthTxIndex is sequential
// across phases: PreBlock(0) → BeginBlock(1) → EndBlock(2)
func TestTransformerPhaseEthTxIndexOrdering(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	nw := network.New(create, options...)
	encodingConfig := nw.GetEncodingConfig()
	clientCtx := client.Context{}.WithTxConfig(encodingConfig.TxConfig).WithCodec(encodingConfig.Codec)

	tokenAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	validReceiverAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())

	db := dbm.NewMemDB()
	idxer := indexer.NewKVIndexer(db, log.NewNopLogger(), clientCtx)

	bankTransformer := NewBankTransferTransformer(tokenAddress)
	idxer.RegisterTransformer(bankTransformer)

	block := &cmttypes.Block{
		Header: cmttypes.Header{Height: 10},
		Data:   cmttypes.Data{Txs: []cmttypes.Tx{}},
	}

	// Events in all 3 phases
	finalizeEvents := []abci.Event{
		// PreBlock
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "100stake"},
			},
		},
		// BeginBlock
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "200stake"},
				{Key: "mode", Value: "BeginBlock"},
			},
		},
		// EndBlock
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "300stake"},
				{Key: "mode", Value: "EndBlock"},
			},
		},
	}

	err := idxer.IndexBlock(block, []*abci.ExecTxResult{}, finalizeEvents)
	require.NoError(t, err)

	// Verify EthTxIndex ordering: PreBlock=0, BeginBlock=1, EndBlock=2
	preBlockHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhasePreBlock), block.Hash())
	beginBlockHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhaseBeginBlock), block.Hash())
	endBlockHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhaseEndBlock), block.Hash())

	preBlockResult, err := idxer.GetByTxHash(preBlockHash)
	require.NoError(t, err)
	require.Equal(t, int32(0), preBlockResult.EthTxIndex, "PreBlock should have EthTxIndex 0")

	beginBlockResult, err := idxer.GetByTxHash(beginBlockHash)
	require.NoError(t, err)
	require.Equal(t, int32(1), beginBlockResult.EthTxIndex, "BeginBlock should have EthTxIndex 1")

	endBlockResult, err := idxer.GetByTxHash(endBlockHash)
	require.NoError(t, err)
	require.Equal(t, int32(2), endBlockResult.EthTxIndex, "EndBlock should have EthTxIndex 2")

	// Verify GetByBlockAndIndex returns correct results
	res0, err := idxer.GetByBlockAndIndex(10, 0)
	require.NoError(t, err)
	require.Equal(t, preBlockResult, res0)

	res1, err := idxer.GetByBlockAndIndex(10, 1)
	require.NoError(t, err)
	require.Equal(t, beginBlockResult, res1)

	res2, err := idxer.GetByBlockAndIndex(10, 2)
	require.NoError(t, err)
	require.Equal(t, endBlockResult, res2)
}

// TestTransformerEmptyPhaseSkipped tests that phases without transformable events
// do not create synthetic txs, and EthTxIndex remains sequential.
func TestTransformerEmptyPhaseSkipped(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	nw := network.New(create, options...)
	encodingConfig := nw.GetEncodingConfig()
	clientCtx := client.Context{}.WithTxConfig(encodingConfig.TxConfig).WithCodec(encodingConfig.Codec)

	tokenAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	validReceiverAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())

	db := dbm.NewMemDB()
	idxer := indexer.NewKVIndexer(db, log.NewNopLogger(), clientCtx)

	bankTransformer := NewBankTransferTransformer(tokenAddress)
	idxer.RegisterTransformer(bankTransformer)

	block := &cmttypes.Block{
		Header: cmttypes.Header{Height: 1},
		Data:   cmttypes.Data{Txs: []cmttypes.Tx{}},
	}

	// Only PreBlock and EndBlock have events (BeginBlock is empty)
	finalizeEvents := []abci.Event{
		// PreBlock
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "100stake"},
			},
		},
		// EndBlock (skipping BeginBlock)
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "300stake"},
				{Key: "mode", Value: "EndBlock"},
			},
		},
	}

	err := idxer.IndexBlock(block, []*abci.ExecTxResult{}, finalizeEvents)
	require.NoError(t, err)

	// Should have 2 synthetic txs (PreBlock and EndBlock only)
	res0, err := idxer.GetByBlockAndIndex(1, 0)
	require.NoError(t, err)
	require.Equal(t, int32(0), res0.EthTxIndex)

	res1, err := idxer.GetByBlockAndIndex(1, 1)
	require.NoError(t, err)
	require.Equal(t, int32(1), res1.EthTxIndex)

	// No 3rd tx
	_, err = idxer.GetByBlockAndIndex(1, 2)
	require.Error(t, err, "should not have tx at index 2")

	// Verify PreBlock receipt exists
	preBlockHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhasePreBlock), block.Hash())
	_, err = idxer.GetEthReceipt(preBlockHash)
	require.NoError(t, err)

	// Verify BeginBlock receipt does NOT exist
	beginBlockHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhaseBeginBlock), block.Hash())
	_, err = idxer.GetEthReceipt(beginBlockHash)
	require.Error(t, err, "BeginBlock should not have receipt")

	// Verify EndBlock receipt exists
	endBlockHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhaseEndBlock), block.Hash())
	_, err = idxer.GetEthReceipt(endBlockHash)
	require.NoError(t, err)
}

// TestTransformerMixedPhasesAndDeliverTx tests EthTxIndex ordering when
// FinalizeBlockEvents (phases) and DeliverTx (eth txs) are mixed together.
// Expected order: BeginBlock(0) → DeliverTx(1,2,3) → EndBlock(4)
func TestTransformerMixedPhasesAndDeliverTx(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	nw := network.New(create, options...)
	encodingConfig := nw.GetEncodingConfig()
	clientCtx := client.Context{}.WithTxConfig(encodingConfig.TxConfig).WithCodec(encodingConfig.Codec)

	tokenAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	validReceiverAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())

	// Create 3 eth transactions
	var txHashes []common.Hash
	var txBytes []cmttypes.Tx
	var txResults []*abci.ExecTxResult

	for i := 0; i < 3; i++ {
		priv, err := ethsecp256k1.GenerateKey()
		require.NoError(t, err)
		from := common.BytesToAddress(priv.PubKey().Address().Bytes())
		signer := utiltx.NewSigner(priv)
		ethSigner := ethtypes.LatestSignerForChainID(nil)

		to := common.BigToAddress(big.NewInt(int64(i + 1)))
		ethTxParams := types.EvmTxArgs{
			Nonce:    0,
			To:       &to,
			Amount:   big.NewInt(1000),
			GasLimit: 21000,
		}
		tx := types.NewTx(&ethTxParams)
		tx.From = from.Bytes()
		require.NoError(t, tx.Sign(ethSigner, signer))
		txHash := tx.AsTransaction().Hash()
		txHashes = append(txHashes, txHash)

		tmTx, err := tx.BuildTx(clientCtx.TxConfig.NewTxBuilder(), constants.ExampleAttoDenom)
		require.NoError(t, err)
		txBz, err := clientCtx.TxConfig.TxEncoder()(tmTx)
		require.NoError(t, err)
		txBytes = append(txBytes, txBz)

		txResults = append(txResults, &abci.ExecTxResult{
			Code: 0,
			Events: []abci.Event{
				{Type: types.EventTypeEthereumTx, Attributes: []abci.EventAttribute{
					{Key: "ethereumTxHash", Value: txHash.Hex()},
					{Key: "txIndex", Value: "0"},
					{Key: "amount", Value: "1000"},
					{Key: "txGasUsed", Value: "21000"},
					{Key: "txHash", Value: ""},
					{Key: "recipient", Value: to.Hex()},
				}},
			},
		})
	}

	db := dbm.NewMemDB()
	idxer := indexer.NewKVIndexer(db, log.NewNopLogger(), clientCtx)

	bankTransformer := NewBankTransferTransformer(tokenAddress)
	idxer.RegisterTransformer(bankTransformer)

	block := &cmttypes.Block{
		Header: cmttypes.Header{Height: 1},
		Data:   cmttypes.Data{Txs: txBytes},
	}

	// FinalizeBlockEvents: BeginBlock and EndBlock (no PreBlock)
	finalizeEvents := []abci.Event{
		// BeginBlock event
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "100stake"},
				{Key: "mode", Value: "BeginBlock"},
			},
		},
		// EndBlock event
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "200stake"},
				{Key: "mode", Value: "EndBlock"},
			},
		},
	}

	err := idxer.IndexBlock(block, txResults, finalizeEvents)
	require.NoError(t, err)

	// Expected EthTxIndex ordering:
	// BeginBlock: 0
	// DeliverTx[0]: 1
	// DeliverTx[1]: 2
	// DeliverTx[2]: 3
	// EndBlock: 4

	// Verify BeginBlock has EthTxIndex = 0
	beginBlockHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhaseBeginBlock), block.Hash())
	beginBlockResult, err := idxer.GetByTxHash(beginBlockHash)
	require.NoError(t, err)
	require.Equal(t, int32(0), beginBlockResult.EthTxIndex, "BeginBlock should have EthTxIndex 0")

	// Verify DeliverTx txs have EthTxIndex = 1, 2, 3
	for i, txHash := range txHashes {
		result, err := idxer.GetByTxHash(txHash)
		require.NoError(t, err)
		expectedIdx := int32(i + 1) // 1, 2, 3
		require.Equal(t, expectedIdx, result.EthTxIndex, "DeliverTx[%d] should have EthTxIndex %d", i, expectedIdx)
	}

	// Verify EndBlock has EthTxIndex = 4
	endBlockHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhaseEndBlock), block.Hash())
	endBlockResult, err := idxer.GetByTxHash(endBlockHash)
	require.NoError(t, err)
	require.Equal(t, int32(4), endBlockResult.EthTxIndex, "EndBlock should have EthTxIndex 4")

	// Verify GetByBlockAndIndex returns correct results
	for i := int32(0); i <= 4; i++ {
		res, err := idxer.GetByBlockAndIndex(1, i)
		require.NoError(t, err, "should have tx at index %d", i)
		require.Equal(t, i, res.EthTxIndex)
	}

	// Verify no 6th tx
	_, err = idxer.GetByBlockAndIndex(1, 5)
	require.Error(t, err, "should not have tx at index 5")
}

// TestTransformerCumulativeGasUsed tests that CumulativeGasUsed is correctly
// accumulated across multiple synthetic txs in a block per Ethereum spec.
func TestTransformerCumulativeGasUsed(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	nw := network.New(create, options...)
	encodingConfig := nw.GetEncodingConfig()
	clientCtx := client.Context{}.WithTxConfig(encodingConfig.TxConfig).WithCodec(encodingConfig.Codec)

	tokenAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	validReceiverAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())

	db := dbm.NewMemDB()
	idxer := indexer.NewKVIndexer(db, log.NewNopLogger(), clientCtx)

	bankTransformer := NewBankTransferTransformer(tokenAddress)
	idxer.RegisterTransformer(bankTransformer)

	block := &cmttypes.Block{
		Header: cmttypes.Header{Height: 1},
		Data:   cmttypes.Data{Txs: []cmttypes.Tx{}},
	}

	// Events in 3 different phases - each phase creates a synthetic tx with GasUsed=21000
	finalizeEvents := []abci.Event{
		// PreBlock event
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "100stake"},
			},
		},
		// BeginBlock event
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "200stake"},
				{Key: "mode", Value: "BeginBlock"},
			},
		},
		// EndBlock event
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "300stake"},
				{Key: "mode", Value: "EndBlock"},
			},
		},
	}

	err := idxer.IndexBlock(block, []*abci.ExecTxResult{}, finalizeEvents)
	require.NoError(t, err)

	// Each phase's transformer returns GasUsed=21000
	// Expected CumulativeGasUsed:
	// - PreBlock (tx 0): 21000
	// - BeginBlock (tx 1): 42000
	// - EndBlock (tx 2): 63000

	preBlockHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhasePreBlock), block.Hash())
	beginBlockHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhaseBeginBlock), block.Hash())
	endBlockHash := indexer.GenerateTransformedEthTxHash([]byte(indexer.BlockPhaseEndBlock), block.Hash())

	// Verify PreBlock receipt CumulativeGasUsed
	preBlockReceiptJSON, err := idxer.GetEthReceipt(preBlockHash)
	require.NoError(t, err)
	var preBlockReceipt ethtypes.Receipt
	err = json.Unmarshal(preBlockReceiptJSON, &preBlockReceipt)
	require.NoError(t, err)
	require.Equal(t, uint64(21000), preBlockReceipt.GasUsed, "PreBlock GasUsed should be 21000")
	require.Equal(t, uint64(21000), preBlockReceipt.CumulativeGasUsed, "PreBlock CumulativeGasUsed should be 21000")

	// Verify BeginBlock receipt CumulativeGasUsed
	beginBlockReceiptJSON, err := idxer.GetEthReceipt(beginBlockHash)
	require.NoError(t, err)
	var beginBlockReceipt ethtypes.Receipt
	err = json.Unmarshal(beginBlockReceiptJSON, &beginBlockReceipt)
	require.NoError(t, err)
	require.Equal(t, uint64(21000), beginBlockReceipt.GasUsed, "BeginBlock GasUsed should be 21000")
	require.Equal(t, uint64(42000), beginBlockReceipt.CumulativeGasUsed, "BeginBlock CumulativeGasUsed should be 42000")

	// Verify EndBlock receipt CumulativeGasUsed
	endBlockReceiptJSON, err := idxer.GetEthReceipt(endBlockHash)
	require.NoError(t, err)
	var endBlockReceipt ethtypes.Receipt
	err = json.Unmarshal(endBlockReceiptJSON, &endBlockReceipt)
	require.NoError(t, err)
	require.Equal(t, uint64(21000), endBlockReceipt.GasUsed, "EndBlock GasUsed should be 21000")
	require.Equal(t, uint64(63000), endBlockReceipt.CumulativeGasUsed, "EndBlock CumulativeGasUsed should be 63000")
}
