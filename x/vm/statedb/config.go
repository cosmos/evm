package statedb

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/cosmos/evm/x/vm/types"
)

// TxConfig encapulates the readonly information of current tx for `StateDB`.
type TxConfig struct {
	TxHash  common.Hash // hash of current tx
	TxIndex uint        // the index of current transaction
}

// NewTxConfig returns a TxConfig
func NewTxConfig(thash common.Hash, txIndex uint) TxConfig {
	return TxConfig{
		TxHash:  thash,
		TxIndex: txIndex,
	}
}

// NewEmptyTxConfig construct an empty TxConfig,
// used in context where there's no transaction, e.g. `eth_call`/`eth_estimateGas`.
func NewEmptyTxConfig() TxConfig {
	return TxConfig{
		TxHash:  common.Hash{},
		TxIndex: 0,
	}
}

// EVMConfig encapsulates common parameters needed to create an EVM to execute a message
// It's mainly to reduce the number of method parameters
type EVMConfig struct {
	Params                  types.Params
	FeeMarketParams         feemarkettypes.Params
	CoinBase                common.Address
	BaseFee                 *big.Int
	EnablePreimageRecording bool
}
