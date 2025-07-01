package types

import (
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	abci "github.com/cometbft/cometbft/abci/types"
	tmrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// EventFormat is the format version of the events.
//
// To fix the issue of tx exceeds block gas limit, we changed the event format in a breaking way.
// But to avoid forcing clients to re-sync from scatch, we make json-rpc logic to be compatible with both formats.
type EventFormat int

const (
	MessageType                    = "message"
	AmountType                     = "amount"
	SenderType                     = "sender"
	eventFormatUnknown EventFormat = iota

	// Event Format 1 (the format used before PR #1062):
	// ```
	// ethereum_tx(amount, ethereumTxHash, [txIndex, txGasUsed], txHash, [recipient], ethereumTxFailed)
	// tx_log(txLog, txLog, ...)
	// ethereum_tx(amount, ethereumTxHash, [txIndex, txGasUsed], txHash, [recipient], ethereumTxFailed)
	// tx_log(txLog, txLog, ...)
	// ...
	// ```
	eventFormat1

	// Event Format 2 (the format used after PR #1062):
	// ```
	// ethereum_tx(ethereumTxHash, txIndex)
	// ethereum_tx(ethereumTxHash, txIndex)
	// ...
	// ethereum_tx(amount, ethereumTxHash, txIndex, txGasUsed, txHash, [recipient], ethereumTxFailed)
	// tx_log(txLog, txLog, ...)
	// ethereum_tx(amount, ethereumTxHash, txIndex, txGasUsed, txHash, [recipient], ethereumTxFailed)
	// tx_log(txLog, txLog, ...)
	// ...
	// ```
	// If the transaction exceeds block gas limit, it only emits the first part.
	eventFormat2
)

// ParsedTx is the tx infos parsed from events.
type ParsedTx struct {
	MsgIndex int

	// the following fields are parsed from events

	Hash common.Hash
	// -1 means uninitialized
	EthTxIndex int32
	GasUsed    uint64
	Failed     bool

	// Additional derived cosmos EVM tx fields
	TxHash    string
	Type      uint64
	Amount    *big.Int
	Recipient common.Address
	Sender    common.Address
	Nonce     uint64
	Data      []byte
}

// NewParsedTx initialize a ParsedTx
func NewParsedTx(msgIndex int) ParsedTx {
	return ParsedTx{MsgIndex: msgIndex, EthTxIndex: -1}
}

// ParsedTxs is the tx infos parsed from eth tx events.
type ParsedTxs struct {
	// one item per message
	Txs []ParsedTx
	// map tx hash to msg index
	TxHashes map[common.Hash]int
}

// ParseTxResult parse eth tx infos from cosmos-sdk events.
// It supports two event formats, the formats are described in the comments of the format constants.
func ParseTxResult(result *abci.ExecTxResult, tx sdk.Tx) (*ParsedTxs, error) {
	format := eventFormatUnknown
	eventIndex := -1

	p := &ParsedTxs{
		TxHashes: make(map[common.Hash]int),
	}

	prevEventType := ""
	for _, event := range result.Events {
		if event.Type != evmtypes.EventTypeEthereumTx &&
			(prevEventType != evmtypes.EventTypeEthereumTx || event.Type != MessageType) {
			continue
		}

		if prevEventType == evmtypes.EventTypeEthereumTx && event.Type == MessageType && eventIndex != -1 {
			if err := fillTxAttributes(&p.Txs[eventIndex], event.Attributes); err != nil {
				return nil, err
			}
		}

		if event.Type == MessageType {
			prevEventType = MessageType
			continue
		}

		if format == eventFormatUnknown {
			// discover the format version by inspect the first ethereum_tx event.
			if len(event.Attributes) > 2 {
				format = eventFormat1
			} else {
				format = eventFormat2
			}
		}

		if len(event.Attributes) == 2 {
			// the first part of format 2
			if err := p.newTx(event.Attributes); err != nil {
				return nil, err
			}
		} else {
			// format 1 or second part of format 2
			eventIndex++
			if format == eventFormat1 {
				// append tx
				if err := p.newTx(event.Attributes); err != nil {
					return nil, err
				}
			} else {
				// the second part of format 2, update tx fields
				if err := p.updateTx(eventIndex, event.Attributes); err != nil {
					return nil, err
				}
			}
		}

		prevEventType = evmtypes.EventTypeEthereumTx
	}

	// Handle fallback GasUsed
	if len(p.Txs) == 1 && p.Txs[0].Type != evmtypes.DerivedTxType {
		p.Txs[0].GasUsed = uint64(result.GasUsed)
	}

	// Handle failure fallback
	if result.Code != 0 && tx != nil {
		for i := 0; i < len(p.Txs); i++ {
			p.Txs[i].Failed = true
			msgs := tx.GetMsgs()
			if i >= len(msgs) {
				continue
			}

			msg, ok := msgs[i].(*evmtypes.MsgEthereumTx)
			if !ok {
				continue
			}

			gasLimit := msg.GetGas()
			p.Txs[i].GasUsed = gasLimit
		}
	}

	// Fix MsgIndexes
	currMsgIndex := 0
	for _, tx := range p.Txs {
		if tx.Type == evmtypes.DerivedTxType {
			tx.MsgIndex = math.MaxUint32
		} else {
			tx.MsgIndex = currMsgIndex
			currMsgIndex++
		}
	}

	return p, nil
}

// ParseTxIndexerResult parse tm tx result to a format compatible with the custom tx indexer.
func ParseTxIndexerResult(
	txResult *tmrpctypes.ResultTx,
	tx sdk.Tx,
	getter func(*ParsedTxs) *ParsedTx,
) (*types.TxResult, *TxResultAdditionalFields, error) {
	txs, err := ParseTxResult(&txResult.TxResult, tx)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"failed to parse tx events: block %d, index %d, %v",
			txResult.Height,
			txResult.Index,
			err,
		)
	}

	parsedTx := getter(txs)
	if parsedTx == nil {
		return nil, nil, fmt.Errorf(
			"ethereum tx not found in msgs: block %d, index %d",
			txResult.Height,
			txResult.Index,
		)
	}
	if parsedTx.Type == evmtypes.DerivedTxType {
		return &types.TxResult{
				Height:  txResult.Height,
				TxIndex: txResult.Index,
				// #nosec G115 always in range
				MsgIndex:          uint32(parsedTx.MsgIndex),
				EthTxIndex:        parsedTx.EthTxIndex,
				Failed:            parsedTx.Failed,
				GasUsed:           parsedTx.GasUsed,
				CumulativeGasUsed: txs.AccumulativeGasUsed(parsedTx.MsgIndex),
			}, &TxResultAdditionalFields{
				Value:     parsedTx.Amount,
				Hash:      parsedTx.Hash,
				TxHash:    parsedTx.TxHash,
				Type:      parsedTx.Type,
				Recipient: parsedTx.Recipient,
				Sender:    parsedTx.Sender,
				GasUsed:   parsedTx.GasUsed,
				Data:      parsedTx.Data,
				Nonce:     parsedTx.Nonce,
			}, nil
	}
	return &types.TxResult{
		Height:  txResult.Height,
		TxIndex: txResult.Index,
		// #nosec G115 always in range
		MsgIndex:          uint32(parsedTx.MsgIndex),
		EthTxIndex:        parsedTx.EthTxIndex,
		Failed:            parsedTx.Failed,
		GasUsed:           parsedTx.GasUsed,
		CumulativeGasUsed: txs.AccumulativeGasUsed(parsedTx.MsgIndex),
	}, nil, nil
}

// ParseTxBlockResult converts an ABCI tx result into indexable EVM-compatible tx metadata, including internal Cosmos EVM txs.
func ParseTxBlockResult(
	txResult *abci.ExecTxResult,
	tx sdk.Tx,
	txIndex int,
	height int64,
) (*types.TxResult, *TxResultAdditionalFields, error) {
	txs, err := ParseTxResult(txResult, tx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse tx events: block %d, index %d, %v", height, txIndex, err)
	}

	if len(txs.Txs) == 0 {
		return nil, nil, nil
	}
	// TODO: check why when there are multiple derived txs events are in reversed order
	parsedTx := txs.Txs[len(txs.Txs)-1]
	if parsedTx.Type == evmtypes.DerivedTxType {
		return &types.TxResult{
				Height: height,
				// #nosec G115 always in range
				TxIndex: uint32(txIndex),
				// #nosec G115 always in range
				MsgIndex:          uint32(parsedTx.MsgIndex),
				EthTxIndex:        parsedTx.EthTxIndex,
				Failed:            parsedTx.Failed,
				GasUsed:           parsedTx.GasUsed,
				CumulativeGasUsed: txs.AccumulativeGasUsed(parsedTx.MsgIndex),
			}, &TxResultAdditionalFields{
				Value:     parsedTx.Amount,
				Hash:      parsedTx.Hash,
				TxHash:    parsedTx.TxHash,
				Type:      parsedTx.Type,
				Recipient: parsedTx.Recipient,
				Sender:    parsedTx.Sender,
				GasUsed:   parsedTx.GasUsed,
				Data:      parsedTx.Data,
				Nonce:     parsedTx.Nonce,
			}, nil
	}
	return &types.TxResult{
		Height: height,
		// #nosec G115 always in range
		TxIndex: uint32(txIndex),
		// #nosec G115 always in range
		MsgIndex:          uint32(parsedTx.MsgIndex),
		EthTxIndex:        parsedTx.EthTxIndex,
		Failed:            parsedTx.Failed,
		GasUsed:           parsedTx.GasUsed,
		CumulativeGasUsed: txs.AccumulativeGasUsed(parsedTx.MsgIndex),
	}, nil, nil
}

func (p *ParsedTxs) newTx(attrs []abci.EventAttribute) error {
	msgIndex := len(p.Txs)
	tx := NewParsedTx(msgIndex)
	if err := fillTxAttributes(&tx, attrs); err != nil {
		return err
	}
	p.Txs = append(p.Txs, tx)
	p.TxHashes[tx.Hash] = msgIndex
	return nil
}

// updateTx updates an exiting tx from events, called during parsing.
// In event format 2, we update the tx with the attributes of the second `ethereum_tx` event,
// Due to bug https://github.com/evmos/ethermint/issues/1175, the first `ethereum_tx` event may emit incorrect tx hash,
// so we prefer the second event and override the first one.
func (p *ParsedTxs) updateTx(eventIndex int, attrs []abci.EventAttribute) error {
	tx := NewParsedTx(eventIndex)
	if err := fillTxAttributes(&tx, attrs); err != nil {
		return err
	}
	if tx.Hash != p.Txs[eventIndex].Hash {
		// if hash is different, index the new one too
		p.TxHashes[tx.Hash] = eventIndex
	}
	// override the tx because the second event is more trustworthy
	p.Txs[eventIndex] = tx
	return nil
}

// GetTxByHash find ParsedTx by tx hash, returns nil if not exists.
func (p *ParsedTxs) GetTxByHash(hash common.Hash) *ParsedTx {
	if idx, ok := p.TxHashes[hash]; ok {
		return &p.Txs[idx]
	}
	return nil
}

// GetTxByMsgIndex returns ParsedTx by msg index
func (p *ParsedTxs) GetTxByMsgIndex(i int) *ParsedTx {
	if i < 0 || i >= len(p.Txs) {
		return nil
	}
	return &p.Txs[i]
}

// GetTxByTxIndex returns ParsedTx by tx index
func (p *ParsedTxs) GetTxByTxIndex(txIndex int) *ParsedTx {
	if len(p.Txs) == 0 {
		return nil
	}
	// assuming the `EthTxIndex` increase continuously,
	// convert TxIndex to MsgIndex by subtract the begin TxIndex.
	msgIndex := txIndex - int(p.Txs[0].EthTxIndex)
	// GetTxByMsgIndex will check the bound
	return p.GetTxByMsgIndex(msgIndex)
}

// AccumulativeGasUsed calculates the accumulated gas used within the batch of txs
func (p *ParsedTxs) AccumulativeGasUsed(msgIndex int) (result uint64) {
	for i := 0; i <= msgIndex; i++ {
		result += p.Txs[i].GasUsed
	}
	return result
}

// fillTxAttribute parse attributes by name, less efficient than hardcode the index, but more stable against event
// format changes.
func fillTxAttribute(tx *ParsedTx, key, value string) error {
	switch key {
	case evmtypes.AttributeKeyEthereumTxHash:
		tx.Hash = common.HexToHash(value)
	case evmtypes.AttributeKeyTxIndex:
		txIndex, err := strconv.ParseUint(value, 10, 31)
		if err != nil {
			return err
		}
		// #nosec G115 always in range
		tx.EthTxIndex = int32(txIndex)
	case evmtypes.AttributeKeyTxGasUsed:
		gasUsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		tx.GasUsed = gasUsed
	case evmtypes.AttributeKeyEthereumTxFailed:
		tx.Failed = len(value) > 0
	case SenderType:
		tx.Sender = common.HexToAddress(value)
	case evmtypes.AttributeKeyRecipient:
		tx.Recipient = common.HexToAddress(value)
	case evmtypes.AttributeKeyTxHash:
		tx.TxHash = value
	case evmtypes.AttributeKeyTxType:
		txType, err := strconv.ParseUint(value, 10, 31)
		if err != nil {
			return err
		}
		tx.Type = txType
	case AmountType:
		var success bool
		tx.Amount, success = big.NewInt(0).SetString(value, 10)
		if !success {
			return nil
		}
	case evmtypes.AttributeKeyTxNonce:
		nonce, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		tx.Nonce = nonce

	case evmtypes.AttributeKeyTxData:
		hexBytes, err := hexutil.Decode(value)
		if err != nil {
			return err
		}
		tx.Data = hexBytes
	}
	return nil
}

func fillTxAttributes(tx *ParsedTx, attrs []abci.EventAttribute) error {
	for _, attr := range attrs {
		if err := fillTxAttribute(tx, attr.Key, attr.Value); err != nil {
			return err
		}
	}
	return nil
}
