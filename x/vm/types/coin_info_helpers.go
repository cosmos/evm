package types

// DenomOrDefault returns the configured EVM denom if set, otherwise the
// module default denom.
func (info EvmCoinInfo) DenomOrDefault() string {
	if info.Denom == "" {
		return DefaultEVMDenom
	}
	return info.Denom
}

// ExtendedDenomOrDefault returns the configured extended denom if set,
// otherwise it falls back to the primary denom.
func (info EvmCoinInfo) ExtendedDenomOrDefault() string {
	if info.ExtendedDenom != "" {
		return info.ExtendedDenom
	}
	return info.DenomOrDefault()
}

// DecimalsOrDefault returns the configured decimals or the default EVM
// decimals (18) when unset.
func (info EvmCoinInfo) DecimalsOrDefault() Decimals {
	if info.Decimals == 0 {
		return EighteenDecimals
	}
	return Decimals(info.Decimals)
}
