package network

import (
	"testing"

	ibctesting "github.com/cosmos/ibc-go/v10/testing"
)

// GetIBCChain returns a TestChain instance for the given network.
// Note: the sender accounts are not populated. Do not use this accounts to send transactions during tests.
// The keyring should be used instead.
func (n *IntegrationNetwork) GetIBCChain(t *testing.T, coord *ibctesting.Coordinator) *ibctesting.TestChain {
	t.Helper()
	return &ibctesting.TestChain{
		TB:          t,
		Coordinator: coord,
		ChainID:     n.GetChainID(),
		App:         n.app,
		TxConfig:    n.app.GetTxConfig(),
		Codec:       n.app.AppCodec(),
		Vals:        n.valSet,
		NextVals:    n.valSet,
		Signers:     n.valSigners,
	}
}
