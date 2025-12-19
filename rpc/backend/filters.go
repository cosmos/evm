package backend

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	evmtrace "github.com/cosmos/evm/trace"
)

// GetLogs returns all the logs from all the ethereum transactions in a block.
func (b *Backend) GetLogs(ctx context.Context, hash common.Hash) (result [][]*ethtypes.Log, err error) {
	ctx, span := tracer.Start(ctx, "GetLogs", trace.WithAttributes(attribute.String("hash", hash.Hex())))
	defer func() { evmtrace.EndSpanErr(span, err) }()

	resBlock, err := b.CometBlockByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if resBlock == nil {
		return nil, errors.Errorf("block not found for hash %s", hash)
	}
	return b.GetLogsByHeight(ctx, &resBlock.Block.Height)
}

// GetLogsByHeight returns all the logs from all the ethereum transactions in a block.
func (b *Backend) GetLogsByHeight(ctx context.Context, height *int64) (result [][]*ethtypes.Log, err error) {
	var heightAttr int64
	if height != nil {
		heightAttr = *height
	}
	ctx, span := tracer.Start(ctx, "GetLogsByHeight", trace.WithAttributes(attribute.Int64("height", heightAttr)))
	defer func() { evmtrace.EndSpanErr(span, err) }()

	// NOTE: we query the state in case the tx result logs are not persisted after an upgrade.
	blockRes, err := b.RPCClient.BlockResults(ctx, height)
	if err != nil {
		return nil, err
	}

	return GetLogsFromBlockResults(blockRes)
}

// BloomStatus returns the BloomBitsBlocks and the number of processed sections maintained
// by the chain indexer.
func (b *Backend) BloomStatus() (uint64, uint64) {
	return 4096, 0
}
