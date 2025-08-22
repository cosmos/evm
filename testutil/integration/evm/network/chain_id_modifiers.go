//
// This files contains handler for the testing suite that has to be run to
// modify the chain configuration depending on the chainID

package network

import (
	testconfig "github.com/cosmos/evm/testutil/config"
	testconstants "github.com/cosmos/evm/testutil/constants"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// updateErc20GenesisStateForChainID modify the default genesis state for the
// bank module of the testing suite depending on the chainID.
func updateBankGenesisStateForChainID(bankGenesisState banktypes.GenesisState) banktypes.GenesisState {
	bankGenesisState.DenomMetadata = generateBankGenesisMetadata()

	return bankGenesisState
}

// generateBankGenesisMetadata generates the metadata entries
// for both extended and native EVM denominations depending on the chain.
func generateBankGenesisMetadata() []banktypes.Metadata {
	// Basic denom settings
	displayDenom := evmtypes.GetEVMCoinDisplayDenom() // e.g., "atom"
	evmDenom := evmtypes.GetEVMCoinDenom()            // e.g., "uatom"
	extDenom := types.ExtendedCoinDenom()             // always 18-decimals base denom
	evmDecimals := evmtypes.GetEVMCoinDecimals()      // native decimal precision, e.g., 6, 12, ..., or 18

	// Standard metadata fields
	name := "Cosmos EVM"
	symbol := "ATOM"

	var metas []banktypes.Metadata

	if evmDenom != extDenom {
		// This means we are initializing a chain with non-18 decimals
		//
		// Note: extDenom is always 18-decimals and handled by the precisebank module's states,
		// So we don't need to add it to the bank module's metadata.
		metas = append(metas, banktypes.Metadata{
			Description: "Native EVM denom metadata",
			Base:        evmDenom,
			DenomUnits: []*banktypes.DenomUnit{
				{Denom: evmDenom, Exponent: 0},
				{Denom: displayDenom, Exponent: uint32(evmDecimals)},
			},
			Name:    name,
			Symbol:  symbol,
			Display: displayDenom,
		})
	} else {
		// EVM native chain: single metadata with 18-decimals
		metas = append(metas, banktypes.Metadata{
			Description: "Native 18-decimal denom metadata for Cosmos EVM chain",
			Base:        evmDenom,
			DenomUnits: []*banktypes.DenomUnit{
				{Denom: evmDenom, Exponent: 0},
				{Denom: displayDenom, Exponent: uint32(evmtypes.EighteenDecimals)},
			},
			Name:    name,
			Symbol:  symbol,
			Display: displayDenom,
		})
	}

	return metas
}

// updateErc20GenesisStateForChainID modify the default genesis state for the
// erc20 module on the testing suite depending on the chainID.
func updateErc20GenesisStateForChainID(chainID testconstants.ChainID, erc20GenesisState erc20types.GenesisState) erc20types.GenesisState {
	erc20GenesisState.TokenPairs = updateErc20TokenPairs(chainID, erc20GenesisState.TokenPairs)

	return erc20GenesisState
}

// updateErc20TokenPairs modifies the erc20 token pairs to use the correct
// WEVMOS depending on ChainID
func updateErc20TokenPairs(chainID testconstants.ChainID, tokenPairs []erc20types.TokenPair) []erc20types.TokenPair {
	testnetAddress := GetWEVMOSContractHex(chainID)
	coinInfo := testconfig.CreateEvmCoinInfoFromDynamicConfig(getTestChainConfigForChainID(chainID))

	mainnetAddress := GetWEVMOSContractHex(testconstants.ExampleChainID)

	updatedTokenPairs := make([]erc20types.TokenPair, len(tokenPairs))
	for i, tokenPair := range tokenPairs {
		if tokenPair.Erc20Address == mainnetAddress {
			updatedTokenPairs[i] = erc20types.TokenPair{
				Erc20Address:  testnetAddress,
				Denom:         coinInfo.Denom,
				Enabled:       tokenPair.Enabled,
				ContractOwner: tokenPair.ContractOwner,
			}
		} else {
			updatedTokenPairs[i] = tokenPair
		}
	}
	return updatedTokenPairs
}

// getTestChainConfigForChainID maps a ChainID struct to the appropriate DynamicChainConfig
func getTestChainConfigForChainID(chainID testconstants.ChainID) testconfig.DynamicChainConfig {
	switch chainID {
	case testconstants.ExampleChainID:
		return testconfig.DefaultTestChain
	case testconstants.SixDecimalsChainID:
		return testconfig.SixDecimalsTestChain
	case testconstants.TwelveDecimalsChainID:
		return testconfig.TwelveDecimalsTestChain
	case testconstants.TwoDecimalsChainID:
		return testconfig.TwoDecimalsTestChain
	default:
		// Default fallback to 18 decimals
		return testconfig.DefaultTestChain
	}
}
