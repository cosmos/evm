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
					{Denom: "atest", Exponent: 0},
				},
			},
			evmDenom:    "atest",
			expectError: true,
		},
		{
			name: "valid 6-decimal token",
			metadata: banktypes.Metadata{
				Description: "Test token with 6 decimals",
				Base:        "utest",
				Display:     "test",
				DenomUnits: []*banktypes.DenomUnit{
					{Denom: "utest", Exponent: 0},
					{Denom: "utest", Exponent: 12},
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
			name: "invalid - not exactly 2 denom units",
			metadata: banktypes.Metadata{
				Description: "Test token with wrong number of units",
				Base:        "utest",
				Display:     "test",
				DenomUnits: []*banktypes.DenomUnit{
					{Denom: "utest", Exponent: 0},
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
					{Denom: "utest", Exponent: 12},
				},
			},
			evmDenom:    "mtest",
			expectError: true,
		},
		{
			name: "invalid - display denom mismatch",
			metadata: banktypes.Metadata{
				Description: "Test token with display denom mismatch",
				Base:        "utest",
				Display:     "test",
				DenomUnits: []*banktypes.DenomUnit{
					{Denom: "utest", Exponent: 0},
					{Denom: "uwrong", Exponent: 12},
				},
			},
			evmDenom:    "utest",
			expectError: true,
		},
		{
			name: "invalid - invalid SI prefix",
			metadata: banktypes.Metadata{
				Description: "Test token with invalid SI prefix",
				Base:        "xtest",
				Display:     "test",
				DenomUnits: []*banktypes.DenomUnit{
					{Denom: "xtest", Exponent: 0},
					{Denom: "xtest", Exponent: 12},
				},
			},
			evmDenom:    "xtest",
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
			name:             "valid staking bond denom with 6-decimal",
			stakingBondDenom: "ustake",
			bankMetadata: []banktypes.Metadata{
				{
					Description: "Staking token",
					Base:        "ustake",
					Display:     "stake",
					DenomUnits: []*banktypes.DenomUnit{
						{Denom: "ustake", Exponent: 0},
						{Denom: "ustake", Exponent: 12},
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
			name:             "invalid - invalid metadata structure",
			stakingBondDenom: "ustake",
			bankMetadata: []banktypes.Metadata{
				{
					Description: "Staking token with invalid structure",
					Base:        "ustake",
					Display:     "stake",
					DenomUnits: []*banktypes.DenomUnit{
						{Denom: "ustake", Exponent: 0},
						{Denom: "mstake", Exponent: 3},
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
