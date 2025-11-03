package types

import (
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SumExtendedCoin returns a sdk.Coin of extended coin denomination with all
// integer and fractional amounts combined. Callers must supply the coin info
// describing the EVM denom configuration.
func SumExtendedCoin(amt sdk.Coins, info evmtypes.EvmCoinInfo) sdk.Coin {
	conversionFactor := info.DecimalsOrDefault().ConversionFactor()
	integerDenom := info.DenomOrDefault()
	extendedDenom := info.ExtendedDenomOrDefault()

	integerAmount := amt.AmountOf(integerDenom).Mul(conversionFactor)
	extendedAmount := amt.AmountOf(extendedDenom)

	fullEmissionAmount := integerAmount.Add(extendedAmount)

	return sdk.NewCoin(
		extendedDenom,
		fullEmissionAmount,
	)
}
