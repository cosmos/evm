package mempool

import (
	"fmt"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type TxEncoder struct {
	txConfig client.TxConfig
}

func NewTxEncoder(txConfig client.TxConfig) *TxEncoder {
	return &TxEncoder{txConfig: txConfig}
}

// EncodeEVMTx encodes an evm tx to its sdk representation as bytes.
func (e *TxEncoder) EVMTx(tx *ethtypes.Transaction) ([]byte, error) {
	// Create MsgEthereumTx from the eth transaction
	msg := &evmtypes.MsgEthereumTx{}
	msg.FromEthereumTx(tx)

	// Build cosmos tx
	txBuilder := e.txConfig.NewTxBuilder()
	if err := txBuilder.SetMsgs(msg); err != nil {
		return nil, fmt.Errorf("failed to set msg in tx builder: %w", err)
	}

	return e.CosmosTx(txBuilder.GetTx())
}

// EncodeCosmosTx encodes a cosmos tx to bytes.
func (e *TxEncoder) CosmosTx(tx sdk.Tx) ([]byte, error) {
	return e.txConfig.TxEncoder()(tx)
}
