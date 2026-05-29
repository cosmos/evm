package types

import (
	"fmt"
	"math/big"

	"github.com/holiman/uint256"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ConvertAmountTo18DecimalsLegacy convert the given amount into a 18 decimals
// representation.
func ConvertAmountTo18DecimalsLegacy(amt sdkmath.LegacyDec) sdkmath.LegacyDec {
	evmCoinDecimal := GetEVMCoinDecimals()

	return amt.MulInt(evmCoinDecimal.ConversionFactor())
}

// ConvertAmountTo18DecimalsBigInt convert the given amount into a 18 decimals
// representation.
func ConvertAmountTo18DecimalsBigInt(amt *big.Int) *big.Int {
	evmCoinDecimal := GetEVMCoinDecimals()

	return new(big.Int).Mul(amt, evmCoinDecimal.ConversionFactor().BigInt())
}

// ConvertAmountTo18Decimals256Int convert the given amount into a 18 decimals
// representation.
func ConvertAmountTo18Decimals256Int(amt *uint256.Int) *uint256.Int {
	evmCoinDecimal := GetEVMCoinDecimals()

	return new(uint256.Int).Mul(amt, uint256.NewInt(evmCoinDecimal.ConversionFactor().Uint64()))
}

// ConvertBigIntFrom18DecimalsToLegacyDec converts the given amount into a LegacyDec
// with the corresponding decimals of the EVM denom.
func ConvertBigIntFrom18DecimalsToLegacyDec(amt *big.Int) sdkmath.LegacyDec {
	evmCoinDecimal := GetEVMCoinDecimals()
	decAmt := sdkmath.LegacyNewDecFromBigInt(amt)
	return decAmt.QuoInt(evmCoinDecimal.ConversionFactor())
}

// ConvertEvmCoinDenomToExtendedDenom converts the coin's Denom to the extended denom.
// Return an error if the coin denom is not the EVM.
func ConvertEvmCoinDenomToExtendedDenom(coin sdk.Coin) (sdk.Coin, error) {
	if coin.Denom != GetEVMCoinDenom() {
		return sdk.Coin{}, fmt.Errorf("expected coin denom %s, received %s", GetEVMCoinDenom(), coin.Denom)
	}

	return sdk.Coin{Denom: GetEVMCoinExtendedDenom(), Amount: coin.Amount}, nil
}

// ConvertCoinsDenomToExtendedDenom returns the given coins with the Denom of the evm
// coin converted to the extended denom.
func ConvertCoinsDenomToExtendedDenom(coins sdk.Coins) sdk.Coins {
	evmDenom := GetEVMCoinDenom()
	convertedCoins := make(sdk.Coins, len(coins))
	for i, coin := range coins {
		if coin.Denom == evmDenom {
			coin, _ = ConvertEvmCoinDenomToExtendedDenom(coin)
		}
		convertedCoins[i] = coin
	}
	return convertedCoins.Sort()
}

// ConvertCoinsDenomToExtendedDenomWithEvmParams returns the given coins with the Denom of the evm
// coin converted to the extended denom using params.
func ConvertCoinsDenomToExtendedDenomWithEvmParams(coins sdk.Coins, params Params) sdk.Coins {
	convertedCoins := make(sdk.Coins, len(coins))
	for i, coin := range coins {
		if coin.Denom == params.EvmDenom {
			coin = sdk.Coin{Denom: params.ExtendedDenomOptions.ExtendedDenom, Amount: coin.Amount}
		}
		convertedCoins[i] = coin
	}
	return convertedCoins.Sort()
}
