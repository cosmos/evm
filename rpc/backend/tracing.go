package backend

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"

	tmrpcclient "github.com/cometbft/cometbft/rpc/client"
	tmrpctypes "github.com/cometbft/cometbft/rpc/core/types"

	rpctypes "github.com/cosmos/evm/rpc/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TraceTransaction returns the structured logs created during the execution of EVM
// and returns them as a JSON object.
func (b *Backend) TraceTransaction(hash common.Hash, config *rpctypes.TraceConfig) (interface{}, error) {
	// Get transaction by hash
	transaction, additional, err := b.GetTxByEthHash(hash)
	if err != nil {
		b.Logger.Debug("tx not found", "hash", hash)
		return nil, err
	}

	// check if block number is 0
	if transaction.Height == 0 {
		return nil, errors.New("genesis is not traceable")
	}

	blk, err := b.CometBlockByNumber(rpctypes.BlockNumber(transaction.Height))
	if err != nil {
		b.Logger.Debug("block not found", "height", transaction.Height)
		return nil, err
	}

	// check tx index is not out of bound
	if len(blk.Block.Txs) > math.MaxUint32 {
		return nil, fmt.Errorf("tx count %d is overflowing", len(blk.Block.Txs))
	}
	txsLen := uint32(len(blk.Block.Txs)) // #nosec G115 -- checked for int overflow already
	if txsLen < transaction.TxIndex {
		b.Logger.Debug("tx index out of bounds", "index", transaction.TxIndex, "hash", hash.String(), "height", blk.Block.Height)
		return nil, fmt.Errorf("transaction not included in block %v", blk.Block.Height)
	}

	var predecessors []*evmtypes.MsgEthereumTx
	// Use EthTxIndex (Ethereum execution counter) as the loop bound, not TxIndex
	// (Cosmos tx slot). The two diverge whenever a Cosmos tx holds multiple EVM
	// messages, contains no EVM messages, or derived txs shift the counter.
	ethTxCount := int(transaction.EthTxIndex)
	if ethTxCount < 0 {
		ethTxCount = 0
	}
	for i := 0; i < ethTxCount; i++ {
		predecessorTx, txAdditional, err := b.GetTxByTxIndex(blk.Block.Height, uint(i))
		if err != nil {
			b.Logger.Debug("failed to get tx by index",
				"height", blk.Block.Height,
				"index", i,
				"error", err.Error())
			continue
		}

		// The after-loop section below handles all predecessors that share the same
		// Cosmos tx slot as the target (intra-tx ordering by MsgIndex / derived-tx
		// event order). Skip them here to avoid double-counting.
		if int(predecessorTx.TxIndex) == int(transaction.TxIndex) {
			continue
		}

		if txAdditional != nil {
			// Derived tx: add it directly. The old approach scanned parsedTxs.Txs
			// for "all derived txs before txAdditional.Hash", which (a) skipped the
			// tx at txAdditional.Hash itself — so the last derived tx in a series
			// was always missed — and (b) double-counted earlier derived txs that
			// were already added by their own outer-loop iterations. Each iteration
			// of this loop corresponds to exactly one Ethereum execution, so adding
			// txAdditional directly is both correct and complete.
			ethMsg := b.parseDerivedTxFromAdditionalFields(txAdditional)
			if ethMsg != nil {
				predecessors = append(predecessors, ethMsg)
			}
			continue
		}

		// Fallback: decode as normal Cosmos tx. Use predecessorTx.TxIndex (Cosmos slot)
		// rather than i (Ethereum index) to address the correct block entry.
		tx, err := b.ClientCtx.TxConfig.TxDecoder()(blk.Block.Txs[predecessorTx.TxIndex])
		if err != nil {
			b.Logger.Debug("failed to decode transaction in block",
				"height", blk.Block.Height,
				"index", i,
				"error", err.Error())
			continue
		}

		// Add the EVM message at this Ethereum index directly. The inner loop used
		// here previously ran j < MsgIndex, which added only messages BEFORE the
		// current position and left the message AT MsgIndex itself unhandled —
		// causing the last message of any multi-message predecessor Cosmos tx to
		// be silently dropped from the predecessor set.
		if ethMsg, ok := tx.GetMsgs()[int(predecessorTx.MsgIndex)].(*evmtypes.MsgEthereumTx); ok {
			predecessors = append(predecessors, ethMsg)
		}
	}

	tx, err := b.ClientCtx.TxConfig.TxDecoder()(blk.Block.Txs[transaction.TxIndex])
	if err != nil {
		b.Logger.Debug("tx not found", "hash", hash)
		return nil, err
	}

	// add predecessor messages in current cosmos tx
	index := int(transaction.MsgIndex) // #nosec G115

	for i := 0; i < index; i++ {
		msg := tx.GetMsgs()[i]
		// Check if it's a normal Ethereum tx
		if ethMsg, ok := msg.(*evmtypes.MsgEthereumTx); ok {
			predecessors = append(predecessors, ethMsg)
			continue
		}
	}

	// For derived transactions, parse all derived txs from the current Cosmos tx's events
	if additional != nil {
		// This is a derived tx, fetch all derived txs from events in this Cosmos tx
		blockRes, err := b.RPCClient.BlockResults(b.Ctx, &blk.Block.Height)
		if err == nil && blockRes != nil && int(transaction.TxIndex) < len(blockRes.TxsResults) {
			txResult := blockRes.TxsResults[transaction.TxIndex]
			parsedTxs, err := rpctypes.ParseTxResult(txResult, tx)
			if err == nil {
				// Add all derived txs that come before the current one as predecessors
				for _, parsedTx := range parsedTxs.Txs {
					// Stop when we reach the current transaction
					if parsedTx.Hash == additional.Hash {
						break
					}
					// Only include derived txs
					if parsedTx.Type == evmtypes.DerivedTxType {
						ethMsg := b.parseDerivedTxFromAdditionalFields(&rpctypes.TxResultAdditionalFields{
							Value:     parsedTx.Amount,
							Hash:      parsedTx.Hash,
							TxHash:    parsedTx.TxHash,
							Type:      parsedTx.Type,
							Recipient: parsedTx.Recipient,
							Sender:    parsedTx.Sender,
							GasUsed:   parsedTx.GasUsed,
							Data:      parsedTx.Data,
							Nonce:     parsedTx.Nonce,
							GasLimit:  &parsedTx.GasLimit,
						})
						if ethMsg != nil {
							predecessors = append(predecessors, ethMsg)
						}
					}
				}
			}
		}
	}

	var ethMessage *evmtypes.MsgEthereumTx
	var ok bool

	if additional == nil {
		ethMessage, ok = tx.GetMsgs()[transaction.MsgIndex].(*evmtypes.MsgEthereumTx)
		if !ok {
			b.Logger.Debug("invalid transaction type", "type", fmt.Sprintf("%T", tx.GetMsgs()[transaction.MsgIndex]))
			return nil, fmt.Errorf("invalid transaction type %T", tx.GetMsgs()[transaction.MsgIndex])
		}
	} else {
		ethMessage = b.parseDerivedTxFromAdditionalFields(additional)
		if ethMessage == nil {
			b.Logger.Error("failed to get derived eth msg from additional fields")
			return nil, fmt.Errorf("failed to get derived eth msg from additional fields")
		}
	}

	nc, ok := b.ClientCtx.Client.(tmrpcclient.NetworkClient)
	if !ok {
		return nil, errors.New("invalid rpc client")
	}

	cp, err := nc.ConsensusParams(b.Ctx, &blk.Block.Height)
	if err != nil {
		return nil, err
	}

	traceTxRequest := evmtypes.QueryTraceTxRequest{
		Msg:             ethMessage,
		Predecessors:    predecessors,
		BlockNumber:     blk.Block.Height,
		BlockTime:       blk.Block.Time,
		BlockHash:       common.Bytes2Hex(blk.BlockID.Hash),
		ProposerAddress: sdk.ConsAddress(blk.Block.ProposerAddress),
		ChainId:         b.EvmChainID.Int64(),
		BlockMaxGas:     cp.ConsensusParams.Block.MaxGas,
	}

	if config != nil {
		traceTxRequest.TraceConfig = b.convertConfig(config)
	}

	// minus one to get the context of block beginning
	contextHeight := transaction.Height - 1
	if contextHeight < 1 {
		// In Ethereum, the genesis block height is 0, but in CometBFT, the genesis block height is 1.
		// So here we set the minimum requested height to 1.
		contextHeight = 1
	}
	traceResult, err := b.QueryClient.TraceTx(rpctypes.ContextWithHeight(contextHeight), &traceTxRequest)
	if err != nil {
		return nil, err
	}

	// Response format is unknown due to custom tracer config param
	// More information can be found here https://geth.ethereum.org/docs/dapp/tracing-filtered
	var decodedResult interface{}
	err = json.Unmarshal(traceResult.Data, &decodedResult)
	if err != nil {
		return nil, err
	}

	return decodedResult, nil
}

func (b *Backend) convertConfig(config *rpctypes.TraceConfig) *evmtypes.TraceConfig {
	if config == nil {
		return &evmtypes.TraceConfig{}
	}
	cfg := config.TraceConfig
	cfg.TracerJsonConfig = string(config.TracerConfig)
	return &cfg
}

// TraceBlock configures a new tracer according to the provided configuration, and
// executes all the transactions contained within. The return value will be one item
// per transaction, dependent on the requested tracer.
func (b *Backend) TraceBlock(height rpctypes.BlockNumber,
	config *rpctypes.TraceConfig,
	block *tmrpctypes.ResultBlock,
) ([]*evmtypes.TxTraceResult, error) {
	txs := block.Block.Txs
	txsLength := len(txs)

	if txsLength == 0 {
		// If there are no transactions return empty array
		return []*evmtypes.TxTraceResult{}, nil
	}

	blockRes, err := b.CometBlockResultByNumber(&block.Block.Height)
	if err != nil {
		b.Logger.Debug("block result not found", "height", block.Block.Height, "error", err.Error())
		return nil, nil
	}
	txDecoder := b.ClientCtx.TxConfig.TxDecoder()

	var txsMessages []*evmtypes.MsgEthereumTx
	for i, tx := range txs {
		if !rpctypes.TxSucessOrExpectedFailure(blockRes.TxsResults[i]) {
			b.Logger.Debug("invalid tx result code", "cosmos-hash", hexutil.Encode(tx.Hash()))
			continue
		}
		decodedTx, err := txDecoder(tx)
		if err != nil {
			b.Logger.Error("failed to decode transaction", "hash", txs[i].Hash(), "error", err.Error())
			continue
		}

		for _, msg := range decodedTx.GetMsgs() {
			ethMessage, ok := msg.(*evmtypes.MsgEthereumTx)
			if !ok {
				// Just considers Ethereum transactions
				continue
			}
			txsMessages = append(txsMessages, ethMessage)
		}
	}

	// minus one to get the context at the beginning of the block
	contextHeight := height - 1
	if contextHeight < 1 {
		// 0 is a special value for `ContextWithHeight`.
		contextHeight = 1
	}
	ctxWithHeight := rpctypes.ContextWithHeight(int64(contextHeight))

	nc, ok := b.ClientCtx.Client.(tmrpcclient.NetworkClient)
	if !ok {
		return nil, errors.New("invalid rpc client")
	}

	cp, err := nc.ConsensusParams(b.Ctx, &block.Block.Height)
	if err != nil {
		return nil, err
	}

	traceBlockRequest := &evmtypes.QueryTraceBlockRequest{
		Txs:             txsMessages,
		TraceConfig:     b.convertConfig(config),
		BlockNumber:     block.Block.Height,
		BlockTime:       block.Block.Time,
		BlockHash:       common.Bytes2Hex(block.BlockID.Hash),
		ProposerAddress: sdk.ConsAddress(block.Block.ProposerAddress),
		ChainId:         b.EvmChainID.Int64(),
		BlockMaxGas:     cp.ConsensusParams.Block.MaxGas,
	}

	res, err := b.QueryClient.TraceBlock(ctxWithHeight, traceBlockRequest)
	if err != nil {
		return nil, err
	}

	decodedResults := make([]*evmtypes.TxTraceResult, txsLength)
	if err := json.Unmarshal(res.Data, &decodedResults); err != nil {
		return nil, err
	}

	return decodedResults, nil
}

// TraceCall executes a call with the given arguments and returns the structured logs
// created during the execution of EVM. It returns them as a JSON object.
func (b *Backend) TraceCall(
	args evmtypes.TransactionArgs,
	blockNrOrHash rpctypes.BlockNumberOrHash,
	config *rpctypes.TraceConfig,
) (interface{}, error) {
	// Marshal tx args
	bz, err := json.Marshal(&args)
	if err != nil {
		return nil, err
	}

	// Get block number from blockNrOrHash
	blockNr, err := b.BlockNumberFromComet(blockNrOrHash)
	if err != nil {
		return nil, err
	}

	// Get the block to get necessary context
	header, err := b.CometHeaderByNumber(blockNr)
	if err != nil {
		b.Logger.Debug("block not found", "number", blockNr)
		return nil, err
	}

	traceCallRequest := evmtypes.QueryTraceCallRequest{
		Args:            bz,
		GasCap:          b.RPCGasCap(),
		ProposerAddress: sdk.ConsAddress(header.Header.ProposerAddress),
		BlockNumber:     header.Header.Height,
		BlockHash:       common.Bytes2Hex(header.Header.Hash()),
		BlockTime:       header.Header.Time,
		ChainId:         b.EvmChainID.Int64(),
	}

	if config != nil {
		traceCallRequest.TraceConfig = b.convertConfig(config)
	}

	// get the context of provided block
	contextHeight := header.Header.Height
	if contextHeight < 1 {
		// In Ethereum, the genesis block height is 0, but in CometBFT, the genesis block height is 1.
		// So here we set the minimum requested height to 1.
		contextHeight = 1
	}

	// Use the block height as context for the query
	ctxWithHeight := rpctypes.ContextWithHeight(contextHeight)
	traceResult, err := b.QueryClient.TraceCall(ctxWithHeight, &traceCallRequest)
	if err != nil {
		return nil, err
	}

	// Response format is unknown due to custom tracer config param
	// More information can be found here https://geth.ethereum.org/docs/dapp/tracing-filtered
	var decodedResult interface{}
	err = json.Unmarshal(traceResult.Data, &decodedResult)
	if err != nil {
		return nil, err
	}

	return decodedResult, nil
}
