package mempool

import (
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
)

var _ mempool.SignerExtractionAdapter = EthSignerExtractionAdapter{}

// EthSignerExtractionAdapter is the default implementation of SignerExtractionAdapter. It extracts the signers
// from a cosmos-sdk tx via GetSignaturesV2.
type EthSignerExtractionAdapter struct {
	fallback mempool.SignerExtractionAdapter
}

// NewEthSignerExtractionAdapter constructs a new EthSignerExtractionAdapter instance
func NewEthSignerExtractionAdapter(fallback mempool.SignerExtractionAdapter) EthSignerExtractionAdapter {
	return EthSignerExtractionAdapter{fallback}
}

// GetSigners returns the EVM tx sender. EIP-7702 authorities are NOT enumerated:
// auth.Nonce is unverifiable from tx data, so eager signaling could falsely
// evict pending txs. Async reset catches authority nonce advances instead.
// Non-EVM cosmos txs fall through to the SDK default.
func (s EthSignerExtractionAdapter) GetSigners(tx sdk.Tx) ([]mempool.SignerData, error) {
	if txWithExtensions, ok := tx.(authante.HasExtensionOptionsTx); ok {
		opts := txWithExtensions.GetExtensionOptions()
		if len(opts) > 0 && opts[0].GetTypeUrl() == "/cosmos.evm.vm.v1.ExtensionOptionsEthereumTx" {
			for _, msg := range tx.GetMsgs() {
				if ethMsg, ok := msg.(*evmtypes.MsgEthereumTx); ok {
					return []mempool.SignerData{
						mempool.NewSignerData(
							ethMsg.GetFrom(),
							ethMsg.AsTransaction().Nonce(),
						),
					}, nil
				}
			}
		}
	}

	return s.fallback.GetSigners(tx)
}
