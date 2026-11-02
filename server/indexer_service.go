package server

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/cometbft/cometbft/libs/service"
	rpcclient "github.com/cometbft/cometbft/rpc/client"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/types"

	servertypes "github.com/cosmos/evm/server/types"
)

const (
	ServiceName = "EVMIndexerService"

	NewBlockWaitTimeout = 60 * time.Second
)

// EVMIndexerService indexes transactions for json-rpc service.
type EVMIndexerService struct {
	service.BaseService

	txIdxr servertypes.EVMTxIndexer
	client rpcclient.Client
}

// NewEVMIndexerService returns a new service instance.
func NewEVMIndexerService(
	txIdxr servertypes.EVMTxIndexer,
	client rpcclient.Client,
) *EVMIndexerService {
	is := &EVMIndexerService{txIdxr: txIdxr, client: client}
	is.BaseService = *service.NewBaseService(nil, ServiceName, is)
	return is
}

// OnStart implements service.Service by subscribing for new blocks
// and indexing them by events.
func (eis *EVMIndexerService) OnStart() error {
	ctx := context.Background()
	status, err := eis.client.Status(ctx)
	if err != nil {
		return err
	}
	// latestBlock is written by the subscription goroutine below and read by
	// the indexing loop.
	var latestBlock atomic.Int64
	latestBlock.Store(status.SyncInfo.LatestBlockHeight)
	newBlockSignal := make(chan struct{}, 1)

	// Use SubscribeUnbuffered here to ensure both subscriptions does not get
	// canceled due to not pulling messages fast enough. Cause this might
	// sometimes happen when there are no other subscribers.
	blockHeadersChan, err := eis.client.Subscribe(
		ctx,
		ServiceName,
		types.QueryForEvent(types.EventNewBlockHeader).String(),
		0)
	if err != nil {
		return err
	}

	go func() {
		for {
			msg := <-blockHeadersChan
			eventDataHeader := msg.Data.(types.EventDataNewBlockHeader)
			if eventDataHeader.Header.Height > latestBlock.Load() {
				latestBlock.Store(eventDataHeader.Header.Height)
				// notify
				select {
				case newBlockSignal <- struct{}{}:
				default:
				}
			}
		}
	}()

	lastBlock, err := eis.txIdxr.LastIndexedBlock()
	if err != nil {
		return err
	}
	if lastBlock == -1 {
		lastBlock = latestBlock.Load()
	}

	eis.indexLoop(ctx, eis.client, lastBlock, latestBlock.Load, func(ctx context.Context) {
		select {
		case <-newBlockSignal:
		case <-time.After(NewBlockWaitTimeout):
		case <-ctx.Done():
		}
	})
	return nil
}

// indexLoop indexes blocks after lastBlock as they are produced, until ctx is
// done. latestBlock reports the newest height; wait returns once a new block
// may be available or a retry is due.
func (eis *EVMIndexerService) indexLoop(
	ctx context.Context,
	src blockSource,
	lastBlock int64,
	latestBlock func() int64,
	wait func(context.Context),
) {
	// blockErr is the error that stopped the previous pass (fetching a
	// block, its results, or indexing it); the failed height is retried.
	var blockErr error

	for ctx.Err() == nil {
		latest := latestBlock()
		if latest <= lastBlock || blockErr != nil {
			// two cases to enter this block:
			// 1. nothing to index (indexer is caught up). wait for signal of new block.
			// 2. previous attempt errored (failed to fetch the Block or BlockResults, or
			//    to index them). in this case, wait before retrying the same height,
			//    rather than infinite looping a failing attempt. this can occur due to
			//    drive latency between the block existing and its block_results getting
			//    saved, or a transient storage error writing the index.
			wait(ctx)
			blockErr = nil
			continue
		}
		lastBlock, blockErr = eis.indexBlocks(ctx, src, lastBlock, latest)
	}
}

// blockSource is the part of the CometBFT RPC client the indexer reads.
type blockSource interface {
	Block(ctx context.Context, height *int64) (*coretypes.ResultBlock, error)
	BlockResults(ctx context.Context, height *int64) (*coretypes.ResultBlockResults, error)
}

// indexBlocks indexes heights lastBlock+1 through latestBlock in order and
// returns the last height it indexed. It stops at the first height it cannot
// fetch or index and returns that error, so the caller retries the height
// instead of skipping it: a skipped height would never be indexed, leaving its
// transactions without receipts or lookups.
func (eis *EVMIndexerService) indexBlocks(ctx context.Context, src blockSource, lastBlock, latestBlock int64) (int64, error) {
	for i := lastBlock + 1; i <= latestBlock; i++ {
		block, err := src.Block(ctx, &i)
		if err != nil {
			eis.Logger.Error("failed to fetch block", "height", i, "err", err)
			return lastBlock, err
		}
		blockResult, err := src.BlockResults(ctx, &i)
		if err != nil {
			eis.Logger.Error("failed to fetch block result", "height", i, "err", err)
			return lastBlock, err
		}
		if err := eis.txIdxr.IndexBlock(block.Block, blockResult.TxsResults); err != nil {
			eis.Logger.Error("failed to index block, will retry", "height", i, "err", err)
			return lastBlock, err
		}
		lastBlock = blockResult.Height
	}
	return lastBlock, nil
}
