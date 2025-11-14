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
	app, ok := n.app.(ibctesting.TestingApp)
	if !ok {
		panic("network app does not implement ibctesting.TestingApp")
	}
	return &ibctesting.TestChain{
		TB:          t,
		Coordinator: coord,
		ChainID:     n.GetChainID(),
		App:         app,
		TxConfig:    app.GetTxConfig(),
		Codec:       app.AppCodec(),
		Vals:        n.valSet,
		NextVals:    n.valSet,
		Signers:     n.valSigners,
	}
}
