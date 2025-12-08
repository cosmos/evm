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
	cosmosTx, err := e.EVMTxToCosmosTx(tx)
	if err != nil {
		return nil, err
	}
	return e.CosmosTx(cosmosTx)
}

// EncodeCosmosTx encodes a cosmos tx to bytes.
func (e *TxEncoder) CosmosTx(tx sdk.Tx) ([]byte, error) {
	return e.txConfig.TxEncoder()(tx)
}

// EVMTxToCosmosTx converts an evm transaction to a cosmos transaction
func (e *TxEncoder) EVMTxToCosmosTx(tx *ethtypes.Transaction) (sdk.Tx, error) {
	// Create MsgEthereumTx from the eth transaction
	var msg evmtypes.MsgEthereumTx
	signer := ethtypes.LatestSigner(evmtypes.GetEthChainConfig())
	if err := msg.FromSignedEthereumTx(tx, signer); err != nil {
		return nil, fmt.Errorf("populating MsgEthereumTx from signed eth tx: %w", err)
	}

	// Build cosmos tx
	txBuilder := e.txConfig.NewTxBuilder()
	cosmosTx, err := msg.BuildTx(txBuilder, evmtypes.GetEVMCoinDenom())
	if err != nil {
		return nil, fmt.Errorf("failed to build cosmos tx from evm tx: %w", err)
	}
	return cosmosTx, nil
}
