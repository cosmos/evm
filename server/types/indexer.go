package types

import (
	"github.com/ethereum/go-ethereum/common"

	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
)

// EVMTxIndexer defines the interface of custom eth tx indexer.
type EVMTxIndexer interface {
	// LastIndexedBlock returns -1 if indexer db is empty
	LastIndexedBlock() (int64, error)
	// IndexBlock indexes all eth txs in a block.
	// finalizeBlockEvents contains BeginBlock + EndBlock events (FinalizeBlockEvents in CometBFT v0.38+)
	IndexBlock(block *cmttypes.Block, txResults []*abci.ExecTxResult, finalizeBlockEvents []abci.Event) error

	// GetByTxHash returns nil if tx not found.
	GetByTxHash(common.Hash) (*TxResult, error)
	// GetByBlockAndIndex returns nil if tx not found.
	GetByBlockAndIndex(int64, int32) (*TxResult, error)

	// GetEthReceipt returns the stored eth receipt JSON by tx hash.
	// Returns error if not found.
	GetEthReceipt(common.Hash) ([]byte, error)
	// GetEthTx returns the stored eth tx JSON by tx hash.
	// Returns error if not found.
	GetEthTx(common.Hash) ([]byte, error)
}
