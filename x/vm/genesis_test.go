package vm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/cosmos/evm/x/vm"
	"github.com/cosmos/evm/x/vm/types"
)

func TestDeriveEvmCoinInfoFromBankMetadata(t *testing.T) {
	tests := []struct {
		name         string
		metadata     banktypes.Metadata
		evmDenom     string
		expectError  bool
		expectedInfo types.EvmCoinInfo
	}{
		{
			name: "valid 18-decimal token",
			metadata: banktypes.Metadata{
				Description: "Test token with 18 decimals",
				Base:        "atest",
				Display:     "test",
				DenomUnits: []*banktypes.DenomUnit{
					{Denom: "atest", Exponent: 0},
					{Denom: "test", Exponent: 18},
				},
			},
			evmDenom:    "atest",
			expectError: false,
			expectedInfo: types.EvmCoinInfo{
				DisplayDenom:     "test",
				Decimals:         types.EighteenDecimals,
				ExtendedDecimals: types.EighteenDecimals,
			},
		},
		{
			name: "valid 6-decimal token with atto alias",
			metadata: banktypes.Metadata{
				Description: "Test token with 6 decimals and atto alias",
				Base:        "utest",
				Display:     "test",
				DenomUnits: []*banktypes.DenomUnit{
					{Denom: "utest", Exponent: 0},
					{Denom: "test", Exponent: 6},
					{Denom: "atest", Exponent: 18, Aliases: []string{"attotest"}},
				},
			},
			evmDenom:    "utest",
			expectError: false,
			expectedInfo: types.EvmCoinInfo{
				DisplayDenom:     "test",
				Decimals:         types.SixDecimals,
				ExtendedDecimals: types.EighteenDecimals,
			},
		},
		{
			name: "invalid - no 18-decimal variant",
			metadata: banktypes.Metadata{
				Description: "Test token without 18-decimal variant",
				Base:        "utest",
				Display:     "test",
				DenomUnits: []*banktypes.DenomUnit{
					{Denom: "utest", Exponent: 0},
					{Denom: "test", Exponent: 6},
				},
			},
			evmDenom:    "utest",
			expectError: true,
		},
		{
			name: "invalid - mismatched base denom",
			metadata: banktypes.Metadata{
				Description: "Test token with mismatched base",
				Base:        "utest",
				Display:     "test",
				DenomUnits: []*banktypes.DenomUnit{
					{Denom: "utest", Exponent: 0},
					{Denom: "test", Exponent: 18},
				},
			},
			evmDenom:    "atest", // Different from metadata base
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create VM params with the test evm_denom
			params := types.Params{
				EvmDenom: tt.evmDenom,
			}

			// Create genesis state
			genesisState := types.GenesisState{
				Params: params,
			}

			// Test ValidateGenesisWithBankMetadata
			bankMetadata := []banktypes.Metadata{tt.metadata}
			err := types.ValidateGenesisWithBankMetadata(genesisState, bankMetadata)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateStakingBondDenomWithBankMetadata(t *testing.T) {
	tests := []struct {
		name             string
		stakingBondDenom string
		bankMetadata     []banktypes.Metadata
		expectError      bool
	}{
		{
			name:             "valid staking bond denom with 18-decimal variant",
			stakingBondDenom: "ustake",
			bankMetadata: []banktypes.Metadata{
				{
					Description: "Staking token",
					Base:        "ustake",
					Display:     "stake",
					DenomUnits: []*banktypes.DenomUnit{
						{Denom: "ustake", Exponent: 0},
						{Denom: "stake", Exponent: 6},
						{Denom: "astake", Exponent: 18, Aliases: []string{"attostake"}},
					},
				},
			},
			expectError: false,
		},
		{
			name:             "invalid - no metadata for staking bond denom",
			stakingBondDenom: "ustake",
			bankMetadata:     []banktypes.Metadata{},
			expectError:      true,
		},
		{
			name:             "invalid - no 18-decimal variant",
			stakingBondDenom: "ustake",
			bankMetadata: []banktypes.Metadata{
				{
					Description: "Staking token without 18-decimal variant",
					Base:        "ustake",
					Display:     "stake",
					DenomUnits: []*banktypes.DenomUnit{
						{Denom: "ustake", Exponent: 0},
						{Denom: "stake", Exponent: 6},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := vm.ValidateStakingBondDenomWithBankMetadata(tt.stakingBondDenom, tt.bankMetadata)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
