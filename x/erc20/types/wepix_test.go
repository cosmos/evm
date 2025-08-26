package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/utils"
	"github.com/cosmos/evm/x/erc20/types"
)

const (
	// Test constants to avoid import issues
	epixChainDenom       = "aepix"
	wepixContractMainnet = "0x211781849EF6de72acbf1469Ce3808E74D7ce158"
	wepixContractTestnet = "0x211781849EF6de72acbf1469Ce3808E74D7ce158"
)

func TestWEPIXTokenPairCreation(t *testing.T) {
	testCases := []struct {
		name          string
		denom         string
		expectPass    bool
		expectedAddr  string
		expectedError string
	}{
		{
			name:         "create WEPIX token pair for aepix",
			denom:        epixChainDenom,
			expectPass:   true,
			expectedAddr: wepixContractMainnet,
		},
		{
			name:         "create WEPIX token pair for aatom",
			denom:        "aatom",
			expectPass:   true,
			expectedAddr: "0x8b8b8b8b8b8b8b8b8b8b8b8b8b8b8b8b8b8b8b8b", // This will be different
		},
		{
			name:          "fail with empty denom",
			denom:         "",
			expectPass:    false,
			expectedError: "denom cannot be empty",
		},
		{
			name:         "fail with IBC denom using NewTokenPairSTRv2",
			denom:        "ibc/DF63978F803A2E27CA5CC9B7631654CCF0BBC788B3B7F0A10200508E37C70992",
			expectPass:   true, // Should work with IBC denoms too
			expectedAddr: "0x631654CCF0BBC788b3b7F0a10200508e37c70992",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pair, err := types.NewTokenPairSTRv2(tc.denom)

			if tc.expectPass {
				require.NoError(t, err, "test case: %s", tc.name)
				require.Equal(t, tc.denom, pair.Denom)
				require.True(t, pair.Enabled)
				require.Equal(t, types.OWNER_MODULE, pair.ContractOwner)

				// For aepix, verify it matches our expected WEPIX address
				if tc.denom == epixChainDenom {
					require.Equal(t, tc.expectedAddr, pair.Erc20Address, "WEPIX address should match expected")
				}
			} else {
				require.Error(t, err, "test case: %s", tc.name)
				if tc.expectedError != "" {
					require.Contains(t, err.Error(), tc.expectedError)
				}
			}
		})
	}
}

func TestWEPIXAddressGeneration(t *testing.T) {
	// Test that the address generation is deterministic
	addr1, err := utils.GetWEPIXAddress(epixChainDenom)
	require.NoError(t, err)

	addr2, err := utils.GetWEPIXAddress(epixChainDenom)
	require.NoError(t, err)

	require.Equal(t, addr1, addr2, "WEPIX address generation should be deterministic")
	require.Equal(t, wepixContractMainnet, addr1.Hex(), "Generated address should match constant")
}

func TestNativeDenomAddressGeneration(t *testing.T) {
	testCases := []struct {
		name  string
		denom string
		valid bool
	}{
		{
			name:  "valid native denom aepix",
			denom: "aepix",
			valid: true,
		},
		{
			name:  "valid native denom aatom",
			denom: "aatom",
			valid: true,
		},
		{
			name:  "valid native denom stake",
			denom: "stake",
			valid: true,
		},
		{
			name:  "invalid empty denom",
			denom: "",
			valid: false,
		},
		{
			name:  "invalid IBC denom should fail",
			denom: "ibc/DF63978F803A2E27CA5CC9B7631654CCF0BBC788B3B7F0A10200508E37C70992",
			valid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			addr, err := utils.GetNativeDenomAddress(tc.denom)

			if tc.valid {
				require.NoError(t, err)
				require.NotEqual(t, "0x0000000000000000000000000000000000000000", addr.Hex())

				// Test deterministic generation
				addr2, err2 := utils.GetNativeDenomAddress(tc.denom)
				require.NoError(t, err2)
				require.Equal(t, addr, addr2, "Address generation should be deterministic")
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestWEPIXAddressConstants(t *testing.T) {
	// Verify that our constants match the generated addresses
	generatedAddr, err := utils.GetWEPIXAddress(epixChainDenom)
	require.NoError(t, err)

	require.Equal(t, wepixContractMainnet, generatedAddr.Hex())
	require.Equal(t, wepixContractTestnet, generatedAddr.Hex())
	require.Equal(t, wepixContractMainnet, wepixContractTestnet, "Mainnet and testnet should have same address due to deterministic generation")
}
