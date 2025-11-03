package types_test

import (
	testconstants "github.com/cosmos/evm/testutil/constants"

	sdkmath "cosmossdk.io/math"
)

var (
	testCoinInfo         = testconstants.ChainsCoinInfo[testconstants.TwelveDecimalsChainID.EVMChainID]
	testConversionFactor = testCoinInfo.DecimalsOrDefault().ConversionFactor()
	testIntegerDenom     = testCoinInfo.DenomOrDefault()
	testExtendedDenom    = testCoinInfo.ExtendedDenomOrDefault()
)

func testMaxFractionalAmount() sdkmath.Int {
	return testConversionFactor.SubRaw(1)
}
