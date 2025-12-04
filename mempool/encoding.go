package mempool

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type TxEncoder struct {
	txConfig client.TxConfig
}

func NewTxEncoder(txConfig client.TxConfig) *TxEncoder {
	return &TxEncoder{txConfig: txConfig}
}

// EncodeEVMTx encodes an evm tx to its sdk representation as bytes.
func (e *TxEncoder) EncodeEVMTx(tx *ethtypes.Transaction) ([]byte, error) {
	// Create MsgEthereumTx from the eth transaction
	msg := &evmtypes.MsgEthereumTx{}
	msg.FromEthereumTx(tx)

	// Build cosmos tx
	txBuilder := e.txConfig.NewTxBuilder()
	if err := txBuilder.SetMsgs(msg); err != nil {
		return nil, fmt.Errorf("failed to set msg in tx builder: %w", err)
	}

	// Encode to bytes
	txBytes, err := e.txConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, fmt.Errorf("failed to encode transaction: %w", err)
	}

	return txBytes, nil
}

// EncodeCosmosTx encodes a cosmos tx to bytes.
func (e *TxEncoder) EncodeCosmosTx(tx sdk.Tx) ([]byte, error) {
	return e.txConfig.TxEncoder()(tx)
}
