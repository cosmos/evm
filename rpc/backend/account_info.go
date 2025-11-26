package backend

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cometbft/cometbft/libs/bytes"

	rpctypes "github.com/cosmos/evm/rpc/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// GetCode returns the contract code at the given address and block number.
func (b *Backend) GetCode(ctx context.Context, address common.Address, blockNrOrHash rpctypes.BlockNumberOrHash) (bz hexutil.Bytes, err error) {
	ctx, span := tracer.Start(ctx, "GetCode", trace.WithAttributes(attribute.String("address", address.String()), attribute.String("blockNorHash", unwrapBlockNOrHash(blockNrOrHash))))
	defer span.End()
	defer func() { span.RecordError(err) }()

	blockNum, err := b.BlockNumberFromComet(ctx, blockNrOrHash)
	if err != nil {
		return nil, err
	}

	ctx = rpctypes.ContextWithHeight(blockNum.Int64(), ctx)
	req := &evmtypes.QueryCodeRequest{
		Address: address.String(),
	}

	res, err := b.QueryClient.Code(ctx, req)
	if err != nil {
		return nil, err
	}

	return res.Code, nil
}

// GetProof returns an account object with proof and any storage proofs
func (b *Backend) GetProof(ctx context.Context, address common.Address, storageKeys []string, blockNrOrHash rpctypes.BlockNumberOrHash) (result *rpctypes.AccountResult, err error) {
	ctx, span := tracer.Start(ctx, "GetProof", trace.WithAttributes(attribute.String("address", address.String()), attribute.StringSlice("storageKeys", storageKeys), attribute.String("blockNorHash", unwrapBlockNOrHash(blockNrOrHash))))
	defer span.End()
	defer func() { span.RecordError(err) }()

	blockNum, err := b.BlockNumberFromComet(ctx, blockNrOrHash)
	if err != nil {
		return nil, err
	}

	height := int64(blockNum)

	_, err = b.CometHeaderByNumber(ctx, blockNum)
	if err != nil {
		// the error message imitates geth behavior
		return nil, errors.New("header not found")
	}

	// if the height is equal to zero, meaning the query condition of the block is either "pending" or "latest"
	if height == 0 {
		bn, err := b.BlockNumber(ctx)
		if err != nil {
			return nil, err
		}

		if bn > math.MaxInt64 {
			return nil, fmt.Errorf("not able to query block number greater than MaxInt64")
		}

		height = int64(bn) //#nosec G115 -- checked for int overflow already
	}

	ctx = rpctypes.ContextWithHeight(height, ctx)
	clientCtx := b.ClientCtx.WithHeight(height).WithCmdContext(ctx)

	// query storage proofs
	storageProofs := make([]rpctypes.StorageResult, len(storageKeys))

	for i, key := range storageKeys {
		hexKey := common.HexToHash(key)
		valueBz, proof, err := b.QueryClient.GetProof(clientCtx, evmtypes.StoreKey, evmtypes.StateKey(address, hexKey.Bytes()))
		if err != nil {
			return nil, err
		}

		storageProofs[i] = rpctypes.StorageResult{
			Key:   key,
			Value: (*hexutil.Big)(new(big.Int).SetBytes(valueBz)),
			Proof: GetHexProofs(proof),
		}
	}

	// query EVM account
	req := &evmtypes.QueryAccountRequest{
		Address: address.String(),
	}

	res, err := b.QueryClient.Account(ctx, req)
	if err != nil {
		return nil, err
	}

	// query account proofs
	accountKey := bytes.HexBytes(append(authtypes.AddressStoreKeyPrefix, address.Bytes()...))
	_, proof, err := b.QueryClient.GetProof(clientCtx, authtypes.StoreKey, accountKey)
	if err != nil {
		return nil, err
	}

	balance, ok := sdkmath.NewIntFromString(res.Balance)
	if !ok {
		return nil, errors.New("invalid balance")
	}

	return &rpctypes.AccountResult{
		Address:      address,
		AccountProof: GetHexProofs(proof),
		Balance:      (*hexutil.Big)(balance.BigInt()),
		CodeHash:     common.HexToHash(res.CodeHash),
		Nonce:        hexutil.Uint64(res.Nonce),
		StorageHash:  common.Hash{}, // NOTE: Cosmos EVM doesn't have a storage hash. TODO: implement?
		StorageProof: storageProofs,
	}, nil
}

// GetStorageAt returns the contract storage at the given address, block number, and key.
func (b *Backend) GetStorageAt(ctx context.Context, address common.Address, key string, blockNrOrHash rpctypes.BlockNumberOrHash) (result hexutil.Bytes, err error) {
	ctx, span := tracer.Start(ctx, "GetStorageAt", trace.WithAttributes(attribute.String("address", address.String()), attribute.String("key", key), attribute.String("blockNorHash", unwrapBlockNOrHash(blockNrOrHash))))
	defer span.End()
	defer func() { span.RecordError(err) }()

	blockNum, err := b.BlockNumberFromComet(ctx, blockNrOrHash)
	if err != nil {
		return nil, err
	}

	ctx = rpctypes.ContextWithHeight(blockNum.Int64(), ctx)
	req := &evmtypes.QueryStorageRequest{
		Address: address.String(),
		Key:     key,
	}

	res, err := b.QueryClient.Storage(ctx, req)
	if err != nil {
		return nil, err
	}

	value := common.HexToHash(res.Value)
	return value.Bytes(), nil
}

// GetBalance returns the provided account's *spendable* balance up to the provided block number.
func (b *Backend) GetBalance(ctx context.Context, address common.Address, blockNrOrHash rpctypes.BlockNumberOrHash) (result *hexutil.Big, err error) {
	ctx, span := tracer.Start(ctx, "GetBalance", trace.WithAttributes(attribute.String("address", address.String()), attribute.String("blockNorHash", unwrapBlockNOrHash(blockNrOrHash))))
	defer span.End()
	defer func() { span.RecordError(err) }()

	blockNum, err := b.BlockNumberFromComet(ctx, blockNrOrHash)
	if err != nil {
		return nil, err
	}
	ctx = rpctypes.ContextWithHeight(blockNum.Int64(), ctx)

	req := &evmtypes.QueryBalanceRequest{
		Address: address.String(),
	}

	_, err = b.CometHeaderByNumber(ctx, blockNum)
	if err != nil {
		return nil, err
	}

	res, err := b.QueryClient.Balance(ctx, req)
	if err != nil {
		return nil, err
	}

	val, ok := sdkmath.NewIntFromString(res.Balance)
	if !ok {
		return nil, errors.New("invalid balance")
	}

	// balance can only be negative in case of pruned node
	if val.IsNegative() {
		return nil, errors.New("couldn't fetch balance. Node state is pruned")
	}

	return (*hexutil.Big)(val.BigInt()), nil
}

// GetTransactionCount returns the number of transactions at the given address up to the given block number.
func (b *Backend) GetTransactionCount(ctx context.Context, address common.Address, blockNum rpctypes.BlockNumber) (result *hexutil.Uint64, err error) {
	ctx, span := tracer.Start(ctx, "GetTransactionCount", trace.WithAttributes(attribute.String("address", address.String()), attribute.Int64("blockNum", blockNum.Int64())))
	defer span.End()
	defer func() { span.RecordError(err) }()

	n := hexutil.Uint64(0)
	bn, err := b.BlockNumber(ctx)
	if err != nil {
		return &n, err
	}
	height := blockNum.Int64()

	currentHeight := int64(bn) //#nosec G115 -- checked for int overflow already
	if height > currentHeight {
		return &n, errorsmod.Wrapf(
			sdkerrors.ErrInvalidHeight,
			"cannot query with height in the future (current: %d, queried: %d); please provide a valid height",
			currentHeight, height,
		)
	}
	// Get nonce (sequence) from account
	from := sdk.AccAddress(address.Bytes())
	accRet := b.ClientCtx.AccountRetriever

	err = accRet.EnsureExists(b.ClientCtx.WithCmdContext(ctx), from)
	if err != nil {
		// account doesn't exist yet, return 0
		return &n, nil
	}

	includePending := blockNum == rpctypes.EthPendingBlockNumber
	nonce, err := b.getAccountNonce(ctx, address, includePending, blockNum.Int64(), b.Logger)
	if err != nil {
		return nil, err
	}

	n = hexutil.Uint64(nonce)
	return &n, nil
}

func unwrapBlockNOrHash(blockNOrHash rpctypes.BlockNumberOrHash) string {
	if blockNOrHash.BlockHash != nil {
		return blockNOrHash.BlockHash.String()
	}
	if blockNOrHash.BlockNumber != nil {
		return fmt.Sprintf("%d", *blockNOrHash.BlockNumber)
	}
	return ""
}
