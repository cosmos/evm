package mempool

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"

	"github.com/cosmos/evm/mempool/txpool"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkerrors "cosmossdk.io/errors"
	"cosmossdk.io/log"
	sdktypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	_ txpool.BlockChain     = &Blockchain{}
	_ legacypool.BlockChain = &Blockchain{}
)

// Blockchain implements the BlockChain interface required by Ethereum transaction pools.
// It bridges Cosmos SDK blockchain state with Ethereum's transaction pool system by providing
// access to block headers, chain configuration, and state databases. This implementation is
// specifically designed for instant finality chains where reorgs never occur.
type Blockchain struct {
	getCtxCallback     func(height int64, prove bool) (sdk.Context, error)
	logger             log.Logger
	vmKeeper           VMKeeperI
	feeMarketKeeper    FeeMarketKeeperI
	chainHeadFeed      *event.Feed
	zeroHeader         *types.Header
	blockGasLimit      uint64
	previousHeaderHash common.Hash
	latestCtx          sdk.Context
	mu                 sync.RWMutex

	testingCommitMu sync.RWMutex
}

// NewBlockchain creates a new Blockchain instance that bridges Cosmos SDK state with Ethereum mempools.
// The getCtxCallback function provides access to Cosmos SDK contexts at different heights, vmKeeper manages EVM state,
// and feeMarketKeeper handles fee market operations like base fee calculations.
func NewBlockchain(ctx func(height int64, prove bool) (sdk.Context, error), logger log.Logger, vmKeeper VMKeeperI, feeMarketKeeper FeeMarketKeeperI, blockGasLimit uint64) *Blockchain {
	// Add the blockchain name to the logger
	logger = logger.With(log.ModuleKey, "Blockchain")

	logger.Debug("creating new blockchain instance", "block_gas_limit", blockGasLimit)

	return &Blockchain{
		getCtxCallback:  ctx,
		logger:          logger,
		vmKeeper:        vmKeeper,
		feeMarketKeeper: feeMarketKeeper,
		chainHeadFeed:   new(event.Feed),
		blockGasLimit:   blockGasLimit,
		// Used as a placeholder for the first block, before the context is available.
		zeroHeader: &types.Header{
			Difficulty: big.NewInt(0),
			Number:     big.NewInt(0),
		},
	}
}

// Config returns the Ethereum chain configuration. It should only be called after the chain is initialized.
// This provides the necessary parameters for EVM execution and transaction validation.
func (b *Blockchain) Config() *params.ChainConfig {
	return evmtypes.GetEthChainConfig()
}

// CurrentBlock returns the current block header for the app.
// It constructs an Ethereum-compatible header from the current Cosmos SDK context,
// including block height, timestamp, gas limits, and base fee (if London fork is active).
// Returns a zero header as placeholder if the context is not yet available.
func (b *Blockchain) CurrentBlock() *types.Header {
	ctx, err := b.GetLatestContext()
	if err != nil {
		return b.zeroHeader
	}

	blockHeight := ctx.BlockHeight()
	// prevent the reorg from triggering after a restart since previousHeaderHash is stored as an in-memory variable
	previousHeaderHash := b.getPreviousHeaderHash()
	if blockHeight > 1 && previousHeaderHash == (common.Hash{}) {
		return b.zeroHeader
	}

	blockTime := ctx.BlockTime().Unix()
	gasUsed := b.feeMarketKeeper.GetBlockGasWanted(ctx)
	appHash := common.BytesToHash(ctx.BlockHeader().AppHash)

	header := &types.Header{
		Number:     big.NewInt(blockHeight),
		Time:       uint64(blockTime), // #nosec G115 -- overflow not a concern with unix time
		GasLimit:   b.blockGasLimit,
		GasUsed:    gasUsed,
		ParentHash: previousHeaderHash,
		Root:       appHash,       // we actually don't care that this isn't the getCtxCallback header, as long as we properly track roots and parent roots to prevent the reorg from triggering
		Difficulty: big.NewInt(0), // 0 difficulty on PoS
	}

	chainConfig := evmtypes.GetEthChainConfig()
	if chainConfig.IsLondon(header.Number) {
		baseFee := b.vmKeeper.GetBaseFee(ctx)
		if baseFee != nil {
			header.BaseFee = baseFee
			b.logger.Debug("added base fee to header", "base_fee", baseFee.String())
		} else {
			b.logger.Debug("no base fee available for London fork")
		}
	} else {
		b.logger.Debug("London fork not active for current block", "block_number", header.Number.String())
	}

	b.logger.Debug("current block header constructed",
		"header_hash", header.Hash().Hex(),
		"number", header.Number.String(),
		"time", header.Time,
		"gas_limit", header.GasLimit,
		"gas_used", header.GasUsed,
		"parent_hash", header.ParentHash.Hex(),
		"root", header.Root.Hex(),
		"difficulty", header.Difficulty.String(),
		"base_fee", func() string {
			if header.BaseFee != nil {
				return header.BaseFee.String()
			}
			return "nil"
		}())
	return header
}

// GetBlock retrieves a block by hash and number.
// Cosmos chains have instant finality, so  this method should only be called for the genesis block (block 0)
// or block 1, as reorgs never occur. Any other call indicates a bug in the mempool logic.
// Panics if called for blocks beyond block 1, as this would indicate an attempted reorg.
func (b *Blockchain) GetBlock(_ common.Hash, _ uint64) *types.Block {
	currBlock := b.CurrentBlock()
	blockNumber := currBlock.Number.Int64()

	b.logger.Debug("GetBlock called", "block_number", blockNumber)

	switch blockNumber {
	case 0:
		b.logger.Debug("returning genesis block", "block_number", blockNumber)
		currBlock.ParentHash = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
		return types.NewBlockWithHeader(currBlock)
	case 1:
		b.logger.Debug("returning block 1", "block_number", blockNumber)
		return types.NewBlockWithHeader(currBlock)
	}

	b.logger.Error("GetBlock called for invalid block number - this indicates a reorg attempt", "block_number", blockNumber)
	panic("GetBlock should never be called on a Cosmos chain due to instant finality - this indicates a reorg is being attempted")
}

// SubscribeChainHeadEvent allows subscribers to receive notifications when new blocks are finalized.
// Returns a subscription that will receive ChainHeadEvent notifications via the provided channel.
func (b *Blockchain) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	b.logger.Debug("new chain head event subscription created")
	return b.chainHeadFeed.Subscribe(ch)
}

// NotifyNewBlock sends a chain head event when a new block is finalized
func (b *Blockchain) NotifyNewBlock() {
	latestCtx, err := b.newLatestContext()
	if err != nil {
		b.setLatestContext(sdk.Context{})
		b.logger.Debug("failed to get latest context, notifying chain head", "error", err)
	}
	b.setLatestContext(latestCtx)
	header := b.CurrentBlock()
	headerHash := header.Hash()

	b.logger.Debug("notifying new block",
		"block_number", header.Number.String(),
		"block_hash", headerHash.Hex(),
		"previous_hash", b.getPreviousHeaderHash().Hex())

	b.setPreviousHeaderHash(headerHash)
	b.chainHeadFeed.Send(core.ChainHeadEvent{Header: header})

	b.logger.Debug("chain head event sent to feed")
}

// StateAt returns the StateDB object for a given block hash.
// In practice, this always returns the most recent state since the mempool
// only needs current state for validation. Historical state access is not supported
// as it's never required by the txpool.
func (b *Blockchain) StateAt(hash common.Hash) (vm.StateDB, error) {
	b.logger.Debug("StateAt called", "requested_hash", hash.Hex())

	// This is returned at block 0, before the context is available.
	if hash == common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000") || hash == types.EmptyCodeHash {
		b.logger.Debug("returning nil StateDB for zero hash or empty code hash")
		return vm.StateDB(nil), nil
	}

	// Always get the latest context to avoid stale nonce state.
	ctx, err := b.GetLatestContext()
	if err != nil {
		// If we can't get the latest context for blocks past 1, something is seriously wrong with the chain state
		return nil, fmt.Errorf("failed to get latest context for StateAt: %w", err)
	}

	// Create a cache context to isolate the StateDB from concurrent commits.
	// This prevents race conditions when the background txpool reorg goroutine
	// reads state while the main thread is committing new blocks.
	cacheCtx, _ := ctx.CacheContext()
	// Use an infinite gas meter to avoid tracking gas for read-only state queries
	cacheCtx = cacheCtx.WithGasMeter(sdktypes.NewInfiniteGasMeter())

	appHash := cacheCtx.BlockHeader().AppHash
	stateDB := statedb.New(cacheCtx, b.vmKeeper, statedb.NewEmptyTxConfig())

	b.logger.Debug("StateDB created successfully", "app_hash", common.Hash(appHash).Hex())
	return stateDB, nil
}

// BeginCommit acquires an exclusive lock to prevent mempool state reads during Commit.
// This avoids data races in the underlying storage (e.g., IAVL) when tests run with -race.
func (b *Blockchain) BeginCommit() { b.testingCommitMu.Lock() }

// EndCommit releases the exclusive lock acquired by BeginCommit.
func (b *Blockchain) EndCommit() { b.testingCommitMu.Unlock() }

// BeginRead acquires a shared lock for background readers (e.g., txpool reorg).
// This enables optional coordination with Commit without importing the type.
func (b *Blockchain) BeginRead() { b.testingCommitMu.RLock() }

// EndRead releases the shared read lock acquired by BeginRead.
func (b *Blockchain) EndRead() { b.testingCommitMu.RUnlock() }

func (b *Blockchain) getPreviousHeaderHash() common.Hash {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.previousHeaderHash
}

func (b *Blockchain) setPreviousHeaderHash(h common.Hash) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.previousHeaderHash = h
}

func (b *Blockchain) setLatestContext(ctx sdk.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.latestCtx = ctx
}

// GetLatestContext returns the latest context as updated by the block,
// or attempts to retrieve it again if unavailable.
func (b *Blockchain) GetLatestContext() (sdk.Context, error) {
	b.logger.Debug("getting latest context")
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.latestCtx.Context() != nil {
		return b.latestCtx, nil
	}

	return b.newLatestContext()
}

// newLatestContext retrieves the most recent query context from the application.
// This provides access to the current blockchain state for transaction validation and execution.
func (b *Blockchain) newLatestContext() (sdk.Context, error) {
	b.logger.Debug("getting latest context")

	ctx, err := b.getCtxCallback(0, false)
	if err != nil {
		return sdk.Context{}, sdkerrors.Wrapf(err, "failed to get latest context")
	}

	ctx = ctx.WithBlockGasMeter(sdktypes.NewGasMeter(b.blockGasLimit))

	b.logger.Debug("latest context retrieved successfully",
		"block_height", ctx.BlockHeight(),
		"gas_limit", b.blockGasLimit)

	return ctx, nil
}
