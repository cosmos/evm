package types

import "cosmossdk.io/math"

func CalcGasBaseFee(gasUsed, gasTarget, baseFeeChangeDenom uint64, baseFee, minUnitGas, minGasPrice math.LegacyDec) math.LegacyDec {
	// If the parent gasUsed is the same as the target, the baseFee remains unchanged.
	if gasUsed == gasTarget {
		return baseFee
	}

	if gasTarget == 0 {
		return math.LegacyZeroDec()
	}

	num := math.LegacyNewDecFromInt(math.NewIntFromUint64(gasUsed).Sub(math.NewIntFromUint64(gasTarget)).Abs())
	num = num.Mul(baseFee)
	num = num.QuoInt(math.NewIntFromUint64(gasTarget))
	num = num.QuoInt(math.NewIntFromUint64(baseFeeChangeDenom))

	if gasUsed > gasTarget {
		// If the parent block used more gas than its target, the baseFee should increase.
		// max(1, parentBaseFee * gasUsedDelta / parentGasTarget / baseFeeChangeDenominator)
		baseFeeDelta := math.LegacyMaxDec(num, minUnitGas)
		return baseFee.Add(baseFeeDelta)
	}

	// Otherwise if the parent block used less gas than its target, the baseFee should decrease.
	// max(minGasPrice, parentBaseFee * gasUsedDelta / parentGasTarget / baseFeeChangeDenominator)
	return math.LegacyMaxDec(baseFee.Sub(num), minGasPrice)
}
