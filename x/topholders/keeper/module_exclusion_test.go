package keeper

import (
	"testing"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestIsModuleAddress(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected bool
	}{
		{
			name:     "fee collector should be identified as module",
			address:  authtypes.NewModuleAddress(authtypes.FeeCollectorName).String(),
			expected: true,
		},
		{
			name:     "distribution module should be identified as module",
			address:  authtypes.NewModuleAddress(distrtypes.ModuleName).String(),
			expected: true,
		},
		{
			name:     "governance module should be identified as module",
			address:  authtypes.NewModuleAddress(govtypes.ModuleName).String(),
			expected: true,
		},
		{
			name:     "bonded pool should be identified as module",
			address:  authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String(),
			expected: true,
		},
		{
			name:     "not bonded pool should be identified as module",
			address:  authtypes.NewModuleAddress(stakingtypes.NotBondedPoolName).String(),
			expected: true,
		},
		{
			name:     "mint module should be identified as module",
			address:  authtypes.NewModuleAddress("mint").String(),
			expected: true,
		},
		{
			name:     "IBC transfer module should be identified as module",
			address:  authtypes.NewModuleAddress("transfer").String(),
			expected: true,
		},
		{
			name:     "epixmint module should be identified as module",
			address:  authtypes.NewModuleAddress("epixmint").String(),
			expected: true,
		},
		{
			name:     "EVM module should be identified as module",
			address:  authtypes.NewModuleAddress("evm").String(),
			expected: true,
		},
		{
			name:     "feemarket module should be identified as module",
			address:  authtypes.NewModuleAddress("feemarket").String(),
			expected: true,
		},
		{
			name:     "erc20 module should be identified as module",
			address:  authtypes.NewModuleAddress("erc20").String(),
			expected: true,
		},
		{
			name:     "precisebank module should be identified as module",
			address:  authtypes.NewModuleAddress("precisebank").String(),
			expected: true,
		},
		{
			name:     "regular address should not be identified as module",
			address:  "epix12apeeggrumg9y4gwtez9smh7pjq9vecc4hc936",
			expected: false,
		},
		{
			name:     "empty address should not be identified as module",
			address:  "",
			expected: false,
		},
		{
			name:     "invalid address should not be identified as module",
			address:  "invalid-address",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsModuleAddress(tt.address)
			require.Equal(t, tt.expected, result, "IsModuleAddress(%s) = %v, expected %v", tt.address, result, tt.expected)
		})
	}
}

func TestShouldExcludeAddress(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		expected bool
	}{
		{
			name:     "fee collector should be excluded",
			address:  authtypes.NewModuleAddress(authtypes.FeeCollectorName).String(),
			expected: true,
		},
		{
			name:     "distribution module should be excluded",
			address:  authtypes.NewModuleAddress(distrtypes.ModuleName).String(),
			expected: true,
		},
		{
			name:     "governance module should be excluded",
			address:  authtypes.NewModuleAddress(govtypes.ModuleName).String(),
			expected: true,
		},
		{
			name:     "bonded pool should be excluded",
			address:  authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String(),
			expected: true,
		},
		{
			name:     "not bonded pool should be excluded",
			address:  authtypes.NewModuleAddress(stakingtypes.NotBondedPoolName).String(),
			expected: true,
		},
		{
			name:     "mint module should be excluded",
			address:  authtypes.NewModuleAddress("mint").String(),
			expected: true,
		},
		{
			name:     "IBC transfer module should be excluded",
			address:  authtypes.NewModuleAddress("transfer").String(),
			expected: true,
		},
		{
			name:     "epixmint module should be excluded",
			address:  authtypes.NewModuleAddress("epixmint").String(),
			expected: true,
		},
		{
			name:     "EVM module should be excluded",
			address:  authtypes.NewModuleAddress("evm").String(),
			expected: true,
		},
		{
			name:     "feemarket module should be excluded",
			address:  authtypes.NewModuleAddress("feemarket").String(),
			expected: true,
		},
		{
			name:     "erc20 module should be excluded",
			address:  authtypes.NewModuleAddress("erc20").String(),
			expected: true,
		},
		{
			name:     "precisebank module should be excluded",
			address:  authtypes.NewModuleAddress("precisebank").String(),
			expected: true,
		},
		{
			name:     "regular address should not be excluded",
			address:  "epix12apeeggrumg9y4gwtez9smh7pjq9vecc4hc936",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldExcludeAddress(tt.address)
			require.Equal(t, tt.expected, result, "shouldExcludeAddress(%s) = %v, expected %v", tt.address, result, tt.expected)
		})
	}
}
