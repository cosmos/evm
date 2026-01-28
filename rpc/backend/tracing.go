package backend

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	tmrpcclient "github.com/cometbft/cometbft/rpc/client"
	tmrpctypes "github.com/cometbft/cometbft/rpc/core/types"

	rpctypes "github.com/cosmos/evm/rpc/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TraceTransaction returns the structured logs created during the execution of EVM
// and returns them as a JSON object.
func (b *Backend) TraceTransaction(hash common.Hash, config *evmtypes.TraceConfig) (interface{}, error) {
	// Get transaction by hash
	transaction, additional, err := b.GetTxByEthHash(hash)
	if err != nil {
		b.logger.Debug("tx not found", "hash", hash)
		return nil, err
	}

	// check if block number is 0
	if transaction.Height == 0 {
		return nil, errors.New("genesis is not traceable")
	}

	blk, err := b.TendermintBlockByNumber(rpctypes.BlockNumber(transaction.Height))
	if err != nil {
		b.logger.Debug("block not found", "height", transaction.Height)
		return nil, err
	}

	// check tx index is not out of bound
	if len(blk.Block.Txs) > math.MaxUint32 {
		return nil, fmt.Errorf("tx count %d is overflowing", len(blk.Block.Txs))
	}
	txsLen := uint32(len(blk.Block.Txs)) // #nosec G115 -- checked for int overflow already
	if txsLen < transaction.TxIndex {
		b.logger.Debug("tx index out of bounds", "index", transaction.TxIndex, "hash", hash.String(), "height", blk.Block.Height)
		return nil, fmt.Errorf("transaction not included in block %v", blk.Block.Height)
	}

	var predecessors []*evmtypes.MsgEthereumTx
	for i := 0; i < int(transaction.TxIndex); i++ {
		predecessorTx, txAdditional, err := b.GetTxByTxIndex(blk.Block.Height, uint(i))
		if err != nil {
			b.logger.Debug("failed to get tx by index",
				"height", blk.Block.Height,
				"index", i,
				"error", err.Error())
			continue
		}

		if txAdditional != nil {
			// This is a derived tx, fetch all derived txs from events in this Cosmos tx
			blockRes, err := b.rpcClient.BlockResults(b.ctx, &blk.Block.Height)
			if err == nil && i < len(blockRes.TxsResults) {
				txResult := blockRes.TxsResults[i]
				cosmosTx, err := b.clientCtx.TxConfig.TxDecoder()(blk.Block.Txs[i])
				if err == nil {
					parsedTxs, err := rpctypes.ParseTxResult(txResult, cosmosTx)
					if err == nil {
						for _, parsedTx := range parsedTxs.Txs {
							// Stop when we reach the current transaction
							if parsedTx.Hash == txAdditional.Hash {
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
			continue
		}

		// Fallback: decode as normal Cosmos tx
		tx, err := b.clientCtx.TxConfig.TxDecoder()(blk.Block.Txs[i])
		if err != nil {
			b.logger.Debug("failed to decode transaction in block",
				"height", blk.Block.Height,
				"index", i,
				"error", err.Error())
			continue
		}

		index := int(predecessorTx.MsgIndex)
		for j := 0; j < index; j++ {
			msg := tx.GetMsgs()[j]
			// Check if it’s a normal Ethereum tx
			if ethMsg, ok := msg.(*evmtypes.MsgEthereumTx); ok {
				predecessors = append(predecessors, ethMsg)
				continue
			}
		}
	}

	tx, err := b.clientCtx.TxConfig.TxDecoder()(blk.Block.Txs[transaction.TxIndex])
	if err != nil {
		b.logger.Debug("tx not found", "hash", hash)
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
		blockRes, err := b.rpcClient.BlockResults(b.ctx, &blk.Block.Height)
		if err == nil && int(transaction.TxIndex) < len(blockRes.TxsResults) {
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
			b.logger.Debug("invalid transaction type", "type", fmt.Sprintf("%T", tx.GetMsgs()[transaction.MsgIndex]))
			return nil, fmt.Errorf("invalid transaction type %T", tx.GetMsgs()[transaction.MsgIndex])
		}
	} else {
		ethMessage = b.parseDerivedTxFromAdditionalFields(additional)
		if ethMessage == nil {
			b.logger.Error("failed to get derived eth msg from additional fields")
			return nil, fmt.Errorf("failed to get derived eth msg from additional fields")
		}
	}

	nc, ok := b.clientCtx.Client.(tmrpcclient.NetworkClient)
	if !ok {
		return nil, errors.New("invalid rpc client")
	}

	cp, err := nc.ConsensusParams(b.ctx, &blk.Block.Height)
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
		ChainId:         b.chainID.Int64(),
		BlockMaxGas:     cp.ConsensusParams.Block.MaxGas,
	}

	if config != nil {
		traceTxRequest.TraceConfig = config
	}

	// minus one to get the context of block beginning
	contextHeight := transaction.Height - 1
	if contextHeight < 1 {
		// 0 is a special value in `ContextWithHeight`
		contextHeight = 1
	}
	traceResult, err := b.queryClient.TraceTx(rpctypes.ContextWithHeight(contextHeight), &traceTxRequest)
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

// TraceBlock configures a new tracer according to the provided configuration, and
// executes all the transactions contained within. The return value will be one item
// per transaction, dependent on the requested tracer.
func (b *Backend) TraceBlock(height rpctypes.BlockNumber,
	config *evmtypes.TraceConfig,
	block *tmrpctypes.ResultBlock,
) ([]*evmtypes.TxTraceResult, error) {
	txs := block.Block.Txs
	txsLength := len(txs)

	if txsLength == 0 {
		// If there are no transactions return empty array
		return []*evmtypes.TxTraceResult{}, nil
	}

	txDecoder := b.clientCtx.TxConfig.TxDecoder()

	var txsMessages []*evmtypes.MsgEthereumTx
	for i, tx := range txs {
		decodedTx, err := txDecoder(tx)
		if err != nil {
			b.logger.Error("failed to decode transaction", "hash", txs[i].Hash(), "error", err.Error())
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

	nc, ok := b.clientCtx.Client.(tmrpcclient.NetworkClient)
	if !ok {
		return nil, errors.New("invalid rpc client")
	}

	cp, err := nc.ConsensusParams(b.ctx, &block.Block.Height)
	if err != nil {
		return nil, err
	}

	traceBlockRequest := &evmtypes.QueryTraceBlockRequest{
		Txs:             txsMessages,
		TraceConfig:     config,
		BlockNumber:     block.Block.Height,
		BlockTime:       block.Block.Time,
		BlockHash:       common.Bytes2Hex(block.BlockID.Hash),
		ProposerAddress: sdk.ConsAddress(block.Block.ProposerAddress),
		ChainId:         b.chainID.Int64(),
		BlockMaxGas:     cp.ConsensusParams.Block.MaxGas,
	}

	res, err := b.queryClient.TraceBlock(ctxWithHeight, traceBlockRequest)
	if err != nil {
		return nil, err
	}

	decodedResults := make([]*evmtypes.TxTraceResult, txsLength)
	if err := json.Unmarshal(res.Data, &decodedResults); err != nil {
		return nil, err
	}

	return decodedResults, nil
}
