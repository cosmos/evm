package types_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/precisebank/types"

	sdkmath "cosmossdk.io/math"
)

func TestConversionFactor_Immutable(t *testing.T) {
	cf1 := testCoinInfo.DecimalsOrDefault().ConversionFactor()
	origInt64 := cf1.Int64()

	// Get the internal pointer to the big.Int without copying
	internalBigInt := cf1.BigIntMut()

	// Mutate the big.Int -- .Add() mutates in place
	internalBigInt.Add(internalBigInt, big.NewInt(5))
	// Ensure bigInt was actually mutated
	require.Equal(t, origInt64+5, internalBigInt.Int64())

	// Fetch the max amount again
	cf2 := testCoinInfo.DecimalsOrDefault().ConversionFactor()

	require.Equal(t, origInt64, cf2.Int64(), "conversion factor should be immutable")
}

func TestConversionFactor_Copied(t *testing.T) {
	max1 := testCoinInfo.DecimalsOrDefault().ConversionFactor().BigIntMut()
	max2 := testCoinInfo.DecimalsOrDefault().ConversionFactor().BigIntMut()

	// Checks that the returned two pointers do not reference the same object
	require.NotSame(t, max1, max2, "max fractional amount should be copied")
}

func TestConversionFactor(t *testing.T) {
	require.Equal(
		t,
		sdkmath.NewInt(1_000_000_000_000),
		testCoinInfo.DecimalsOrDefault().ConversionFactor(),
		"conversion factor should have 12 decimal points",
	)
}

func TestNewFractionalBalance(t *testing.T) {
	tests := []struct {
		name        string
		giveAddress string
		giveAmount  sdkmath.Int
	}{
		{
			"correctly sets fields",
			"cosmos1qperwt9wrnkg5k9e5gzfgjppzpqur82k6c5a0n",
			sdkmath.NewInt(100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := types.NewFractionalBalance(tt.giveAddress, tt.giveAmount)

			require.Equal(t, tt.giveAddress, fb.Address)
			require.Equal(t, tt.giveAmount, fb.Amount)
		})
	}
}

func TestFractionalBalance_Validate(t *testing.T) {
	tests := []struct {
		name        string
		giveAddress string
		giveAmount  sdkmath.Int
		wantErr     string
	}{
		{
			"valid",
			"cosmos1gpxd677pp8zr97xvy3pmgk70a9vcpagsprcjap",
			sdkmath.NewInt(100),
			"",
		},
		{
			"valid - uppercase address",
			"COSMOS1GPXD677PP8ZR97XVY3PMGK70A9VCPAGSPRCJAP",
			sdkmath.NewInt(100),
			"",
		},
		{
			"valid - min balance",
			"cosmos1gpxd677pp8zr97xvy3pmgk70a9vcpagsprcjap",
			sdkmath.NewInt(1),
			"",
		},
		{
			"valid - max balance",
			"cosmos1gpxd677pp8zr97xvy3pmgk70a9vcpagsprcjap",
			testMaxFractionalAmount(),
			"",
		},
		{
			"invalid - 0 balance",
			"cosmos1gpxd677pp8zr97xvy3pmgk70a9vcpagsprcjap",
			sdkmath.NewInt(0),
			"non-positive amount 0",
		},
		{
			"invalid - empty",
			"cosmos1gpxd677pp8zr97xvy3pmgk70a9vcpagsprcjap",
			sdkmath.Int{},
			"nil amount",
		},
		{
			"invalid - mixed case address",
			"cosmos1gpxd677pP8zr97xvy3pmgk70a9vcpagsprcjap",
			sdkmath.NewInt(100),
			"decoding bech32 failed: string not all lowercase or all uppercase",
		},
		{
			"invalid - non-bech32 address",
			"invalid",
			sdkmath.NewInt(100),
			"decoding bech32 failed: invalid bech32 string length 7",
		},
		{
			"invalid - negative amount",
			"cosmos1gpxd677pp8zr97xvy3pmgk70a9vcpagsprcjap",
			sdkmath.NewInt(-100),
			"non-positive amount -100",
		},
		{
			"invalid - max amount + 1",
			"cosmos1gpxd677pp8zr97xvy3pmgk70a9vcpagsprcjap",
			testConversionFactor,
			fmt.Sprintf("amount %s exceeds max of %s", testConversionFactor.String(), testMaxFractionalAmount().String()),
		},
		{
			"invalid - much more than max amount",
			"cosmos1gpxd677pp8zr97xvy3pmgk70a9vcpagsprcjap",
			testConversionFactor.MulRaw(100),
			fmt.Sprintf("amount %s exceeds max of %s", testConversionFactor.MulRaw(100).String(), testMaxFractionalAmount().String()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fb := types.NewFractionalBalance(tt.giveAddress, tt.giveAmount)
			err := fb.Validate(testConversionFactor)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			require.EqualError(t, err, tt.wantErr)
		})
	}
}
