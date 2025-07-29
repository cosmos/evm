package types

import (
	"fmt"
)

// NewGenesisState creates a new genesis state.
func NewGenesisState(params Params, pairs []TokenPair, allowances []Allowance) GenesisState {
	return GenesisState{
		Params:     params,
		TokenPairs: pairs,
		Allowances: allowances,
	}
}

// DefaultGenesisState sets default evm genesis state with empty accounts and
// default params and chain config values.
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params:     DefaultParams(),
		TokenPairs: []TokenPair{},
		Allowances: []Allowance{},
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
// TODO: Validate that the precompiles have a corresponding token pair
func (gs GenesisState) Validate() error {
	seenErc20 := make(map[string]bool)
	seenDenom := make(map[string]bool)

	for _, b := range gs.TokenPairs {
		if seenErc20[b.Erc20Address] {
			return fmt.Errorf("token ERC20 contract duplicated on genesis '%s'", b.Erc20Address)
		}
		if seenDenom[b.Denom] {
			return fmt.Errorf("coin denomination duplicated on genesis: '%s'", b.Denom)
		}

		if err := b.Validate(); err != nil {
			return err
		}

		seenErc20[b.Erc20Address] = true
		seenDenom[b.Denom] = true
	}

	// Check if active precompiles have a corresponding token pair
	if err := validatePrecompiles(gs.TokenPairs, gs.DynamicPrecompiles); err != nil {
		return fmt.Errorf("invalid dynamic precompiles on genesis: %w", err)
	}

	if err := validatePrecompiles(gs.TokenPairs, gs.NativePrecompiles); err != nil {
		return fmt.Errorf("invalid native precompiles on genesis: %w", err)
	}

	// Check if allowances are valid
	seenAllowance := make(map[string]bool)
	for _, a := range gs.Allowances {
		if seenAllowance[a.Erc20Address+a.Owner+a.Spender] {
			return fmt.Errorf("duplicated allowance on genesis: %s", a.Erc20Address+a.Owner+a.Spender)
		}

		if !seenErc20[a.Erc20Address] {
			return fmt.Errorf("allowance has no corresponding token pair on genesis: %s", a.Erc20Address)
		}

		if err := a.Validate(); err != nil {
			return fmt.Errorf("invalid allowance on genesis: %w", err)
		}

		seenAllowance[a.Erc20Address+a.Owner+a.Spender] = true
	}

	return nil
}

// validatePrecompiles checks if every precompile has a corresponding enabled token pair
func validatePrecompiles(tokenPairs []TokenPair, precompiles []string) error {
	for _, precompile := range precompiles {
		if !hasActiveTokenPair(tokenPairs, precompile) {
			return fmt.Errorf("precompile address '%s' not found in token pairs", precompile)
		}
	}
	return nil
}

func hasActiveTokenPair(pairs []TokenPair, address string) bool {
	for _, p := range pairs {
		if p.Erc20Address == address && p.Enabled {
			return true
		}
	}
	return false
}
