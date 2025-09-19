package eip712

import (
	types2 "github.com/cosmos/evm/ante/types"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// RegisterInterfaces registers the CometBFT concrete client-related
// implementations and interfaces.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations(
		(*sdktypes.AccountI)(nil),
		// TODO: uncomment after moving into migrations for EVM version
		// &EthAccount{},
	)
	registry.RegisterImplementations(
		(*authtypes.GenesisAccount)(nil),
		// TODO: uncomment after moving into migrations for EVM version
		// &EthAccount{},
	)
	registry.RegisterImplementations(
		(*tx.TxExtensionOptionI)(nil),
		&ExtensionOptionsWeb3Tx{},
		&types2.ExtensionOptionDynamicFeeTx{},
	)
}
