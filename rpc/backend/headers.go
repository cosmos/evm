package backend

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	rpctypes "github.com/cosmos/evm/rpc/types"
)

// GetBlockByNumber returns the JSON-RPC compatible Ethereum block identified by
// block number. Depending on fullTx it either returns the full transaction
// objects or if false only the hashes of the transactions.
func (b *Backend) GetHeaderByNumber(ctx context.Context, blockNum rpctypes.BlockNumber) (result map[string]interface{}, err error) {
	ctx, span := tracer.Start(ctx, "GetHeaderByNumber", trace.WithAttributes(attribute.Int64("blockNum", blockNum.Int64())))
	defer span.End()
	defer func() { span.RecordError(err) }()

	resBlock, err := b.CometBlockByNumber(ctx, blockNum)
	if err != nil {
		return nil, nil
	}

	// return if requested block height is greater than the current one
	if resBlock == nil || resBlock.Block == nil {
		return nil, nil
	}

	blockRes, err := b.RPCClient.BlockResults(ctx, &resBlock.Block.Height)
	if err != nil {
		b.Logger.Debug("failed to fetch block result from CometBFT", "height", blockNum, "error", err.Error())
		return nil, nil
	}

	res, err := b.RPCHeaderFromCometBlock(ctx, resBlock, blockRes)
	if err != nil {
		b.Logger.Debug("RPCBlockFromCometBlock failed", "height", blockNum, "error", err.Error())
		return nil, err
	}

	return res, nil
}

// GetBlockByHash returns the JSON-RPC compatible Ethereum block identified by
// hash.
func (b *Backend) GetHeaderByHash(ctx context.Context, hash common.Hash) (result map[string]interface{}, err error) {
	ctx, span := tracer.Start(ctx, "GetHeaderByHash", trace.WithAttributes(attribute.String("hash", hash.Hex())))
	defer span.End()
	defer func() { span.RecordError(err) }()

	resBlock, err := b.CometBlockByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	if resBlock == nil {
		// block not found
		return nil, nil
	}

	blockRes, err := b.RPCClient.BlockResults(ctx, &resBlock.Block.Height)
	if err != nil {
		b.Logger.Debug("failed to fetch block result from CometBFT", "block-hash", hash.String(), "error", err.Error())
		return nil, nil
	}

	res, err := b.RPCHeaderFromCometBlock(ctx, resBlock, blockRes)
	if err != nil {
		b.Logger.Debug("RPCBlockFromCometBlock failed", "hash", hash, "error", err.Error())
		return nil, err
	}

	return res, nil
}

// HeaderByNumber returns the block header identified by height.
func (b *Backend) HeaderByNumber(ctx context.Context, blockNum rpctypes.BlockNumber) (result *ethtypes.Header, err error) {
	ctx, span := tracer.Start(ctx, "HeaderByNumber", trace.WithAttributes(attribute.Int64("blockNum", blockNum.Int64())))
	defer span.End()
	defer func() { span.RecordError(err) }()

	resBlock, err := b.CometBlockByNumber(ctx, blockNum)
	if err != nil {
		return nil, err
	}

	if resBlock == nil {
		// block not found
		return nil, nil
	}

	blockRes, err := b.CometBlockResultByNumber(ctx, &resBlock.Block.Height)
	if err != nil {
		return nil, fmt.Errorf("header result not found for height %d", resBlock.Block.Height)
	}

	ethBlock, err := b.EthBlockFromCometBlock(ctx, resBlock, blockRes)
	if err != nil {
		return nil, fmt.Errorf("failed to get rpc block from comet block: %w", err)
	}

	return ethBlock.Header(), nil
}

// HeaderByHash returns the block header identified by hash.
func (b *Backend) HeaderByHash(ctx context.Context, blockHash common.Hash) (result *ethtypes.Header, err error) {
	ctx, span := tracer.Start(ctx, "HeaderByHash", trace.WithAttributes(attribute.String("blockHash", blockHash.Hex())))
	defer span.End()
	defer func() { span.RecordError(err) }()

	resBlock, err := b.CometBlockByHash(ctx, blockHash)
	if err != nil {
		return nil, err
	}

	if resBlock == nil {
		// block not found
		return nil, nil
	}

	blockRes, err := b.RPCClient.BlockResults(ctx, &resBlock.Block.Height)
	if err != nil {
		b.Logger.Debug("failed to fetch block result from CometBFT", "block-hash", blockHash.String(), "error", err.Error())
		return nil, nil
	}

	ethBlock, err := b.EthBlockFromCometBlock(ctx, resBlock, blockRes)
	if err != nil {
		return nil, fmt.Errorf("failed to get rpc block from comet block: %w", err)
	}

	return ethBlock.Header(), nil
}
