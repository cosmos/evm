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
	evmApp, ok := network.app.(evm.EvmApp)
	if !ok {
		panic("provided application does not implement evm.EvmApp")
	}
	return &UnitTestNetwork{
		IntegrationNetwork: *network,
		App:                evmApp,
	}
}

// GetStateDB returns the state database for the current block.
func (n *UnitTestNetwork) GetStateDB() *statedb.StateDB {
	return statedb.New(
		n.GetContext(),
		mustGetEVMKeeper(n.app),
		statedb.NewEmptyTxConfig(),
	)
}

// FundAccount funds the given account with the given amount of coins.
func (n *UnitTestNetwork) FundAccount(addr sdktypes.AccAddress, coins sdktypes.Coins) error {
	ctx := n.GetContext()
	bankKeeper := mustGetBankKeeper(n.app)

	if err := bankKeeper.MintCoins(ctx, minttypes.ModuleName, coins); err != nil {
		return err
	}

	return bankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, coins)
}
