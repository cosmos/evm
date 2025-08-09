package mempool

import (
	"fmt"
	"math/big"

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

	errors2 "cosmossdk.io/errors"
	types2 "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	_ txpool.BlockChain     = Blockchain{}
	_ legacypool.BlockChain = Blockchain{}
)

// Blockchain implements the BlockChain interface required by Ethereum transaction pools.
// It bridges Cosmos SDK blockchain state with Ethereum's transaction pool system by providing
// access to block headers, chain configuration, and state databases. This implementation is
// specifically designed for instant finality chains where reorgs never occur.
type Blockchain struct {
	ctx                func(height int64, prove bool) (sdk.Context, error)
	vmKeeper           VMKeeperI
	feeMarketKeeper    FeeMarketKeeperI
	chainHeadFeed      *event.Feed
	zeroHeader         *types.Header
	blockGasLimit      uint64
	previousHeaderHash common.Hash
}

// NewBlockchain creates a new Blockchain instance that bridges Cosmos SDK state with Ethereum mempools.
// The ctx function provides access to Cosmos SDK contexts at different heights, vmKeeper manages EVM state,
// and feeMarketKeeper handles fee market operations like base fee calculations.
func NewBlockchain(ctx func(height int64, prove bool) (sdk.Context, error), vmKeeper VMKeeperI, feeMarketKeeper FeeMarketKeeperI, blockGasLimit uint64) *Blockchain {
	return &Blockchain{
		ctx:             ctx,
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
func (b Blockchain) Config() *params.ChainConfig {
	return evmtypes.GetEthChainConfig()
}

// CurrentBlock returns the current block header for the app.
// It constructs an Ethereum-compatible header from the current Cosmos SDK context,
// including block height, timestamp, gas limits, and base fee (if London fork is active).
// Returns a zero header as placeholder if the context is not yet available.
func (b Blockchain) CurrentBlock() *types.Header {
	ctx, err := b.GetLatestCtx()
	// This should only error out on the first block.
	if err != nil {
		return b.zeroHeader
	}

	header := &types.Header{
		Number:     big.NewInt(ctx.BlockHeight()),
		Time:       uint64(ctx.BlockTime().Unix()), // #nosec G115 -- overflow not a concern with unix time
		GasLimit:   b.blockGasLimit,
		GasUsed:    b.feeMarketKeeper.GetBlockGasWanted(ctx),
		ParentHash: b.previousHeaderHash,
		Root:       common.BytesToHash(ctx.BlockHeader().AppHash), // we actually don't care that this isn't the ctx header, as long as we properly track roots and parent roots to prevent the reorg from triggering
		Difficulty: big.NewInt(0),                                 // 0 difficulty on PoS
	}

	chainConfig := evmtypes.GetEthChainConfig()
	if chainConfig.IsLondon(header.Number) {
		baseFee := b.vmKeeper.GetBaseFee(ctx)
		if baseFee != nil {
			header.BaseFee = baseFee
		}
	}

	return header
}

// GetBlock retrieves a block by hash and number.
// Cosmos chains have instant finality, so  this method should only be called for the genesis block (block 0)
// or block 1, as reorgs never occur. Any other call indicates a bug in the mempool logic.
// Panics if called for blocks beyond block 1, as this would indicate an attempted reorg.
func (b Blockchain) GetBlock(_ common.Hash, _ uint64) *types.Block {
	currBlock := b.CurrentBlock()
	if currBlock.Number.Cmp(big.NewInt(0)) == 0 {
		currBlock.ParentHash = common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
		return types.NewBlockWithHeader(currBlock)
	} else if currBlock.Number.Cmp(big.NewInt(1)) == 0 {
		return types.NewBlockWithHeader(currBlock)
	}

	panic("GetBlock should never be called on a Cosmos chain due to instant finality - this indicates a reorg is being attempted")
}

// SubscribeChainHeadEvent allows subscribers to receive notifications when new blocks are finalized.
// Returns a subscription that will receive ChainHeadEvent notifications via the provided channel.
func (b Blockchain) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.chainHeadFeed.Subscribe(ch)
}

// NotifyNewBlock sends a chain head event when a new block is finalized
func (b *Blockchain) NotifyNewBlock() {
	header := b.CurrentBlock()
	b.previousHeaderHash = header.Hash()
	b.chainHeadFeed.Send(core.ChainHeadEvent{Header: header})
}

// StateAt returns the StateDB object for a given block hash.
// In practice, this always returns the most recent state since the mempool
// only needs current state for validation. Historical state access is not supported
// as it's never required by the txpool.
func (b Blockchain) StateAt(hash common.Hash) (vm.StateDB, error) {
	// This is returned at block 0, before the context is available.
	if hash == common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000") || hash == types.EmptyCodeHash {
		return vm.StateDB(nil), nil
	}

	// Always get the latest context to avoid stale nonce state.
	ctx, err := b.GetLatestCtx()
	if err != nil {
		// If we can't get the latest context for blocks past 1, something is seriously wrong with the chain state
		return nil, fmt.Errorf("failed to get latest context for StateAt: %w", err)
	}

	return statedb.New(ctx, b.vmKeeper, statedb.NewEmptyTxConfig(common.Hash(ctx.BlockHeader().AppHash))), nil
}

// GetLatestCtx retrieves the most recent query context from the application.
// This provides access to the current blockchain state for transaction validation and execution.
func (b Blockchain) GetLatestCtx() (sdk.Context, error) {
	ctx, err := b.ctx(0, false)
	if err != nil {
		return sdk.Context{}, errors2.Wrapf(err, "getting latest context")
	}
	ctx = ctx.WithBlockGasMeter(types2.NewGasMeter(b.blockGasLimit))
	return ctx, nil
}
