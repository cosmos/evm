package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/log"
	"github.com/holiman/uint256"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ConvertAmountToLegacy18Decimals convert the given amount into a 18 decimals
// representation.
func ConvertAmountTo18DecimalsLegacy(amt sdkmath.LegacyDec, decimals Decimals) sdkmath.LegacyDec {
	return amt.MulInt(decimals.ConversionFactor())
}

// ConvertAmountTo18DecimalsBigInt convert the given amount into a 18 decimals
// representation.
func ConvertAmountTo18DecimalsBigInt(amt *big.Int, decimals Decimals) *big.Int {
	return new(big.Int).Mul(amt, decimals.ConversionFactor().BigInt())
}

// ConvertAmountTo18Decimals256Int convert the given amount into a 18 decimals
// representation.
func ConvertAmountTo18Decimals256Int(amt *uint256.Int, decimals Decimals) *uint256.Int {
	return new(uint256.Int).Mul(amt, uint256.NewInt(decimals.ConversionFactor().Uint64()))
}

// ConvertBigIntFrom18DecimalsToLegacyDec converts the given amount into a LegacyDec
// with the corresponding decimals of the EVM denom.
func ConvertBigIntFrom18DecimalsToLegacyDec(amt *big.Int, decimals Decimals) sdkmath.LegacyDec {
	decAmt := sdkmath.LegacyNewDecFromBigInt(amt)
	return decAmt.QuoInt(decimals.ConversionFactor())
}

// ConvertCoinDenomTo18DecimalsDenom converts the coin's Denom to the extended denom.
func ConvertCoinDenomTo18DecimalsDenom(coin sdk.Coin) (sdk.Coin, error) {
	if err := sdk.ValidateDenom(coin.Denom); err != nil {
		return sdk.Coin{}, err
	}
	displayDenom := coin.Denom[1:]
	extendedDenom := CreateDenomStr(EighteenDecimals, displayDenom)

	// we are just changing the denom without scaling the amount with conversion factor
	return sdk.Coin{
		Denom:  extendedDenom,
		Amount: coin.Amount,
	}, nil
}

// ConvertCoinsDenomTo18DecimalsDenom returns the given coins with the Denom of the evm
// coin converted to the extended denom.
func ConvertCoinsDenomTo18DecimalsDenom(coins sdk.Coins, matchDenom string) sdk.Coins {
	convertedCoins := make(sdk.Coins, len(coins))
	for i, coin := range coins {
		var err error
		if coin.Denom == matchDenom {
			coin, err = ConvertCoinDenomTo18DecimalsDenom(coin)
			if err != nil {
				log.Debug("failed to set denom to 18 decimals, adding zero coin", "error", err)
			}
		}
		convertedCoins[i] = coin
	}
	return convertedCoins.Sort()
}
