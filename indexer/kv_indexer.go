package indexer

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log/v2"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"

	servertypes "github.com/cosmos/evm/server/types"
)

const (
	KeyPrefixTxHash     = 1
	KeyPrefixTxIndex    = 2
	KeyPrefixEthReceipt = 3
	KeyPrefixEthTx      = 4

	// TxIndexKeyLength is the length of tx-index key
	TxIndexKeyLength = 1 + 8 + 8
)

var _ servertypes.EVMTxIndexer = &KVIndexer{}

// KVIndexer implements a eth tx indexer on a KV db.
type KVIndexer struct {
	db           dbm.DB
	logger       log.Logger
	clientCtx    client.Context
	transformers []CosmosEventTransformer
	*ethTxIndexingContext
}

type ethTxIndexingContext struct {
	// Block indexing state (reset at start of IndexBlock)
	batch             dbm.Batch
	ethTxIndex        int32
	cumulativeGasUsed uint64
	processedEthTx    map[common.Hash]interface{}
}

func (kv *KVIndexer) Reset() {
	kv.ethTxIndexingContext = &ethTxIndexingContext{
		batch:          kv.db.NewBatch(),
		processedEthTx: make(map[common.Hash]interface{}),
	}
}

// IsProcessed checks if an eth tx hash has already been processed.
func (ctx *ethTxIndexingContext) IsProcessed(hash common.Hash) bool {
	return ctx.processedEthTx[hash] != nil
}

// NextTx marks the hash as processed, advances the eth tx index, and accumulates gas.
// Returns the current index (before increment) and updated cumulative gas.
func (ctx *ethTxIndexingContext) NextTx(hash common.Hash, gasUsed uint64) (currentIndex int32, cumulativeGas uint64) {
	ctx.processedEthTx[hash] = struct{}{}
	ctx.cumulativeGasUsed += gasUsed
	currentIndex = ctx.ethTxIndex
	cumulativeGas = ctx.cumulativeGasUsed
	ctx.ethTxIndex++
	return
}

// NewKVIndexer creates the KVIndexer
func NewKVIndexer(db dbm.DB, logger log.Logger, clientCtx client.Context) *KVIndexer {
	return &KVIndexer{
		db:           db,
		logger:       logger,
		clientCtx:    clientCtx,
		transformers: []CosmosEventTransformer{},
	}
}

// RegisterTransformer registers a cosmos event transformer.
// transformers can be registered externally by the caller.
func (kv *KVIndexer) RegisterTransformer(t CosmosEventTransformer) {
	kv.transformers = append(kv.transformers, t)
}

// findTransformer returns a transformer that can handle the given event type
func (kv *KVIndexer) findTransformer(eventType string) CosmosEventTransformer {
	for _, t := range kv.transformers {
		if t.CanHandle(eventType) {
			return t
		}
	}
	return nil
}

// IndexBlock indexes all eth txs in a block through the following steps:
// 1. Process FinalizeBlockEvents (PreBlock, BeginBlock, EndBlock) - create synthetic txs for transformable events
// 2. Process DeliverTx events with single loop + buffering:
//   - Buffer cosmos events that belong to eth txs
//   - When ethereum_tx event is seen, process it and append buffered cosmos logs
//   - For cosmos-only txs, create synthetic txs immediately
func (kv *KVIndexer) IndexBlock(block *cmttypes.Block, txResults []*abci.ExecTxResult, finalizeBlockEvents []abci.Event) error {
	kv.Reset()
	defer kv.batch.Close()

	preblockEvents, beginBlockEvents, endBlockEvents := partitionFinalizeBlockEvents(finalizeBlockEvents)

	// Process PreBlock events (single synthetic tx per phase)
	if err := kv.processBlockPhaseEvents(block, BlockPhasePreBlock, preblockEvents); err != nil {
		return err
	}

	// Process BeginBlock events (single synthetic tx per phase)
	if err := kv.processBlockPhaseEvents(block, BlockPhaseBeginBlock, beginBlockEvents); err != nil {
		return err
	}

	// Process DeliverTx events
	if err := kv.processDeliverTxEvents(block, txResults); err != nil {
		return err
	}

	// Process EndBlock events (single synthetic tx per phase)
	if err := kv.processBlockPhaseEvents(block, BlockPhaseEndBlock, endBlockEvents); err != nil {
		return err
	}

	if err := kv.batch.Write(); err != nil {
		return errorsmod.Wrapf(err, "IndexBlock %d, write batch", block.Height)
	}
	return nil
}

// LastIndexedBlock returns the latest indexed block number, returns -1 if db is empty
func (kv *KVIndexer) LastIndexedBlock() (int64, error) {
	return LoadLastBlock(kv.db)
}

// FirstIndexedBlock returns the first indexed block number, returns -1 if db is empty
func (kv *KVIndexer) FirstIndexedBlock() (int64, error) {
	return LoadFirstBlock(kv.db)
}

// GetByTxHash finds eth tx by eth tx hash
func (kv *KVIndexer) GetByTxHash(hash common.Hash) (*servertypes.TxResult, error) {
	bz, err := kv.db.Get(TxHashKey(hash))
	if err != nil {
		return nil, errorsmod.Wrapf(err, "GetByTxHash %s", hash.Hex())
	}
	if len(bz) == 0 {
		return nil, fmt.Errorf("tx not found, hash: %s", hash.Hex())
	}
	var txKey servertypes.TxResult
	if err := kv.clientCtx.Codec.Unmarshal(bz, &txKey); err != nil {
		return nil, errorsmod.Wrapf(err, "GetByTxHash %s", hash.Hex())
	}
	return &txKey, nil
}

// GetByBlockAndIndex finds eth tx by block number and eth tx index
func (kv *KVIndexer) GetByBlockAndIndex(blockNumber int64, txIndex int32) (*servertypes.TxResult, error) {
	bz, err := kv.db.Get(TxIndexKey(blockNumber, txIndex))
	if err != nil {
		return nil, errorsmod.Wrapf(err, "GetByBlockAndIndex %d %d", blockNumber, txIndex)
	}
	if len(bz) == 0 {
		return nil, fmt.Errorf("tx not found, block: %d, eth-index: %d", blockNumber, txIndex)
	}
	return kv.GetByTxHash(common.BytesToHash(bz))
}

// GetEthReceipt returns the stored eth receipt JSON by tx hash
func (kv *KVIndexer) GetEthReceipt(hash common.Hash) ([]byte, error) {
	bz, err := kv.db.Get(EthReceiptKey(hash))
	if err != nil {
		return nil, errorsmod.Wrapf(err, "GetEthReceipt %s", hash.Hex())
	}
	if len(bz) == 0 {
		return nil, fmt.Errorf("eth receipt not found, hash: %s", hash.Hex())
	}
	return bz, nil
}

// GetEthTx returns the stored eth tx JSON by tx hash
func (kv *KVIndexer) GetEthTx(hash common.Hash) ([]byte, error) {
	bz, err := kv.db.Get(EthTxKey(hash))
	if err != nil {
		return nil, errorsmod.Wrapf(err, "GetEthTx %s", hash.Hex())
	}
	if len(bz) == 0 {
		return nil, fmt.Errorf("eth tx not found, hash: %s", hash.Hex())
	}
	return bz, nil
}

// TxHashKey returns the key for db entry: `tx hash -> tx result struct`
func TxHashKey(hash common.Hash) []byte {
	return append([]byte{KeyPrefixTxHash}, hash.Bytes()...)
}

// TxIndexKey returns the key for db entry: `(block number, tx index) -> tx hash`
func TxIndexKey(blockNumber int64, txIndex int32) []byte {
	bz1 := sdk.Uint64ToBigEndian(uint64(blockNumber)) //nolint:gosec // G115 // block number won't exceed uint64
	bz2 := sdk.Uint64ToBigEndian(uint64(txIndex))     //nolint:gosec // G115 // index won't exceed uint64
	return append(append([]byte{KeyPrefixTxIndex}, bz1...), bz2...)
}

// EthReceiptKey returns the key for db entry: `tx hash -> eth receipt JSON`
func EthReceiptKey(hash common.Hash) []byte {
	return append([]byte{KeyPrefixEthReceipt}, hash.Bytes()...)
}

// EthTxKey returns the key for db entry: `tx hash -> eth tx JSON`
func EthTxKey(hash common.Hash) []byte {
	return append([]byte{KeyPrefixEthTx}, hash.Bytes()...)
}

// LoadLastBlock returns the latest indexed block number, returns -1 if db is empty
func LoadLastBlock(db dbm.DB) (int64, error) {
	it, err := db.ReverseIterator([]byte{KeyPrefixTxIndex}, []byte{KeyPrefixTxIndex + 1})
	if err != nil {
		return 0, errorsmod.Wrap(err, "LoadLastBlock")
	}
	defer it.Close()
	if !it.Valid() {
		return -1, nil
	}
	return parseBlockNumberFromKey(it.Key())
}

// LoadFirstBlock loads the first indexed block, returns -1 if db is empty
func LoadFirstBlock(db dbm.DB) (int64, error) {
	it, err := db.Iterator([]byte{KeyPrefixTxIndex}, []byte{KeyPrefixTxIndex + 1})
	if err != nil {
		return 0, errorsmod.Wrap(err, "LoadFirstBlock")
	}
	defer it.Close()
	if !it.Valid() {
		return -1, nil
	}
	return parseBlockNumberFromKey(it.Key())
}

// isEthTx check if the tx is an eth tx
func isEthTx(tx sdk.Tx) bool {
	extTx, ok := tx.(authante.HasExtensionOptionsTx)
	if !ok {
		return false
	}
	opts := extTx.GetExtensionOptions()
	if len(opts) != 1 || opts[0].GetTypeUrl() != "/cosmos.evm.vm.v1.ExtensionOptionsEthereumTx" {
		return false
	}
	return true
}

// saveTxResult index the txResult into the kv db batch
func saveTxResult(codec codec.Codec, batch dbm.Batch, txHash common.Hash, txResult *servertypes.TxResult) error {
	bz := codec.MustMarshal(txResult)
	if err := batch.Set(TxHashKey(txHash), bz); err != nil {
		return errorsmod.Wrap(err, "set tx-hash key")
	}
	if err := batch.Set(TxIndexKey(txResult.Height, txResult.EthTxIndex), txHash.Bytes()); err != nil {
		return errorsmod.Wrap(err, "set tx-index key")
	}
	return nil
}

func parseBlockNumberFromKey(key []byte) (int64, error) {
	if len(key) != TxIndexKeyLength {
		return 0, fmt.Errorf("wrong tx index key length, expect: %d, got: %d", TxIndexKeyLength, len(key))
	}

	return int64(sdk.BigEndianToUint64(key[1:9])), nil //#nosec G115 -- int overflow is not a concern here, block number is unlikely to exceed 9,223,372,036,854,775,807
}
