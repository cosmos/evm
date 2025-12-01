package config

import (
	"encoding/json"

	"cosmossdk.io/math"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	testconstants "github.com/cosmos/evm/testutil/constants"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// GenesisState of the blockchain is represented here as a map of raw json
// messages key'd by an identifier string.
// The identifier is used to determine which module genesis information belongs
// to so it may be appropriately routed during init chain.
// Within this application default genesis information is retrieved from
// the ModuleBasicManager which populates json from each BasicModule
// object provided to it during init.
type GenesisState map[string]json.RawMessage

// NewEVMGenesisState returns the default genesis state for the EVM module.
//
// NOTE: for the example chain implementation we need to set the default EVM denomination
// and enable ALL precompiles.
func NewEVMGenesisState() *evmtypes.GenesisState {
	evmGenState := evmtypes.DefaultGenesisState()
	evmGenState.Params.ActiveStaticPrecompiles = evmtypes.AvailableStaticPrecompiles
	evmGenState.Params.EvmDenom = testconstants.ExampleAttoDenom
	evmGenState.Params.ExtendedDenomOptions = &evmtypes.ExtendedDenomOptions{ExtendedDenom: testconstants.
		ExampleAttoDenom}

	return evmGenState
}

// NewErc20GenesisState returns the default genesis state for the ERC20 module.
//
// NOTE: for the example chain implementation we are also adding a default token pair,
// which is the base denomination of the chain (i.e. the WEVMOS contract).
func NewErc20GenesisState() *erc20types.GenesisState {
	erc20GenState := erc20types.DefaultGenesisState()
	erc20GenState.TokenPairs = testconstants.ExampleTokenPairs
	erc20GenState.NativePrecompiles = []string{testconstants.WEVMOSContractMainnet}

	return erc20GenState
}
// NewMintGenesisState returns the default genesis state for the mint module.
//
// NOTE: for the Epix chain implementation we are setting up the minting parameters
// for the initial inflation rate of 10.527 billion EPIX per year.
func NewMintGenesisState() *minttypes.GenesisState {
	mintGenState := minttypes.DefaultGenesisState()
	mintGenState.Params.MintDenom = testconstants.ExampleAttoDenom

	// Set Epix-specific minting parameters
	// Initial inflation: 10.527 billion EPIX per year / 42 billion max supply = ~25.06%
	mintGenState.Params.InflationRateChange = math.LegacyMustNewDecFromStr("0.130000000000000000") // 13% max annual change
	mintGenState.Params.InflationMax = math.LegacyMustNewDecFromStr("1.000000000000000000")        // 100% max (42B max supply)
	mintGenState.Params.InflationMin = math.LegacyMustNewDecFromStr("0.070000000000000000")        // 7% minimum
	mintGenState.Params.GoalBonded = math.LegacyMustNewDecFromStr("0.670000000000000000")          // 67% bonding goal
	mintGenState.Params.BlocksPerYear = 5256000                                                    // ~6 second blocks

	// Set initial inflation rate
	mintGenState.Minter.Inflation = math.LegacyMustNewDecFromStr("0.250642857142857000") // Initial rate

	return mintGenState
}

// NewFeeMarketGenesisState returns the default genesis state for the feemarket module.
//
// NOTE: Enabling base fee for proper EIP-1559 support with wallets like Keplr.
func NewFeeMarketGenesisState() *feemarkettypes.GenesisState {
	feeMarketGenState := feemarkettypes.DefaultGenesisState()
	feeMarketGenState.Params.NoBaseFee = false
	// Set a reasonable initial base fee (1 billion wei = 1 gwei equivalent)
	feeMarketGenState.Params.BaseFee = math.LegacyNewDec(1_000_000_000)
	// Enable from genesis
	feeMarketGenState.Params.EnableHeight = 0

	return feeMarketGenState
}
