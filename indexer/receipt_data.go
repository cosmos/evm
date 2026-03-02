package indexer

import (
	"encoding/json"
	"math/big"

	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	rpctypes "github.com/cosmos/evm/rpc/types"
)

// EthReceiptData represents the transformed EVM receipt/tx data from cosmos events.
// The transformer populates this struct, which then determines the content of
// GetEthReceipt and GetEthTx responses for synthetic cosmos transactions.
type EthReceiptData struct {
	EthTxHash common.Hash
	From      common.Address
	To        *common.Address
	Value     *big.Int
	GasUsed   uint64
	Status    uint64 // 1=success, 0=fail
	Logs      []*ethtypes.Log

	// Signature fields (optional, from cosmos tx signature)
	// If nil, defaults to 0
	V *big.Int
	R *big.Int
	S *big.Int
}

// NewEthReceiptData creates a new EthReceiptData with required fields.
func NewEthReceiptData(
	ethTxHash common.Hash,
	from common.Address,
	to *common.Address,
	value *big.Int,
	gasUsed, status uint64,
	logs []*ethtypes.Log,
) *EthReceiptData {
	return &EthReceiptData{
		EthTxHash: ethTxHash,
		From:      from,
		To:        to,
		Value:     value,
		GasUsed:   gasUsed,
		Status:    status,
		Logs:      logs,
	}
}

// getReceipt creates an ethtypes.Receipt from EthReceiptData
func (d *EthReceiptData) getReceipt(blockHash common.Hash, blockNumber int64, txIndex uint) *ethtypes.Receipt {
	logs := d.Logs
	if logs == nil {
		logs = []*ethtypes.Log{}
	}

	// Calculate bloom filter from logs
	bloom := ethtypes.CreateBloom(&ethtypes.Receipt{Logs: logs})

	return &ethtypes.Receipt{
		Type:              0,
		Status:            d.Status,
		CumulativeGasUsed: d.GasUsed,
		Bloom:             bloom,
		Logs:              logs,
		TxHash:            d.EthTxHash,
		ContractAddress:   common.Address{},
		GasUsed:           d.GasUsed,
		BlockHash:         blockHash,
		BlockNumber:       big.NewInt(blockNumber),
		TransactionIndex:  txIndex,
	}
}

// getRPCTransaction creates an rpctypes.RPCTransaction from EthReceiptData
func (d *EthReceiptData) getRPCTransaction(blockHash common.Hash, blockNumber int64, txIndex uint) *rpctypes.RPCTransaction {
	blockNum := hexutil.Big(*big.NewInt(blockNumber))
	txIdx := hexutil.Uint64(txIndex)

	value := hexutil.Big(*big.NewInt(0))
	if d.Value != nil {
		value = hexutil.Big(*d.Value)
	}

	// Use signature values if provided, otherwise default to 0
	v := big.NewInt(0)
	r := big.NewInt(0)
	s := big.NewInt(0)
	if d.V != nil {
		v = d.V
	}
	if d.R != nil {
		r = d.R
	}
	if d.S != nil {
		s = d.S
	}

	return &rpctypes.RPCTransaction{
		BlockHash:        &blockHash,
		BlockNumber:      &blockNum,
		From:             d.From,
		Gas:              hexutil.Uint64(d.GasUsed),
		GasPrice:         (*hexutil.Big)(big.NewInt(0)),
		Hash:             d.EthTxHash,
		Input:            []byte{},
		Nonce:            0,
		To:               d.To,
		TransactionIndex: &txIdx,
		Value:            &value,
		Type:             0,
		V:                (*hexutil.Big)(v),
		R:                (*hexutil.Big)(r),
		S:                (*hexutil.Big)(s),
	}
}

// buildMarshaledReceipt builds the eth receipt JSON from EthReceiptData
func (d *EthReceiptData) buildMarshaledReceipt(block *cmttypes.Block, ethTxIndex int32) ([]byte, error) {
	blockHash := common.BytesToHash(block.Hash())
	receipt := d.getReceipt(blockHash, block.Height, uint(ethTxIndex)) //#nosec G115
	return json.Marshal(receipt)
}

// buildMarshaledTx builds the eth tx JSON from EthReceiptData
func (d *EthReceiptData) buildMarshaledTx(block *cmttypes.Block, ethTxIndex int32) ([]byte, error) {
	blockHash := common.BytesToHash(block.Hash())
	rpcTx := d.getRPCTransaction(blockHash, block.Height, uint(ethTxIndex)) //#nosec G115
	return json.Marshal(rpcTx)
}

// UpdateEthLogs sets block/tx fields on each log and returns the next log index.
func (d *EthReceiptData) UpdateEthLogs(baseIndex uint, txHash, blockHash common.Hash, height int64) uint {
	for i, log := range d.Logs {
		log.Index = baseIndex + uint(i)
		log.TxHash = txHash
		log.BlockHash = blockHash
		log.BlockNumber = uint64(height) //#nosec G115
	}
	return baseIndex + uint(len(d.Logs))
}
