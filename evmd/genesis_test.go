package evmd_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/x/erc20/types"
)

func TestNewEpixErc20GenesisState(t *testing.T) {
	genesisState := evmd.NewEpixErc20GenesisState()

	require.NotNil(t, genesisState)
	require.Len(t, genesisState.TokenPairs, 1, "Should have exactly one token pair (WEPIX)")
	require.Len(t, genesisState.NativePrecompiles, 1, "Should have exactly one native precompile (WEPIX)")

	// Verify the WEPIX token pair
	wepixPair := genesisState.TokenPairs[0]
	require.Equal(t, "aepix", wepixPair.Denom)
	require.Equal(t, "0x211781849EF6de72acbf1469Ce3808E74D7ce158", wepixPair.Erc20Address)
	require.True(t, wepixPair.Enabled)
	require.Equal(t, types.OWNER_MODULE, wepixPair.ContractOwner)

	// Verify the native precompile address matches
	require.Equal(t, wepixPair.Erc20Address, genesisState.NativePrecompiles[0])
}

func TestWEPIXTokenPairConsistency(t *testing.T) {
	// Test that the token pair creation is consistent
	pair1, err := types.NewTokenPairSTRv2("aepix")
	require.NoError(t, err)

	pair2, err := types.NewTokenPairSTRv2("aepix")
	require.NoError(t, err)

	require.Equal(t, pair1.Erc20Address, pair2.Erc20Address, "Token pair creation should be deterministic")
	require.Equal(t, "0x211781849EF6de72acbf1469Ce3808E74D7ce158", pair1.Erc20Address, "Should match expected WEPIX address")
}
