package backend

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	cmtrpctypes "github.com/cometbft/cometbft/rpc/core/types"

	rpctypes "github.com/cosmos/evm/rpc/types"
	cosmosevmtypes "github.com/cosmos/evm/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"
)

// BlockNumber returns the current block number in abci app state. Because abci
// app state could lag behind from cometbft latest block, it's more stable for
// the client to use the latest block number in abci app state than cometbft
// rpc.
func (b *Backend) BlockNumber() (hexutil.Uint64, error) {
	// do any grpc query, ignore the response and use the returned block height
	var header metadata.MD
	_, err := b.QueryClient.Params(b.Ctx, &evmtypes.QueryParamsRequest{}, grpc.Header(&header))
	if err != nil {
		return hexutil.Uint64(0), err
	}

	blockHeightHeader := header.Get(grpctypes.GRPCBlockHeightHeader)
	if headerLen := len(blockHeightHeader); headerLen != 1 {
		return 0, fmt.Errorf("unexpected '%s' gRPC header length; got %d, expected: %d", grpctypes.GRPCBlockHeightHeader, headerLen, 1)
	}

	height, err := strconv.ParseUint(blockHeightHeader[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse block height: %w", err)
	}

	return hexutil.Uint64(height), nil
}

// GetBlockByNumber returns the JSON-RPC compatible Ethereum block identified by
// block number. Depending on fullTx it either returns the full transaction
// objects or if false only the hashes of the transactions.
func (b *Backend) GetBlockByNumber(blockNum rpctypes.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	resBlock, err := b.CometBlockByNumber(blockNum)
	if err != nil {
		return nil, nil
	}

	// return if requested block height is greater than the current one
	if resBlock == nil || resBlock.Block == nil {
		return nil, nil
	}

	blockRes, err := b.RPCClient.BlockResults(b.Ctx, &resBlock.Block.Height)
	if err != nil {
		b.Logger.Debug("failed to fetch block result from CometBFT", "height", blockNum, "error", err.Error())
		return nil, nil
	}

	res, err := b.RPCBlockFromCometBlock(resBlock, blockRes, fullTx)
	if err != nil {
		b.Logger.Debug("RPCBlockFromCometBlock failed", "height", blockNum, "error", err.Error())
		return nil, err
	}

	return res, nil
}

// GetBlockByHash returns the JSON-RPC compatible Ethereum block identified by
// hash.
func (b *Backend) GetBlockByHash(hash common.Hash, fullTx bool) (map[string]interface{}, error) {
	resBlock, err := b.CometBlockByHash(hash)
	if err != nil {
		return nil, err
	}

	if resBlock == nil {
		// block not found
		return nil, nil
	}

	blockRes, err := b.RPCClient.BlockResults(b.Ctx, &resBlock.Block.Height)
	if err != nil {
		b.Logger.Debug("failed to fetch block result from CometBFT", "block-hash", hash.String(), "error", err.Error())
		return nil, nil
	}

	res, err := b.RPCBlockFromCometBlock(resBlock, blockRes, fullTx)
	if err != nil {
		b.Logger.Debug("RPCBlockFromCometBlock failed", "hash", hash, "error", err.Error())
		return nil, err
	}

	return res, nil
}

// GetBlockTransactionCountByHash returns the number of Ethereum transactions in
// the block identified by hash.
func (b *Backend) GetBlockTransactionCountByHash(hash common.Hash) *hexutil.Uint {
	block, err := b.RPCClient.BlockByHash(b.Ctx, hash.Bytes())
	if err != nil {
		b.Logger.Debug("block not found", "hash", hash.Hex(), "error", err.Error())
		return nil
	}

	if block.Block == nil {
		b.Logger.Debug("block not found", "hash", hash.Hex())
		return nil
	}

	return b.GetBlockTransactionCount(block)
}

// GetBlockTransactionCountByNumber returns the number of Ethereum transactions
// in the block identified by number.
func (b *Backend) GetBlockTransactionCountByNumber(blockNum rpctypes.BlockNumber) *hexutil.Uint {
	block, err := b.CometBlockByNumber(blockNum)
	if err != nil {
		b.Logger.Debug("block not found", "height", blockNum.Int64(), "error", err.Error())
		return nil
	}

	if block.Block == nil {
		b.Logger.Debug("block not found", "height", blockNum.Int64())
		return nil
	}

	return b.GetBlockTransactionCount(block)
}

// GetBlockTransactionCount returns the number of Ethereum transactions in a
// given block.
func (b *Backend) GetBlockTransactionCount(block *cmtrpctypes.ResultBlock) *hexutil.Uint {
	blockRes, err := b.RPCClient.BlockResults(b.Ctx, &block.Block.Height)
	if err != nil {
		return nil
	}

	ethMsgs := b.EthMsgsFromCometBlock(block, blockRes)
	n := hexutil.Uint(len(ethMsgs))
	return &n
}

// CometBlockByNumber returns a CometBFT-formatted block for a given
// block number
func (b *Backend) CometBlockByNumber(blockNum rpctypes.BlockNumber) (*cmtrpctypes.ResultBlock, error) {
	height, err := b.getHeightByBlockNum(blockNum)
	if err != nil {
		return nil, err
	}
	resBlock, err := b.RPCClient.Block(b.Ctx, &height)
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

func (b *Backend) getHeightByBlockNum(blockNum rpctypes.BlockNumber) (int64, error) {
	height := blockNum.Int64()
	if height <= 0 {
		// fetch the latest block number from the app state, more accurate than the CometBFT block store state.
		n, err := b.BlockNumber()
		if err != nil {
			return 0, err
		}
		height, err = cosmosevmtypes.SafeHexToInt64(n)
		if err != nil {
			return 0, err
		}
	}
	return height, nil
}

// CometHeaderByNumber returns a CometBFT-formatted header for a given
// block number
func (b *Backend) CometHeaderByNumber(blockNum rpctypes.BlockNumber) (*cmtrpctypes.ResultHeader, error) {
	height, err := b.getHeightByBlockNum(blockNum)
	if err != nil {
		return nil, err
	}
	return b.RPCClient.Header(b.Ctx, &height)
}

// CometBlockResultByNumber returns a CometBFT-formatted block result
// by block number
func (b *Backend) CometBlockResultByNumber(height *int64) (*cmtrpctypes.ResultBlockResults, error) {
	res, err := b.RPCClient.BlockResults(b.Ctx, height)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block result from CometBFT %d: %w", *height, err)
	}

	return res, nil
}

// CometBlockByHash returns a CometBFT-formatted block by block number
func (b *Backend) CometBlockByHash(blockHash common.Hash) (*cmtrpctypes.ResultBlock, error) {
	resBlock, err := b.RPCClient.BlockByHash(b.Ctx, blockHash.Bytes())
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

// BlockNumberFromComet returns the BlockNumber from BlockNumberOrHash
func (b *Backend) BlockNumberFromComet(blockNrOrHash rpctypes.BlockNumberOrHash) (rpctypes.BlockNumber, error) {
	switch {
	case blockNrOrHash.BlockHash == nil && blockNrOrHash.BlockNumber == nil:
		return rpctypes.EthEarliestBlockNumber, fmt.Errorf("types BlockHash and BlockNumber cannot be both nil")
	case blockNrOrHash.BlockHash != nil:
		blockNumber, err := b.BlockNumberFromCometByHash(*blockNrOrHash.BlockHash)
		if err != nil {
			return rpctypes.EthEarliestBlockNumber, err
		}
		return rpctypes.NewBlockNumber(blockNumber), nil
	case blockNrOrHash.BlockNumber != nil:
		return *blockNrOrHash.BlockNumber, nil
	default:
		return rpctypes.EthEarliestBlockNumber, nil
	}
}

// BlockNumberFromCometByHash returns the block height of given block hash
func (b *Backend) BlockNumberFromCometByHash(blockHash common.Hash) (*big.Int, error) {
	resHeader, err := b.RPCClient.HeaderByHash(b.Ctx, blockHash.Bytes())
	if err != nil {
		return nil, err
	}

	if resHeader == nil || resHeader.Header == nil {
		return nil, errors.Errorf("header not found for hash %s", blockHash.Hex())
	}

	return big.NewInt(resHeader.Header.Height), nil
}

// EthMsgsFromCometBlock returns all real MsgEthereumTxs from a
// CometBFT block. It also ensures consistency over the correct txs indexes
// across RPC endpoints
func (b *Backend) EthMsgsFromCometBlock(
	resBlock *cmtrpctypes.ResultBlock,
	blockRes *cmtrpctypes.ResultBlockResults,
) []*evmtypes.MsgEthereumTx {
	var result []*evmtypes.MsgEthereumTx
	block := resBlock.Block

	txResults := blockRes.TxsResults

	for i, tx := range block.Txs {
		// Check if tx exists on EVM by cross checking with blockResults:
		//  - Include unsuccessful tx that exceeds block gas limit
		//  - Include unsuccessful tx that failed when committing changes to stateDB
		//  - Exclude unsuccessful tx with any other error but ExceedBlockGasLimit
		if !rpctypes.TxSucessOrExpectedFailure(txResults[i]) {
			b.Logger.Debug("invalid tx result code", "cosmos-hash", hexutil.Encode(tx.Hash()))
			continue
		}

		tx, err := b.ClientCtx.TxConfig.TxDecoder()(tx)
		if err != nil {
			b.Logger.Debug("failed to decode transaction in block", "height", block.Height, "error", err.Error())
			continue
		}

		for _, msg := range tx.GetMsgs() {
			ethMsg, ok := msg.(*evmtypes.MsgEthereumTx)
			if !ok {
				continue
			}

			result = append(result, ethMsg)
		}
	}

	return result
}

// HeaderByNumber returns the block header identified by height.
func (b *Backend) HeaderByNumber(blockNum rpctypes.BlockNumber) (*ethtypes.Header, error) {
	resBlock, err := b.CometHeaderByNumber(blockNum)
	if err != nil {
		return nil, err
	}

	if resBlock == nil || resBlock.Header == nil {
		return nil, errors.Errorf("header not found for height %d", blockNum)
	}

	blockRes, err := b.CometBlockResultByNumber(&resBlock.Header.Height)
	if err != nil {
		return nil, fmt.Errorf("header result not found for height %d", resBlock.Header.Height)
	}

	bloom, err := b.BlockBloom(blockRes)
	if err != nil {
		b.Logger.Debug("HeaderByNumber BlockBloom failed", "height", resBlock.Header.Height)
	}

	baseFee, err := b.BaseFee(blockRes)
	if err != nil {
		// handle the error for pruned node.
		b.Logger.Error("failed to fetch Base Fee from prunned block. Check node prunning configuration", "height", resBlock.Header.Height, "error", err)
	}

	ethHeader := rpctypes.EthHeaderFromComet(*resBlock.Header, bloom, baseFee)
	return ethHeader, nil
}

// HeaderByHash returns the block header identified by hash.
func (b *Backend) HeaderByHash(blockHash common.Hash) (*ethtypes.Header, error) {
	resHeader, err := b.RPCClient.HeaderByHash(b.Ctx, blockHash.Bytes())
	if err != nil {
		return nil, err
	}

	if resHeader == nil || resHeader.Header == nil {
		return nil, errors.Errorf("header not found for hash %s", blockHash.Hex())
	}

	height := resHeader.Header.Height

	blockRes, err := b.RPCClient.BlockResults(b.Ctx, &resHeader.Header.Height)
	if err != nil {
		return nil, errors.Errorf("block result not found for height %d", height)
	}

	bloom, err := b.BlockBloom(blockRes)
	if err != nil {
		b.Logger.Debug("HeaderByHash BlockBloom failed", "height", height)
	}

	baseFee, err := b.BaseFee(blockRes)
	if err != nil {
		// handle the error for pruned node.
		b.Logger.Error("failed to fetch Base Fee from prunned block. Check node prunning configuration", "height", height, "error", err)
	}

	ethHeader := rpctypes.EthHeaderFromComet(*resHeader.Header, bloom, baseFee)
	return ethHeader, nil
}

// BlockBloom query block bloom filter from block results
func (b *Backend) BlockBloom(blockRes *cmtrpctypes.ResultBlockResults) (ethtypes.Bloom, error) {
	for _, event := range blockRes.FinalizeBlockEvents {
		if event.Type != evmtypes.EventTypeBlockBloom {
			continue
		}

		for _, attr := range event.Attributes {
			if attr.Key == evmtypes.AttributeKeyEthereumBloom {
				return ethtypes.BytesToBloom([]byte(attr.Value)), nil
			}
		}
	}
	return ethtypes.Bloom{}, errors.New("block bloom event is not found")
}

// RPCBlockFromCometBlock returns a JSON-RPC compatible Ethereum block from a
// given CometBFT block and its block result.
func (b *Backend) RPCBlockFromCometBlock(
	resBlock *cmtrpctypes.ResultBlock,
	blockRes *cmtrpctypes.ResultBlockResults,
	fullTx bool,
) (map[string]interface{}, error) {
	ethRPCTxs := []interface{}{}
	block := resBlock.Block

	baseFee, err := b.BaseFee(blockRes)
	if err != nil {
		// handle the error for pruned node.
		b.Logger.Error("failed to fetch Base Fee from prunned block. Check node prunning configuration", "height", block.Height, "error", err)
	}

	msgs := b.EthMsgsFromCometBlock(resBlock, blockRes)
	for txIndex, ethMsg := range msgs {
		if !fullTx {
			hash := ethMsg.Hash()
			ethRPCTxs = append(ethRPCTxs, hash)
			continue
		}

		height := uint64(block.Height) //#nosec G115 -- checked for int overflow already
		index := uint64(txIndex)       //#nosec G115 -- checked for int overflow already
		rpcTx, err := rpctypes.NewRPCTransaction(
			ethMsg,
			common.BytesToHash(block.Hash()),
			height,
			index,
			baseFee,
			b.EvmChainID,
		)
		if err != nil {
			b.Logger.Debug("NewTransactionFromData for receipt failed", "hash", ethMsg.Hash, "error", err.Error())
			continue
		}
		ethRPCTxs = append(ethRPCTxs, rpcTx)
	}

	bloom, err := b.BlockBloom(blockRes)
	if err != nil {
		b.Logger.Debug("failed to query BlockBloom", "height", block.Height, "error", err.Error())
	}

	req := &evmtypes.QueryValidatorAccountRequest{
		ConsAddress: sdk.ConsAddress(block.Header.ProposerAddress).String(),
	}

	var validatorAccAddr sdk.AccAddress

	ctx := rpctypes.ContextWithHeight(block.Height)
	res, err := b.QueryClient.ValidatorAccount(ctx, req)
	if err != nil {
		b.Logger.Debug(
			"failed to query validator operator address",
			"height", block.Height,
			"cons-address", req.ConsAddress,
			"error", err.Error(),
		)
		// use zero address as the validator operator address
		validatorAccAddr = sdk.AccAddress(common.Address{}.Bytes())
	} else {
		validatorAccAddr, err = sdk.AccAddressFromBech32(res.AccountAddress)
		if err != nil {
			return nil, err
		}
	}

	validatorAddr := common.BytesToAddress(validatorAccAddr)

	gasLimit, err := rpctypes.BlockMaxGasFromConsensusParams(ctx, b.ClientCtx, block.Height)
	if err != nil {
		b.Logger.Error("failed to query consensus params", "error", err.Error())
	}

	gasUsed := uint64(0)

	for _, txsResult := range blockRes.TxsResults {
		// workaround for cosmos-sdk bug. https://github.com/cosmos/cosmos-sdk/issues/10832
		if ShouldIgnoreGasUsed(txsResult) {
			// block gas limit has exceeded, other txs must have failed with same reason.
			break
		}
		gasUsed += uint64(txsResult.GetGasUsed()) // #nosec G115 -- checked for int overflow already
	}

	formattedBlock := rpctypes.FormatBlock(
		block.Header, block.Size(),
		gasLimit, new(big.Int).SetUint64(gasUsed),
		ethRPCTxs, bloom, validatorAddr, baseFee,
	)
	return formattedBlock, nil
}

// EthBlockByNumber returns the Ethereum Block identified by number.
func (b *Backend) EthBlockByNumber(blockNum rpctypes.BlockNumber) (*ethtypes.Block, error) {
	resBlock, err := b.CometBlockByNumber(blockNum)
	if err != nil {
		return nil, err
	}

	if resBlock == nil {
		// block not found
		return nil, fmt.Errorf("block not found for height %d", blockNum)
	}

	blockRes, err := b.RPCClient.BlockResults(b.Ctx, &resBlock.Block.Height)
	if err != nil {
		return nil, fmt.Errorf("block result not found for height %d", resBlock.Block.Height)
	}

	return b.EthBlockFromCometBlock(resBlock, blockRes)
}

// EthBlockFromCometBlock returns an Ethereum Block type from CometBFT block
func (b *Backend) EthBlockFromCometBlock(
	resBlock *cmtrpctypes.ResultBlock,
	blockRes *cmtrpctypes.ResultBlockResults,
) (*ethtypes.Block, error) {
	block := resBlock.Block
	height := block.Height
	bloom, err := b.BlockBloom(blockRes)
	if err != nil {
		b.Logger.Debug("HeaderByNumber BlockBloom failed", "height", height)
	}

	baseFee, err := b.BaseFee(blockRes)
	if err != nil {
		// handle error for pruned node and log
		b.Logger.Error("failed to fetch Base Fee from pruned block. Check node pruning configuration", "height", height, "error", err)
	}

	ethHeader := rpctypes.EthHeaderFromComet(block.Header, bloom, baseFee)
	msgs := b.EthMsgsFromCometBlock(resBlock, blockRes)

	txs := make([]*ethtypes.Transaction, len(msgs))
	for i, ethMsg := range msgs {
		txs[i] = ethMsg.AsTransaction()
	}

	// TODO: add tx receipts
	ethBlock := ethtypes.NewBlock(
		ethHeader,
		&ethtypes.Body{Transactions: txs, Uncles: nil, Withdrawals: nil},
		nil,
		trie.NewStackTrie(nil))
	return ethBlock, nil
}

// GetBlockReceipts returns the receipts for a given block number or hash.
func (b *Backend) GetBlockReceipts(
	blockNrOrHash rpctypes.BlockNumberOrHash,
) ([]map[string]interface{}, error) {
	blockNum, err := b.BlockNumberFromComet(blockNrOrHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get block number from hash: %w", err)
	}

	resBlock, err := b.CometBlockByNumber(blockNum)
	if err != nil {
		return nil, fmt.Errorf("failed to get block by number: %w", err)
	}

	if resBlock == nil {
		return nil, fmt.Errorf("block not found for height %d", *blockNum.CmtHeight())
	}

	blockRes, err := b.RPCClient.BlockResults(b.Ctx, blockNum.CmtHeight())
	if err != nil {
		return nil, fmt.Errorf("block result not found for height %d", resBlock.Block.Height)
	}

	msgs := b.EthMsgsFromCometBlock(resBlock, blockRes)
	result := make([]map[string]interface{}, len(msgs))
	blockHash := common.BytesToHash(resBlock.Block.Header.Hash()).Hex()
	for i, msg := range msgs {
		txResult, err := b.GetTxByEthHash(msg.Hash())
		if err != nil {
			return nil, fmt.Errorf("tx not found: hash=%s, error=%s", msg.Hash(), err.Error())
		}
		result[i], err = b.formatTxReceipt(
			msg,
			txResult,
			blockRes,
			blockHash,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get transaction receipt for tx %s: %w", msg.Hash().Hex(), err)
		}
	}

	return result, nil
}

func (b *Backend) formatTxReceipt(
	ethMsg *evmtypes.MsgEthereumTx,
	txResult *cosmosevmtypes.TxResult,
	blockRes *cmtrpctypes.ResultBlockResults,
	blockHeaderHash string,
) (map[string]interface{}, error) {
	ethTx := ethMsg.AsTransaction()
	cumulativeGasUsed := uint64(0)

	for _, txResult := range blockRes.TxsResults[0:txResult.TxIndex] {
		cumulativeGasUsed += uint64(txResult.GasUsed) // #nosec G115 -- checked for int overflow already
	}

	cumulativeGasUsed += txResult.CumulativeGasUsed

	var status hexutil.Uint
	if txResult.Failed {
		status = hexutil.Uint(ethtypes.ReceiptStatusFailed)
	} else {
		status = hexutil.Uint(ethtypes.ReceiptStatusSuccessful)
	}

	chainID, err := b.ChainID()
	if err != nil {
		return nil, err
	}

	from, err := ethMsg.GetSenderLegacy(ethtypes.LatestSignerForChainID(chainID.ToInt()))
	if err != nil {
		return nil, err
	}

	// parse tx logs from events
	msgIndex := int(txResult.MsgIndex) // #nosec G115 -- checked for int overflow already
	logs, err := evmtypes.TxLogsFromEvents(blockRes.TxsResults[txResult.TxIndex].Events, msgIndex)
	if err != nil {
		b.Logger.Debug("failed to parse logs", "hash", ethMsg.Hash().String(), "error", err.Error())
	}

	// return error if still unable to find the eth tx index
	if txResult.EthTxIndex == -1 {
		return nil, fmt.Errorf("can't find index of ethereum tx")
	}

	receipt := map[string]interface{}{
		// Consensus fields: These fields are defined by the Yellow Paper
		"status":            status,
		"cumulativeGasUsed": hexutil.Uint64(cumulativeGasUsed),
		"logsBloom":         ethtypes.CreateBloom(&ethtypes.Receipt{Logs: logs}),
		"logs":              logs,

		// Implementation fields: These fields are added by geth when processing a transaction.
		// They are stored in the chain database.
		"transactionHash": ethMsg.Hash(),
		"contractAddress": nil,
		"gasUsed":         hexutil.Uint64(b.GetGasUsed(txResult, ethTx.GasPrice(), ethTx.Gas())),

		// Inclusion information: These fields provide information about the inclusion of the
		// transaction corresponding to this receipt.
		"blockHash":        blockHeaderHash,
		"blockNumber":      hexutil.Uint64(txResult.Height),     //nolint:gosec // G115 // won't exceed uint64
		"transactionIndex": hexutil.Uint64(txResult.EthTxIndex), //nolint:gosec // G115 // no int overflow expected here

		// https://github.com/foundry-rs/foundry/issues/7640
		"effectiveGasPrice": (*hexutil.Big)(ethTx.GasPrice()),

		// sender and receiver (contract or EOA) addreses
		"from": from,
		"to":   ethTx.To(),
		"type": hexutil.Uint(ethMsg.AsTransaction().Type()),
	}

	if logs == nil {
		receipt["logs"] = [][]*ethtypes.Log{}
	}

	// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
	if ethTx.To() == nil {
		receipt["contractAddress"] = crypto.CreateAddress(from, ethTx.Nonce())
	}

	if ethTx.Type() >= ethtypes.DynamicFeeTxType {
		baseFee, err := b.BaseFee(blockRes)
		if err != nil {
			// tolerate the error for pruned node.
			b.Logger.Error("fetch basefee failed, node is pruned?", "height", txResult.Height, "error", err)
		} else {
			gasTip, _ := ethTx.EffectiveGasTip(baseFee)
			effectiveGasPrice := new(big.Int).Add(gasTip, baseFee)
			receipt["effectiveGasPrice"] = hexutil.Big(*effectiveGasPrice)
		}
	}

	return receipt, nil
}
