package keeper_test

import (
	"fmt"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/testutil/config"
	"github.com/cosmos/evm/x/topholders/types"
)

func TestNewFeatures(t *testing.T) {
	// Set up SDK config with correct bech32 prefixes
	sdkConfig := sdk.GetConfig()
	config.SetBech32Prefixes(sdkConfig)

	// Test 1: Create holder with bonded and unbonding balances
	holder := types.NewHolderInfo(
		"epix12apeeggrumg9y4gwtez9smh7pjq9vecc4hc936",
		math.NewInt(1000000), // liquid: 1 EPIX
		math.NewInt(5000000), // bonded: 5 EPIX
		math.NewInt(2000000), // unbonding: 2 EPIX
		1,
	)

	require.NoError(t, holder.Validate())
	require.Equal(t, math.NewInt(8000000), holder.TotalBalance) // 1+5+2 = 8 EPIX
	require.Equal(t, uint32(1), holder.Rank)

	fmt.Printf("Holder: %s\n", holder.Address)
	fmt.Printf("  Liquid: %s\n", holder.LiquidBalance.String())
	fmt.Printf("  Bonded: %s\n", holder.BondedBalance.String())
	fmt.Printf("  Unbonding: %s\n", holder.UnbondingBalance.String())
	fmt.Printf("  Total: %s\n", holder.TotalBalance.String())
	fmt.Printf("  Rank: %d\n", holder.Rank)

	// Test 2: Create holder with module tag
	moduleHolder := types.NewHolderInfoWithTag(
		"epix1jv65s3grqf6v6jl3dp4t6c9t9rk99cd8j52fwy", // distribution module address
		math.NewInt(10000000),                         // 10 EPIX
		math.ZeroInt(),
		math.ZeroInt(),
		2,
		"Distribution",
	)

	require.NoError(t, moduleHolder.Validate())
	require.Equal(t, "Distribution", moduleHolder.ModuleTag)

	fmt.Printf("\nModule Holder: %s (%s)\n", moduleHolder.Address, moduleHolder.ModuleTag)
	fmt.Printf("  Liquid: %s\n", moduleHolder.LiquidBalance.String())
	fmt.Printf("  Total: %s\n", moduleHolder.TotalBalance.String())

	// Test 3: Validate 1000 holder limit with pagination
	request := &types.QueryTopHoldersRequest{
		Pagination: &query.PageRequest{Limit: 1000},
	}
	require.NoError(t, request.Validate())

	// Test 4: Validate that over 1000 fails
	requestTooMany := &types.QueryTopHoldersRequest{
		Pagination: &query.PageRequest{Limit: 1001},
	}
	require.Error(t, requestTooMany.Validate())

	fmt.Printf("\nValidation tests passed!\n")
	fmt.Printf("- Max holders supported: 1000\n")
	fmt.Printf("- New balance types: liquid, bonded, unbonding\n")
	fmt.Printf("- Module address tagging: supported\n")
}
