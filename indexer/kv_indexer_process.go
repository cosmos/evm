package indexer

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"

	rpctypes "github.com/cosmos/evm/rpc/types"
	servertypes "github.com/cosmos/evm/server/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// ethTx holds parsed eth tx information
type ethTx struct {
	txIndex  int
	msgIndex int
	ethHash  common.Hash        // hash from eth message (always available)
	parsedTx *rpctypes.ParsedTx // parsed tx from events (may be nil for failed txs)
	result   *abci.ExecTxResult
}

// Hash returns the ethereum tx hash
func (info *ethTx) Hash() common.Hash {
	return info.ethHash
}

// GasUsed returns the gas used from parsed tx (0 if parsedTx is nil)
func (info *ethTx) GasUsed() uint64 {
	if info.parsedTx != nil {
		return info.parsedTx.GasUsed
	}
	return 0
}

// Failed returns whether the tx failed (false if parsedTx is nil)
func (info *ethTx) Failed() bool {
	return info.parsedTx != nil && info.parsedTx.Failed
}

// processDeliverTxEvents processes DeliverTx results and indexes ethereum/cosmos transactions.
//
//  1. For failed txs:
//     - Skip txs that should not be indexed (e.g., nonce mismatch)
//     - Process eth txs with expected failure (e.g., block gas limit exceeded)
//  2. For successful txs, iterate events with buffering:
//     - Buffer cosmos events until ethereum_tx event arrives (ethereum tx events will be last of all events in a tx)
//     - On ethereum_tx: process eth tx with buffered cosmos logs merged
//     - Cosmos-only txs: create transformed tx from all buffered events
func (kv *KVIndexer) processDeliverTxEvents(
	block *cmttypes.Block,
	txResults []*abci.ExecTxResult,
) error {
	for txIdx, txBytes := range block.Txs {
		result := txResults[txIdx]

		tx, err := kv.clientCtx.TxConfig.TxDecoder()(txBytes)
		if err != nil {
			kv.logger.Error("Fail to decode tx", "err", err, "block", block.Height, "txIndex", txIdx)
			continue
		}

		cosmosTxHash := txBytes.Hash()
		isEth := isEthTx(tx)

		// 1. For failed txs:
		// - Skip failed txs that should not be indexed (e.g., nonce mismatch)
		if !rpctypes.TxSucessOrExpectedFailure(result) {
			continue
		}

		// - Process eth txs with expected failure (e.g., block gas limit exceeded)
		var ethTx *ethTx
		if isEth {
			ethTx = kv.parseEthTx(tx, result, txIdx)
		}

		if result.Code != abci.CodeTypeOK {
			if ethTx == nil { // parsing failure - skip processing this tx
				continue
			}
			ethTxIndex, cumulativeGas := kv.NextTx(ethTx.Hash(), ethTx.GasUsed())
			if err := kv.processEthereumTx(block, ethTx, nil, ethTxIndex, cumulativeGas); err != nil {
				return err
			}
			continue
		}

		// 2. Process successful tx events:
		//    - eth txs: buffer cosmos events, merge into eth receipt on ethereum_tx if it can be transformed
		//    - cosmos-only txs: create transformed tx with merged logs
		var pendingCosmosEvents []abci.Event

		for _, event := range result.Events {
			if ethTxHash := getEthTxHash(event); ethTxHash.Cmp(common.Hash{}) != 0 {
				if ethTx == nil || ethTxHash != ethTx.Hash() || kv.IsProcessed(ethTxHash) {
					continue
				}
				ethTxIndex, cumulativeGas := kv.NextTx(ethTxHash, ethTx.GasUsed())
				if err := kv.processEthereumTx(block, ethTx, pendingCosmosEvents, ethTxIndex, cumulativeGas); err != nil {
					return err
				}
				pendingCosmosEvents = nil

			} else {
				pendingCosmosEvents = append(pendingCosmosEvents, event)
			}
		}

		if !isEth && len(pendingCosmosEvents) > 0 {
			if err := kv.processCosmosEvents(block, cosmosTxHash, pendingCosmosEvents); err != nil {
				return err
			}
		}
	}
	return nil
}

// parseEthTx parses the ethereum message from a transaction.
// Protocol only allows 1 MsgEthereumTx per tx (enforced by ante handler).
func (kv *KVIndexer) parseEthTx(tx sdk.Tx, result *abci.ExecTxResult, txIndex int) *ethTx {
	msgs := tx.GetMsgs()
	if len(msgs) == 0 {
		return nil
	}
	ethMsg, ok := msgs[0].(*evmtypes.MsgEthereumTx)
	if !ok {
		return nil
	}

	parsedTxs, err := rpctypes.ParseTxResult(result, tx)
	if err != nil {
		kv.logger.Error("Fail to parse event", "err", err, "txIndex", txIndex)
	}

	var parsedTx *rpctypes.ParsedTx
	if parsedTxs != nil {
		parsedTx = parsedTxs.GetTxByMsgIndex(0)
	}

	return &ethTx{
		txIndex:  txIndex,
		msgIndex: 0,
		ethHash:  ethMsg.Hash(),
		parsedTx: parsedTx,
		result:   result,
	}
}

// processBlockPhaseEvents processes all events in a block phase as a single transformed tx.
// All transformable events in the phase are combined into one receipt with multiple logs.
func (kv *KVIndexer) processBlockPhaseEvents(
	block *cmttypes.Block,
	phase BlockPhase,
	events []abci.Event,
) error {
	ethTxHash := GenerateTransformedEthTxHash([]byte(phase), block.Hash())
	blockHash := common.BytesToHash(block.Hash())
	ethLogs, totalGasUsed := kv.trasformToEthLogs(events, block.Height, 0, ethTxHash, blockHash)
	if len(ethLogs) == 0 {
		return nil
	}

	ethTxIndex, cumulativeGas := kv.NextTx(ethTxHash, totalGasUsed)
	return kv.saveTransformedTx(block, ethTxHash, ethLogs, totalGasUsed, ethTxIndex, cumulativeGas)
}

// processEthereumTx handles ethereum_tx events and optionally appends cosmos event logs.
func (kv *KVIndexer) processEthereumTx(
	block *cmttypes.Block,
	ethTx *ethTx,
	cosmosEvents []abci.Event,
	ethTxIndex int32,
	cumulativeGas uint64,
) error {
	ethTxHash := ethTx.Hash()

	txResult := servertypes.TxResult{
		Height:            block.Height,
		TxIndex:           uint32(ethTx.txIndex),  //#nosec G115
		MsgIndex:          uint32(ethTx.msgIndex), //#nosec G115
		EthTxIndex:        ethTxIndex,
		GasUsed:           ethTx.GasUsed(),
		CumulativeGasUsed: cumulativeGas,
		Failed:            ethTx.Failed(),
	}

	if err := saveTxResult(kv.clientCtx.Codec, kv.batch, ethTxHash, &txResult); err != nil {
		return errorsmod.Wrapf(err, "IndexBlock %d", block.Height)
	}

	// Get Ethereum logs from parsed tx
	var allEthLogs []*ethtypes.Log
	blockHash := common.BytesToHash(block.Hash())
	if ethTx.parsedTx != nil && !ethTx.Failed() {
		logs, err := evmtypes.DecodeMsgLogs(
			ethTx.result.Data,
			int(ethTx.msgIndex),
			uint64(block.Height), //#nosec G115
		)
		if err != nil {
			kv.logger.Error("Failed to decode EVM logs", "err", err, "txHash", ethTxHash.Hex())
		} else {
			for i, log := range logs {
				log.Index = uint(i)
				log.BlockHash = blockHash
			}
			allEthLogs = logs
		}
	}

	// Transform cosmos events to logs and append
	if len(cosmosEvents) > 0 {
		baseIndex := uint(len(allEthLogs))
		cosmosLogs, _ := kv.trasformToEthLogs(cosmosEvents, block.Height, baseIndex, ethTxHash, blockHash)
		allEthLogs = append(allEthLogs, cosmosLogs...)
	}

	if len(allEthLogs) == 0 {
		return nil
	}

	// Build and save receipt
	var status uint64 = 1
	if ethTx.Failed() {
		status = 0
	}

	receipt := &ethtypes.Receipt{
		Type:              0,
		Status:            status,
		CumulativeGasUsed: cumulativeGas,
		Bloom:             ethtypes.CreateBloom(&ethtypes.Receipt{Logs: allEthLogs}),
		Logs:              allEthLogs,
		TxHash:            ethTxHash,
		BlockHash:         blockHash,
		BlockNumber:       big.NewInt(block.Height),
		TransactionIndex:  uint(ethTxIndex), //#nosec G115
		GasUsed:           ethTx.GasUsed(),
	}

	receiptBytes, err := json.Marshal(receipt)
	if err != nil {
		kv.logger.Error("Failed to marshal receipt", "err", err)
		return nil
	}

	if err := kv.batch.Set(EthReceiptKey(ethTxHash), receiptBytes); err != nil {
		return errorsmod.Wrapf(err, "save eth receipt")
	}

	return nil
}

// processCosmosEvents handles multiple cosmos events and merges them into a single receipt.
// All events from the same cosmos tx share the same synthetic eth tx hash.
func (kv *KVIndexer) processCosmosEvents(
	block *cmttypes.Block,
	cosmosTxHash []byte,
	events []abci.Event,
) error {
	ethTxHash := GenerateTransformedEthTxHash(cosmosTxHash)
	blockHash := common.BytesToHash(block.Hash())
	ethLogs, gasUsed := kv.trasformToEthLogs(events, block.Height, 0, ethTxHash, blockHash)
	if len(ethLogs) == 0 {
		return nil
	}

	ethTxIndex, cumulativeGas := kv.NextTx(ethTxHash, gasUsed)
	return kv.saveTransformedTx(block, ethTxHash, ethLogs, gasUsed, ethTxIndex, cumulativeGas)
}

// trasformToEthLogs transforms events into logs with proper indexing and block/tx fields.
// baseIndex is the starting log index (for appending to existing logs).
// txHash and blockHash are set on each log if non-zero.
func (kv *KVIndexer) trasformToEthLogs(
	events []abci.Event,
	height int64,
	baseIndex uint,
	txHash common.Hash,
	blockHash common.Hash,
) ([]*ethtypes.Log, uint64) {
	var logs []*ethtypes.Log
	var totalGasUsed uint64
	logIndex := baseIndex

	for _, event := range events {
		transformer := kv.findTransformer(event.Type)
		if transformer == nil {
			continue
		}

		ethData, err := transformer.Transform(event, height, txHash)
		if err != nil {
			kv.logger.Error("Failed to transform event", "err", err, "eventType", event.Type)
			continue
		}

		logIndex = ethData.UpdateEthLogs(logIndex, txHash, blockHash, height)
		logs = append(logs, ethData.Logs...)
		totalGasUsed += ethData.GasUsed
	}

	return logs, totalGasUsed
}

// saveTransformedTx saves a transformed transaction with its receipt and tx data.
// This is shared by processBlockPhaseEvents and processCosmosEvents.
// Log fields (TxHash, BlockHash, BlockNumber) are already set by trasformToEthLogs.
func (kv *KVIndexer) saveTransformedTx(
	block *cmttypes.Block,
	ethTxHash common.Hash,
	logs []*ethtypes.Log,
	gasUsed uint64,
	ethTxIndex int32,
	cumulativeGas uint64,
) error {
	// Save tx result
	txResult := servertypes.TxResult{
		Height:            block.Height,
		TxIndex:           uint32(ethTxIndex), //#nosec G115
		EthTxIndex:        ethTxIndex,
		GasUsed:           gasUsed,
		CumulativeGasUsed: cumulativeGas,
		Failed:            false,
	}
	if err := saveTxResult(kv.clientCtx.Codec, kv.batch, ethTxHash, &txResult); err != nil {
		return errorsmod.Wrapf(err, "save tx result")
	}

	// Create TransformedTxData
	ethData := &TransformedTxData{
		EthTxHash: ethTxHash,
		From:      common.Address{},
		To:        nil,
		Value:     big.NewInt(0),
		GasUsed:   gasUsed,
		Status:    1, // success
		Logs:      logs,
	}

	// Save receipt
	receipt, err := ethData.buildMarshaledReceipt(block, ethTxIndex)
	if err != nil {
		kv.logger.Error("Failed to build receipt JSON", "err", err)
		return nil
	}
	if err := kv.batch.Set(EthReceiptKey(ethTxHash), receipt); err != nil {
		return errorsmod.Wrapf(err, "save eth receipt")
	}

	// Save tx
	txbyte, err := ethData.buildMarshaledTx(block, ethTxIndex)
	if err != nil {
		kv.logger.Error("Failed to build tx JSON", "err", err)
		return nil
	}
	if err := kv.batch.Set(EthTxKey(ethTxHash), txbyte); err != nil {
		return errorsmod.Wrapf(err, "save eth tx")
	}

	return nil
}
