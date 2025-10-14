package keeper_test

import (
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/testutil/config"
	"github.com/cosmos/evm/x/topholders/types"
)

func TestHolderInfoValidation(t *testing.T) {
	// Set up SDK config with correct bech32 prefixes
	sdkConfig := sdk.GetConfig()
	config.SetBech32Prefixes(sdkConfig)

	tests := []struct {
		name      string
		holder    types.HolderInfo
		expectErr bool
	}{
		{
			name: "valid holder info",
			holder: types.NewHolderInfo(
				"epix12apeeggrumg9y4gwtez9smh7pjq9vecc4hc936",
				math.NewInt(1000000),
				math.NewInt(500000),
				math.NewInt(200000),
				1,
			),
			expectErr: false,
		},
		{
			name: "empty address",
			holder: types.HolderInfo{
				Address:          "",
				LiquidBalance:    math.NewInt(1000000),
				BondedBalance:    math.NewInt(500000),
				UnbondingBalance: math.NewInt(200000),
				TotalBalance:     math.NewInt(1700000),
				Rank:             1,
			},
			expectErr: true,
		},
		{
			name: "negative liquid balance",
			holder: types.HolderInfo{
				Address:          "epix12apeeggrumg9y4gwtez9smh7pjq9vecc4hc936",
				LiquidBalance:    math.NewInt(-1000000),
				BondedBalance:    math.NewInt(500000),
				UnbondingBalance: math.NewInt(200000),
				TotalBalance:     math.NewInt(-300000),
				Rank:             1,
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.holder.Validate()
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTopHoldersCacheValidation(t *testing.T) {
	// Set up SDK config with correct bech32 prefixes
	sdkConfig := sdk.GetConfig()
	config.SetBech32Prefixes(sdkConfig)

	validHolder := types.NewHolderInfo(
		"epix12apeeggrumg9y4gwtez9smh7pjq9vecc4hc936",
		math.NewInt(1000000),
		math.NewInt(500000),
		math.NewInt(200000),
		1,
	)

	tests := []struct {
		name      string
		cache     types.TopHoldersCache
		expectErr bool
	}{
		{
			name: "valid cache",
			cache: types.NewTopHoldersCache(
				[]types.HolderInfo{validHolder},
				1234567890,
				100,
			),
			expectErr: false,
		},
		{
			name: "empty cache",
			cache: types.NewTopHoldersCache(
				[]types.HolderInfo{},
				1234567890,
				100,
			),
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cache.Validate()
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
