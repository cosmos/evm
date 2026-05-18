package backend

import (
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"

	cmtrpcclient "github.com/cometbft/cometbft/rpc/client"
	cmtrpctypes "github.com/cometbft/cometbft/rpc/core/types"

	rpctypes "github.com/cosmos/evm/rpc/types"
	"github.com/cosmos/evm/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetTransactionByHash returns the Ethereum format transaction identified by Ethereum transaction hash
func (b *Backend) GetTransactionByHash(txHash common.Hash) (*rpctypes.RPCTransaction, error) {
	res, additional, err := b.GetTxByEthHash(txHash)
	if err != nil {
		return b.GetTransactionByHashPending(txHash)
	}

	block, err := b.CometBlockByNumber(rpctypes.BlockNumber(res.Height))
	if err != nil {
		return nil, err
	}

	blockRes, err := b.RPCClient.BlockResults(b.Ctx, &block.Block.Height)
	if err != nil {
		b.Logger.Debug("block result not found", "height", block.Block.Height, "error", err.Error())
		return nil, fmt.Errorf("block result not found: %w", err)
	}

	var ethMsg *evmtypes.MsgEthereumTx
	if additional == nil {
		// #nosec G115 always in range
		tx, err := b.ClientCtx.TxConfig.TxDecoder()(block.Block.Txs[res.TxIndex])
		if err != nil {
			b.Logger.Debug("decoding failed", "error", err.Error())
			return nil, fmt.Errorf("failed to decode tx: %w", err)
		}
		ethMsg = tx.GetMsgs()[res.MsgIndex].(*evmtypes.MsgEthereumTx)
		if ethMsg == nil {
			b.Logger.Error("failed to get eth msg from sdk.Msgs")
			return nil, fmt.Errorf("failed to get eth msg from sdk.Msgs")
		}
	} else {
		ethMsg = b.parseDerivedTxFromAdditionalFields(additional)
		if ethMsg == nil {
			b.Logger.Error("failed to get derived eth msg from additional fields")
			return nil, fmt.Errorf("failed to get derived eth msg from additional fields")
		}
	}

	if res.EthTxIndex == -1 {
		// Fallback to find tx index by iterating all valid eth transactions
		msgs, _ := b.EthMsgsFromCometBlock(block, blockRes)
		for i := range msgs {
			if msgs[i].Hash() == txHash {
				if i > math.MaxInt32 {
					return nil, errors.New("tx index overflow")
				}
				res.EthTxIndex = int32(i)
				break
			}
		}
	}

	// if we still unable to find the eth tx index, return error, shouldn't happen.
	if res.EthTxIndex == -1 && additional == nil {
		return nil, errors.New("can't find index of ethereum tx")
	}

	baseFee, err := b.BaseFee(blockRes)
	if err != nil {
		// handle the error for pruned node.
		b.Logger.Error("failed to fetch Base Fee from prunned block. Check node prunning configuration", "height", blockRes.Height, "error", err)
	}

	height := uint64(res.Height)    //#nosec G115 -- checked for int overflow already
	index := uint64(res.EthTxIndex) //#nosec G115 -- checked for int overflow already
	blockHash := common.BytesToHash(block.BlockID.Hash.Bytes())
	if additional == nil {
		return rpctypes.NewTransactionFromMsg(ethMsg, blockHash, height, index, baseFee, b.EvmChainID)
	}
	return rpctypes.NewRPCTransactionFromIncompleteMsg(ethMsg, blockHash, height, index, baseFee, b.EvmChainID, additional.Hash)
}

// GetTransactionByHashPending find pending tx from mempool
func (b *Backend) GetTransactionByHashPending(txHash common.Hash) (*rpctypes.RPCTransaction, error) {
	hexTx := txHash.Hex()
	// try to find tx in mempool
	txs, err := b.PendingTransactions()
	if err != nil {
		b.Logger.Debug("tx not found", "hash", hexTx, "error", err.Error())
		return nil, nil
	}

	for _, tx := range txs {
		msg, err := evmtypes.UnwrapEthereumMsg(tx, txHash)
		if err != nil {
			// not ethereum tx
			continue
		}

		if msg.Hash() == txHash {
			// use zero block values since it's not included in a block yet
			rpctx, err := rpctypes.NewTransactionFromMsg(
				msg,
				common.Hash{},
				uint64(0),
				uint64(0),
				nil,
				b.EvmChainID,
			)
			if err != nil {
				return nil, err
			}
			return rpctx, nil
		}
	}

	b.Logger.Debug("tx not found", "hash", hexTx)
	return nil, nil
}

// GetGasUsed returns gasUsed from transaction
func (b *Backend) GetGasUsed(res *types.TxResult, price *big.Int, gas uint64) uint64 {
	// patch gasUsed if tx is reverted and happened before height on which fixed was introduced
	// to return real gas charged
	// more info at https://github.com/evmos/ethermint/pull/1557
	if res.Failed && price != nil && res.Height < b.Cfg.JSONRPC.FixRevertGasRefundHeight {
		return new(big.Int).Mul(price, new(big.Int).SetUint64(gas)).Uint64()
	}
	return res.GasUsed
}

// GetTransactionReceipt returns the transaction receipt identified by hash.
func (b *Backend) GetTransactionReceipt(hash common.Hash) (map[string]interface{}, error) {
	hexTx := hash.Hex()
	b.Logger.Debug("eth_getTransactionReceipt", "hash", hexTx)

	// Retry logic for transaction lookup with exponential backoff
	maxRetries := 10
	baseDelay := 50 * time.Millisecond

	var res *types.TxResult
	var additional *rpctypes.TxResultAdditionalFields
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		res, additional, err = b.GetTxByEthHash(hash)
		if err == nil {
			break // Found the transaction
		}

		if attempt < maxRetries {
			// Exponential backoff: 50ms, 100ms, 200ms
			delay := time.Duration(1<<attempt) * baseDelay
			b.Logger.Debug("tx not found, retrying", "hash", hexTx, "attempt", attempt+1, "delay", delay)
			time.Sleep(delay)
		}
	}

	if err != nil {
		b.Logger.Debug("tx not found after retries", "hash", hexTx, "error", err.Error())
		return nil, nil
	}

	resBlock, err := b.CometBlockByNumber(rpctypes.BlockNumber(res.Height))
	if err != nil {
		b.Logger.Debug("block not found", "height", res.Height, "error", err.Error())
		return nil, fmt.Errorf("block not found at height %d: %w", res.Height, err)
	}

	var ethMsg *evmtypes.MsgEthereumTx
	if additional == nil {
		// #nosec G115 always in range
		if int(res.TxIndex) >= len(resBlock.Block.Txs) {
			b.Logger.Error("tx out of bounds")
			return nil, fmt.Errorf("tx out of bounds")
		}
		tx, err := b.ClientCtx.TxConfig.TxDecoder()(resBlock.Block.Txs[res.TxIndex])
		if err != nil {
			b.Logger.Debug("decoding failed", "error", err.Error())
			return nil, fmt.Errorf("failed to decode tx: %w", err)
		}
		var ok bool
		ethMsg, ok = tx.GetMsgs()[res.MsgIndex].(*evmtypes.MsgEthereumTx)
		if !ok {
			b.Logger.Error("failed to get eth msg")
			return nil, fmt.Errorf("failed to get eth msg")
		}
	} else {
		ethMsg = b.parseDerivedTxFromAdditionalFields(additional)
		if ethMsg == nil {
			b.Logger.Error("failed to parse derived tx")
			return nil, fmt.Errorf("failed to parse tx")
		}
	}

	blockRes, err := b.RPCClient.BlockResults(b.Ctx, &res.Height)
	if err != nil {
		b.Logger.Debug("failed to retrieve block results", "height", res.Height, "error", err.Error())
		return nil, fmt.Errorf("block result not found at height %d: %w", res.Height, err)
	}

	blockHeaderHash := common.BytesToHash(resBlock.Block.Header.Hash()).Hex()
	return b.formatTxReceipt(ethMsg, res, blockRes, blockHeaderHash)
}

// GetTransactionLogs returns the transaction logs identified by hash.
func (b *Backend) GetTransactionLogs(hash common.Hash) ([]*ethtypes.Log, error) {
	hexTx := hash.Hex()

	res, _, err := b.GetTxByEthHash(hash)
	if err != nil {
		b.Logger.Debug("tx not found", "hash", hexTx, "error", err.Error())
		return nil, nil
	}

	if res.Failed {
		// failed, return empty logs
		return nil, nil
	}

	resBlockResult, err := b.RPCClient.BlockResults(b.Ctx, &res.Height)
	if err != nil {
		b.Logger.Debug("block result not found", "number", res.Height, "error", err.Error())
		return nil, nil
	}

	// parse tx logs from events
	index := int(res.MsgIndex) // #nosec G701
	return evmtypes.TxLogsFromEvents(resBlockResult.TxsResults[res.TxIndex].Events, index)
}

// GetTransactionByBlockHashAndIndex returns the transaction identified by hash and index.
func (b *Backend) GetTransactionByBlockHashAndIndex(hash common.Hash, idx hexutil.Uint) (*rpctypes.RPCTransaction, error) {
	b.Logger.Debug("eth_getTransactionByBlockHashAndIndex", "hash", hash.Hex(), "index", idx)
	sc, ok := b.ClientCtx.Client.(cmtrpcclient.SignClient)
	if !ok {
		return nil, errors.New("invalid rpc client")
	}

	block, err := sc.BlockByHash(b.Ctx, hash.Bytes())
	if err != nil {
		b.Logger.Debug("block not found", "hash", hash.Hex(), "error", err.Error())
		return nil, nil
	}

	if block.Block == nil {
		b.Logger.Debug("block not found", "hash", hash.Hex())
		return nil, nil
	}

	return b.GetTransactionByBlockAndIndex(block, idx)
}

// GetTransactionByBlockNumberAndIndex returns the transaction identified by number and index.
func (b *Backend) GetTransactionByBlockNumberAndIndex(blockNum rpctypes.BlockNumber, idx hexutil.Uint) (*rpctypes.RPCTransaction, error) {
	b.Logger.Debug("eth_getTransactionByBlockNumberAndIndex", "number", blockNum, "index", idx)

	block, err := b.CometBlockByNumber(blockNum)
	if err != nil {
		b.Logger.Debug("block not found", "height", blockNum.Int64(), "error", err.Error())
		return nil, nil
	}

	if block.Block == nil {
		b.Logger.Debug("block not found", "height", blockNum.Int64())
		return nil, nil
	}

	return b.GetTransactionByBlockAndIndex(block, idx)
}

// GetTxByEthHash uses `/tx_query` to find transaction by ethereum tx hash
// TODO: Don't need to convert once hashing is fixed on CometBFT
// https://github.com/cometbft/cometbft/issues/6539
func (b *Backend) GetTxByEthHash(hash common.Hash) (*types.TxResult, *rpctypes.TxResultAdditionalFields, error) {
	if b.Indexer != nil {
		txRes, err := b.Indexer.GetByTxHash(hash)
		if err != nil {
			return nil, nil, err
		}
		return txRes, nil, nil
	}

	// fallback to CometBFT tx indexer
	query := fmt.Sprintf("%s.%s='%s'", evmtypes.TypeMsgEthereumTx, evmtypes.AttributeKeyEthereumTxHash, hash.Hex())
	txResult, txAdditional, err := b.QueryCometTxIndexer(query, func(txs *rpctypes.ParsedTxs) *rpctypes.ParsedTx {
		return txs.GetTxByHash(hash)
	})
	if err != nil {
		return nil, nil, errorsmod.Wrapf(err, "GetTxByEthHash %s", hash.Hex())
	}
	return txResult, txAdditional, nil
}

func (b *Backend) GetTxByEthHashAndMsgIndex(hash common.Hash, index int) (*types.TxResult, *rpctypes.TxResultAdditionalFields, error) {
	if b.Indexer != nil {
		txRes, err := b.Indexer.GetByTxHash(hash)
		if err != nil {
			return nil, nil, err
		}
		return txRes, nil, nil
	}

	// fallback to CometBFT tx indexer
	query := fmt.Sprintf("%s.%s='%s'", evmtypes.TypeMsgEthereumTx, evmtypes.AttributeKeyEthereumTxHash, hash.Hex())
	txResult, txAdditional, err := b.QueryCometTxIndexer(query, func(txs *rpctypes.ParsedTxs) *rpctypes.ParsedTx {
		return txs.GetTxByMsgIndex(index)
	})
	if err != nil {
		return nil, nil, errorsmod.Wrapf(err, "GetTxByEthHash %s", hash.Hex())
	}
	return txResult, txAdditional, nil
}

// GetTxByTxIndex uses `/tx_query` to find transaction by tx index of valid ethereum txs
func (b *Backend) GetTxByTxIndex(height int64, index uint) (*types.TxResult, *rpctypes.TxResultAdditionalFields, error) {
	int32Index := int32(index) //#nosec G115 -- checked for int overflow already
	if b.Indexer != nil {
		txRes, err := b.Indexer.GetByBlockAndIndex(height, int32Index)
		if err == nil {
			return txRes, nil, nil
		}
	}

	// fallback to CometBFT tx indexer
	query := fmt.Sprintf("tx.height=%d AND %s.%s=%d",
		height, evmtypes.TypeMsgEthereumTx,
		evmtypes.AttributeKeyTxIndex, index,
	)
	txResult, txAdditional, err := b.QueryCometTxIndexer(query, func(txs *rpctypes.ParsedTxs) *rpctypes.ParsedTx {
		return txs.GetTxByTxIndex(int(index)) // #nosec G115 -- checked for int overflow already
	})
	if err != nil {
		return nil, nil, errorsmod.Wrapf(err, "GetTxByTxIndex %d %d", height, index)
	}
	return txResult, txAdditional, nil
}

// QueryCometTxIndexer query tx in CometBFT tx indexer
func (b *Backend) QueryCometTxIndexer(
	query string,
	txGetter func(*rpctypes.ParsedTxs) *rpctypes.ParsedTx,
) (*types.TxResult, *rpctypes.TxResultAdditionalFields, error) {
	resTxs, err := b.ClientCtx.Client.TxSearch(b.Ctx, query, false, nil, nil, "")
	if err != nil {
		return nil, nil, err
	}
	if len(resTxs.Txs) == 0 {
		return nil, nil, errors.New("ethereum tx not found")
	}
	txResult := resTxs.Txs[0]
	if !rpctypes.TxSucessOrExpectedFailure(&txResult.TxResult) {
		return nil, nil, errors.New("invalid ethereum tx")
	}

	var tx sdk.Tx
	if txResult.TxResult.Code != 0 {
		// it's only needed when the tx exceeds block gas limit
		tx, err = b.ClientCtx.TxConfig.TxDecoder()(txResult.Tx)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid ethereum tx")
		}
	}

	return rpctypes.ParseTxIndexerResult(txResult, tx, txGetter)
}

// GetTransactionByBlockAndIndex is the common code shared by `GetTransactionByBlockNumberAndIndex` and `GetTransactionByBlockHashAndIndex`.
func (b *Backend) GetTransactionByBlockAndIndex(block *cmtrpctypes.ResultBlock, idx hexutil.Uint) (*rpctypes.RPCTransaction, error) {
	blockRes, err := b.RPCClient.BlockResults(b.Ctx, &block.Block.Height)
	if err != nil {
		return nil, nil
	}

	// #nosec G115 always in range
	i := int(idx)
	ethMsgs, additionals := b.EthMsgsFromCometBlock(block, blockRes)
	if i >= len(ethMsgs) {
		b.Logger.Debug("block txs index out of bound", "index", i)
		return nil, nil
	}

	msg := ethMsgs[i]
	additional := additionals[i]
	baseFee, err := b.BaseFee(blockRes)
	if err != nil {
		// handle the error for pruned node.
		b.Logger.Error("failed to fetch Base Fee from prunned block. Check node prunning configuration", "height", block.Block.Height, "error", err)
	}

	height := uint64(block.Block.Height) // #nosec G115 -- checked for int overflow already
	index := uint64(idx)                 // #nosec G115 -- checked for int overflow already
	blockHash := common.BytesToHash(block.Block.Hash())
	if additional == nil {
		return rpctypes.NewTransactionFromMsg(msg, blockHash, height, index, baseFee, b.EvmChainID)
	}
	return rpctypes.NewRPCTransactionFromIncompleteMsg(msg, blockHash, height, index, baseFee, b.EvmChainID, additional.Hash)
}
