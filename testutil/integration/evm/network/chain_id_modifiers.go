//
// This files contains handler for the testing suite that has to be run to
// modify the chain configuration depending on the chainID

package network

import (
	testconfig "github.com/cosmos/evm/testutil/config"
	erc20types "github.com/cosmos/evm/x/erc20/types"
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
	coinInfo := testconfig.DefaultChainConfig.EvmConfig.CoinInfo

	// Standard metadata fields
	name := "Cosmos EVM"
	symbol := "ATOM"

	var metas []banktypes.Metadata

	if coinInfo.Decimals != coinInfo.ExtendedDecimals {
		// This means we are initializing a chain with non-18 decimals
		//
		// Note: extDenom is always 18-decimals and handled by the precisebank module's states,
		// So we don't need to add it to the bank module's metadata.
		metas = append(metas, banktypes.Metadata{
			Description: "Native EVM denom metadata",
			Base:        coinInfo.GetDenom(),
			DenomUnits: []*banktypes.DenomUnit{
				{Denom: coinInfo.GetDenom(), Exponent: 0},
				{Denom: coinInfo.DisplayDenom, Exponent: uint32(coinInfo.Decimals)},
			},
			Name:    name,
			Symbol:  symbol,
			Display: coinInfo.DisplayDenom,
		})
	} else {
		// EVM native chain: single metadata with 18-decimals
		metas = append(metas, banktypes.Metadata{
			Description: "Native 18-decimal denom metadata for Cosmos EVM chain",
			Base:        coinInfo.GetDenom(),
			DenomUnits: []*banktypes.DenomUnit{
				{Denom: coinInfo.GetDenom(), Exponent: 0},
				{Denom: coinInfo.DisplayDenom, Exponent: uint32(evmtypes.EighteenDecimals)},
			},
			Name:    name,
			Symbol:  symbol,
			Display: coinInfo.DisplayDenom,
		})
	}

	return metas
}

// updateErc20GenesisStateForCoinInfo modify the default genesis state for the
// erc20 module on the testing suite depending on the coin info
func updateErc20GenesisStateForCoinInfo(coinInfo evmtypes.EvmCoinInfo, erc20GenesisState erc20types.GenesisState) erc20types.GenesisState {
	erc20GenesisState.TokenPairs = updateErc20TokenPairs(coinInfo, erc20GenesisState.TokenPairs)

	return erc20GenesisState
}

// updateErc20TokenPairs modifies the erc20 token pairs to use the correct
// WEVMOS depending on the coin info
func updateErc20TokenPairs(coinInfo evmtypes.EvmCoinInfo, tokenPairs []erc20types.TokenPair) []erc20types.TokenPair {
	updatedTokenPairs := make([]erc20types.TokenPair, len(tokenPairs))
	for i, tokenPair := range tokenPairs {
		if tokenPair.Erc20Address == testconfig.DefaultWevmosContractMainnet {
			updatedTokenPairs[i] = erc20types.TokenPair{
				Erc20Address:  testconfig.DefaultWevmosContractTestnet,
				Denom:         coinInfo.GetDenom(),
				Enabled:       tokenPair.Enabled,
				ContractOwner: tokenPair.ContractOwner,
			}
		} else {
			updatedTokenPairs[i] = tokenPair
		}
	}
	return updatedTokenPairs
}
