package backend

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	cmtrpctypes "github.com/cometbft/cometbft/rpc/core/types"

	rpctypes "github.com/cosmos/evm/rpc/types"
	"github.com/cosmos/evm/utils"
)

// CometBlockByNumber returns a CometBFT-formatted block for a given
// block number
func (b *Backend) CometBlockByNumber(ctx context.Context, blockNum rpctypes.BlockNumber) (result *cmtrpctypes.ResultBlock, err error) {
	ctx, span := tracer.Start(ctx, "CometBlockByNumber", trace.WithAttributes(attribute.Int64("blockNum", blockNum.Int64())))
	defer span.End()
	defer func() { span.RecordError(err) }()

	height, err := b.getHeightByBlockNum(ctx, blockNum)
	if err != nil {
		return nil, err
	}
	resBlock, err := b.RPCClient.Block(ctx, &height)
	if err != nil {
		b.Logger.Debug("cometbft client failed to get block", "height", height, "error", err.Error())
		return nil, err
	}

	if resBlock.Block == nil {
		b.Logger.Debug("CometBlockByNumber block not found", "height", height)
		return nil, nil
	}

	return resBlock, nil
}

// CometHeaderByNumber returns a CometBFT-formatted header for a given
// block number
func (b *Backend) CometHeaderByNumber(ctx context.Context, blockNum rpctypes.BlockNumber) (result *cmtrpctypes.ResultHeader, err error) {
	ctx, span := tracer.Start(ctx, "CometHeaderByNumber", trace.WithAttributes(attribute.Int64("blockNum", blockNum.Int64())))
	defer span.End()
	defer func() { span.RecordError(err) }()

	height, err := b.getHeightByBlockNum(ctx, blockNum)
	if err != nil {
		return nil, err
	}
	return b.RPCClient.Header(ctx, &height)
}

// CometBlockResultByNumber returns a CometBFT-formatted block result
// by block number
func (b *Backend) CometBlockResultByNumber(ctx context.Context, height *int64) (result *cmtrpctypes.ResultBlockResults, err error) {
	var heightAttr int64
	if height != nil {
		heightAttr = *height
	}
	ctx, span := tracer.Start(ctx, "CometBlockResultByNumber", trace.WithAttributes(attribute.Int64("height", heightAttr)))
	defer span.End()
	defer func() { span.RecordError(err) }()

	if height != nil && *height == 0 {
		height = nil
	}
	res, err := b.RPCClient.BlockResults(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block result from CometBFT %d: %w", *height, err)
	}

	return res, nil
}

// CometBlockByHash returns a CometBFT-formatted block by block number
func (b *Backend) CometBlockByHash(ctx context.Context, blockHash common.Hash) (result *cmtrpctypes.ResultBlock, err error) {
	ctx, span := tracer.Start(ctx, "CometBlockByHash", trace.WithAttributes(attribute.String("blockHash", blockHash.Hex())))
	defer span.End()
	defer func() { span.RecordError(err) }()

	resBlock, err := b.RPCClient.BlockByHash(ctx, blockHash.Bytes())
	if err != nil {
		b.Logger.Debug("CometBFT client failed to get block", "blockHash", blockHash.Hex(), "error", err.Error())
		return nil, err
	}

	if resBlock == nil || resBlock.Block == nil {
		b.Logger.Debug("CometBlockByHash block not found", "blockHash", blockHash.Hex())
		return nil, fmt.Errorf("block not found for hash %s", blockHash.Hex())
	}

	return resBlock, nil
}

func (b *Backend) getHeightByBlockNum(ctx context.Context, blockNum rpctypes.BlockNumber) (height int64, err error) {
	ctx, span := tracer.Start(ctx, "getHeightByBlockNum", trace.WithAttributes(attribute.Int64("blockNum", blockNum.Int64())))
	defer span.End()
	defer func() { span.RecordError(err) }()

	if blockNum == rpctypes.EthEarliestBlockNumber {
		status, err := b.ClientCtx.Client.Status(ctx)
		if err != nil {
			return 0, errors.New("failed to get earliest block height")
		}

		return status.SyncInfo.EarliestBlockHeight, nil
	}

	height = blockNum.Int64()
	if height <= 0 {
		// In cometBFT, LatestBlockNumber, FinalizedBlockNumber, SafeBlockNumber all map to the latest block height.
		// Fetch the latest block number from the app state, more accurate than the CometBFT block store state.
		//
		// For PendingBlockNumber, we alsoe returns the latest block height.
		// The reason is that CometBFT does not have the concept of pending block,
		// and the application state is only updated when a block is committed.
		n, err := b.BlockNumber(ctx)
		if err != nil {
			return 0, err
		}
		height, err = utils.SafeHexToInt64(n)
		if err != nil {
			return 0, err
		}
	}

	return height, nil
}
