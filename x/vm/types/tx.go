package types

import (
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

// EvmTxArgs encapsulates all possible params to create all EVM txs types.
// This includes LegacyTx, DynamicFeeTx and AccessListTx
type EvmTxArgs struct {
	Nonce     uint64
	GasLimit  uint64
	Input     []byte
	GasFeeCap *big.Int
	GasPrice  *big.Int
	ChainID   *big.Int
	Amount    *big.Int
	GasTipCap *big.Int
	To        *common.Address
	Accesses  *ethtypes.AccessList

	// For SetCodeTxType
	AuthorizationList []ethtypes.SetCodeAuthorization `json:"authorizationList"`
}

// ToTxData converts the EvmTxArgs to TxData
func (args *EvmTxArgs) ToTxData() *TransactionArgs {
	return &TransactionArgs{
		Nonce:                (*hexutil.Uint64)(&args.Nonce),
		Gas:                  (*hexutil.Uint64)(&args.GasLimit),
		Data:                 (*hexutil.Bytes)(&args.Input),
		MaxFeePerGas:         (*hexutil.Big)(args.GasFeeCap),
		GasPrice:             (*hexutil.Big)(args.GasPrice),
		ChainID:              (*hexutil.Big)(args.ChainID),
		MaxPriorityFeePerGas: (*hexutil.Big)(args.GasTipCap),
		Value:                (*hexutil.Big)(args.Amount),
		To:                   args.To,
		AccessList:           args.Accesses,
		AuthorizationList:    args.AuthorizationList,
	}
}

func (args *EvmTxArgs) ToTx() *ethtypes.Transaction {
	return args.ToTxData().ToTransaction(ethtypes.LegacyTxType)
}

// GetTxPriority returns the priority of a given Ethereum tx. It relies of the
// priority reduction global variable to calculate the tx priority given the tx
// tip price:
//
//	tx_priority = tip_price / priority_reduction
func GetTxPriority(ethTx *ethtypes.Transaction, baseFee *big.Int) (priority int64) {
	// calculate priority based on effective gas price
	tipPrice, _ := ethTx.EffectiveGasTip(baseFee)
	priority = math.MaxInt64
	priorityBig := new(big.Int).Quo(tipPrice, DefaultPriorityReduction.BigInt())

	// safety check
	if priorityBig.IsInt64() {
		priority = priorityBig.Int64()
	}

	return priority
}

// Failed returns if the contract execution failed in vm errors
func (m *MsgEthereumTxResponse) Failed() bool {
	return len(m.VmError) > 0
}

// Return is a helper function to help caller distinguish between revert reason
// and function return. Return returns the data after execution if no error occurs.
func (m *MsgEthereumTxResponse) Return() []byte {
	if m.Failed() {
		return nil
	}
	return common.CopyBytes(m.Ret)
}

// Revert returns the concrete revert reason if the execution is aborted by `REVERT`
// opcode. Note the reason can be nil if no data supplied with revert opcode.
func (m *MsgEthereumTxResponse) Revert() []byte {
	if m.VmError != vm.ErrExecutionReverted.Error() {
		return nil
	}
	return common.CopyBytes(m.Ret)
}
