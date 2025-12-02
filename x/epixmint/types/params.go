package types

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultParams returns default parameters
func DefaultParams() Params {
	// 10.527B EPIX in aepix (18 decimals): 10.527 * 10^9 * 10^18 = 10.527 * 10^27
	initialAnnualMintAmount, ok := math.NewIntFromString("10527000000000000000000000000")
	if !ok {
		panic("invalid initial annual mint amount")
	}
	// 42B EPIX in aepix (18 decimals): 42 * 10^9 * 10^18 = 42 * 10^27
	maxSupply, ok := math.NewIntFromString("42000000000000000000000000000")
	if !ok {
		panic("invalid max supply")
	}

	return Params{
		MintDenom:               "aepix", // Extended precision denomination
		InitialAnnualMintAmount: initialAnnualMintAmount,
		AnnualReductionRate:     math.LegacyMustNewDecFromStr("0.25"), // 25% annual reduction
		BlockTimeSeconds:        6,                                    // 6 second blocks
		MaxSupply:               maxSupply,
		CommunityPoolRate:       math.LegacyMustNewDecFromStr("0.02"), // 2% to community pool
		StakingRewardsRate:      math.LegacyMustNewDecFromStr("0.98"), // 98% to staking rewards
	}
}

// Validate validates the set of params
func (p Params) Validate() error {
	if err := validateMintDenom(p.MintDenom); err != nil {
		return err
	}
	if err := validateInitialAnnualMintAmount(p.InitialAnnualMintAmount); err != nil {
		return err
	}
	if err := validateAnnualReductionRate(p.AnnualReductionRate); err != nil {
		return err
	}
	if err := validateBlockTimeSeconds(p.BlockTimeSeconds); err != nil {
		return err
	}
	if err := validateMaxSupply(p.MaxSupply); err != nil {
		return err
	}
	if err := validateCommunityPoolRate(p.CommunityPoolRate); err != nil {
		return err
	}
	if err := validateStakingRewardsRate(p.StakingRewardsRate); err != nil {
		return err
	}
	return nil
}

func validateMintDenom(i interface{}) error {
	v, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if err := sdk.ValidateDenom(v); err != nil {
		return err
	}

	return nil
}

func validateInitialAnnualMintAmount(i interface{}) error {
	v, ok := i.(math.Int)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v.IsNil() {
		return fmt.Errorf("initial annual mint amount cannot be nil")
	}

	if v.IsNegative() {
		return fmt.Errorf("initial annual mint amount cannot be negative: %s", v)
	}

	return nil
}

func validateAnnualReductionRate(i interface{}) error {
	v, ok := i.(math.LegacyDec)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v.IsNil() {
		return fmt.Errorf("annual reduction rate cannot be nil")
	}

	if v.IsNegative() {
		return fmt.Errorf("annual reduction rate cannot be negative: %s", v)
	}

	if v.GTE(math.LegacyOneDec()) {
		return fmt.Errorf("annual reduction rate must be less than 1: %s", v)
	}

	return nil
}

func validateBlockTimeSeconds(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v == 0 {
		return fmt.Errorf("block time seconds cannot be zero")
	}

	return nil
}

func validateMaxSupply(i interface{}) error {
	v, ok := i.(math.Int)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v.IsNil() {
		return fmt.Errorf("max supply cannot be nil")
	}

	if v.IsNegative() {
		return fmt.Errorf("max supply cannot be negative: %s", v)
	}

	return nil
}

func validateCommunityPoolRate(i interface{}) error {
	v, ok := i.(math.LegacyDec)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v.IsNil() {
		return fmt.Errorf("community pool rate cannot be nil")
	}

	if v.IsNegative() {
		return fmt.Errorf("community pool rate cannot be negative: %s", v)
	}

	if v.GT(math.LegacyOneDec()) {
		return fmt.Errorf("community pool rate cannot be greater than 1: %s", v)
	}

	return nil
}

func validateStakingRewardsRate(i interface{}) error {
	v, ok := i.(math.LegacyDec)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v.IsNil() {
		return fmt.Errorf("staking rewards rate cannot be nil")
	}

	if v.IsNegative() {
		return fmt.Errorf("staking rewards rate cannot be negative: %s", v)
	}

	if v.GT(math.LegacyOneDec()) {
		return fmt.Errorf("staking rewards rate cannot be greater than 1: %s", v)
	}

	return nil
}

// String implements the Stringer interface.
func (p Params) String() string {
	return fmt.Sprintf(`Mint Params:
  Mint Denom:                    %s
  Initial Annual Mint Amount:    %s
  Annual Reduction Rate:         %s
  Block Time Seconds:            %d
  Max Supply:                    %s
  Community Pool Rate:           %s
  Staking Rewards Rate:          %s`,
		p.MintDenom, p.InitialAnnualMintAmount, p.AnnualReductionRate, p.BlockTimeSeconds,
		p.MaxSupply, p.CommunityPoolRate, p.StakingRewardsRate,
	)
}
