package network

import (
	"github.com/cosmos/evm"
	"github.com/cosmos/evm/x/vm/statedb"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

// UnitTestNetwork is the implementation of the Network interface for unit tests.
// It embeds the IntegrationNetwork struct to reuse its methods and
// makes the App public for easier testing.
type UnitTestNetwork struct {
	IntegrationNetwork
	App evm.EvmApp
}

var _ Network = (*UnitTestNetwork)(nil)

// NewUnitTestNetwork configures and initializes a new Cosmos EVM Network instance with
// the given configuration options. If no configuration options are provided
// it uses the default configuration.
//
// It panics if an error occurs.
// Note: Only uses for Unit Tests
func NewUnitTestNetwork(createEvmApp CreateEvmApp, opts ...ConfigOption) *UnitTestNetwork {
	network := New(createEvmApp, opts...)
	return &UnitTestNetwork{
		IntegrationNetwork: *network,
		App:                network.app,
	}
}

// GetStateDB returns the state database for the current block.
func (n *UnitTestNetwork) GetStateDB() *statedb.StateDB {
	return statedb.New(
		n.GetContext(),
		n.app.GetEVMKeeper(),
		statedb.NewEmptyTxConfig(),
	)
}

// FundAccount funds the given account with the given amount of coins.
func (n *UnitTestNetwork) FundAccount(addr sdktypes.AccAddress, coins sdktypes.Coins) error {
	ctx := n.GetContext()

	if err := n.app.GetBankKeeper().MintCoins(ctx, minttypes.ModuleName, coins); err != nil {
		return err
	}

	return n.app.GetBankKeeper().SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, coins)
}
