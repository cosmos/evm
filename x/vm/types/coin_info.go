package types

// EvmCoinInfo struct holds the display name and decimal precisions of the EVM denom
type EvmCoinInfo struct {
	// DisplayDenom defines the display denomination shown to users
	DisplayDenom string
	// ExtendedDenom defines the extended EVM denom used in the EVM
	ExtendedDenom string
	// BaseDenom defines the base (or bond) denom used in the chain. May be the same as the ExtendedDenom.
	BaseDenom string
	// Decimals defines the precision/decimals for the base denomination (1-18)
	Decimals Decimals
}

// GetDenom returns the base denomination used in the chain, derived by SI prefix
func (c EvmCoinInfo) GetDenom() string {
	return c.BaseDenom
}

func (c EvmCoinInfo) GetExtendedDenom() string {
	return c.ExtendedDenom
}

func (c EvmCoinInfo) GetDecimals() Decimals {
	return c.Decimals
}

func (c EvmCoinInfo) GetDisplayDenom() string {
	return c.DisplayDenom
}
