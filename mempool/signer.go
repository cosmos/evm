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
// whether each authorization succeeds on chain (and therefore actually bumps
// the authority's nonce) is unverifiable without chain state — a failed
// authorization does not increment the nonce. Eagerly reporting auth.Nonce
// would falsely evict legitimate pending txs from authorities whose auths
// failed. Async reset catches authority nonce advances instead.
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
