package testutil

import (
	"math/big"

	"cosmossdk.io/math"
)

var (
	// ExampleMinGasPrices defines 20B related to atto units as the minimum gas price value on the fee market module.
	// See https://commonwealth.im/evmos/discussion/5073-global-min-gas-price-value-for-cosmos-sdk-and-evm-transaction-choosing-a-value for reference
	ExampleMinGasPrices = big.NewInt(20_000_000_000)

	// ExampleMinGasMultiplier defines the min gas multiplier value on the fee market module.
	// 50% of the leftover gas will be refunded
	ExampleMinGasMultiplier = math.LegacyNewDecWithPrec(5, 1)
)
