package types

import (
	"fmt"
	"strings"

	"github.com/cosmos/evm/types"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// Validate performs a basic validation of a GenesisAccount fields.
func (ga GenesisAccount) Validate() error {
	if err := types.ValidateAddress(ga.Address); err != nil {
		return err
	}
	return ga.Storage.Validate()
}

// DefaultGenesisState sets default evm genesis state with empty accounts and default params and
// chain config values.
func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Accounts:    []GenesisAccount{},
		Params:      DefaultParams(),
		Preinstalls: []Preinstall{},
	}
}

// NewGenesisState creates a new genesis state.
func NewGenesisState(params Params, accounts []GenesisAccount, preinstalls []Preinstall) *GenesisState {
	return &GenesisState{
		Accounts:    accounts,
		Params:      params,
		Preinstalls: preinstalls,
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	seenAccounts := make(map[string]bool)
	for _, acc := range gs.Accounts {
		if seenAccounts[acc.Address] {
			return fmt.Errorf("duplicated genesis account %s", acc.Address)
		}
		if err := acc.Validate(); err != nil {
			return fmt.Errorf("invalid genesis account %s: %w", acc.Address, err)
		}
		seenAccounts[acc.Address] = true
	}

	// Validate preinstalls
	seenPreinstalls := make(map[string]bool)
	for _, preinstall := range gs.Preinstalls {
		if seenPreinstalls[preinstall.Address] {
			return fmt.Errorf("duplicated preinstall address %s", preinstall.Address)
		}
		if err := preinstall.Validate(); err != nil {
			return fmt.Errorf("invalid preinstall %s: %w", preinstall.Address, err)
		}

		// Check that preinstall address doesn't conflict with any genesis account
		// Both genesis accounts and preinstalls use Ethereum hex addresses
		if seenAccounts[preinstall.Address] {
			return fmt.Errorf("preinstall address %s conflicts with genesis account %s", preinstall.Address, preinstall.Address)
		}

		seenPreinstalls[preinstall.Address] = true
	}

	return gs.Params.Validate()
}

// ValidateGenesisWithBankMetadata validates the EVM genesis state against bank metadata
// to ensure proper configuration for EVM coin derivation. This function should be called
// during genesis validation when both bank and EVM module genesis states are available.
func ValidateGenesisWithBankMetadata(evmGenesis GenesisState, bankMetadata []banktypes.Metadata) error {
	// Basic validation first
	if err := evmGenesis.Validate(); err != nil {
		return err
	}

	// Get the evm_denom from VM params
	evmDenom := evmGenesis.Params.EvmDenom
	if evmDenom == "" {
		return fmt.Errorf("evm_denom parameter is empty")
	}

	// Find the bank metadata for the evm_denom
	var evmMetadata *banktypes.Metadata
	for _, metadata := range bankMetadata {
		if metadata.Base == evmDenom {
			evmMetadata = &metadata
			break
		}
	}

	if evmMetadata == nil {
		return fmt.Errorf("bank metadata not found for evm_denom: %s. "+
			"The bank module genesis must include metadata for the EVM denomination", evmDenom)
	}

	// Validate that the metadata can be used to derive valid coin info
	_, err := DeriveCoinInfoFromMetadata(*evmMetadata, evmDenom)
	if err != nil {
		return fmt.Errorf("invalid bank metadata for evm_denom %s: %w", evmDenom, err)
	}

	return nil
}

// DeriveCoinInfoFromMetadata extracts EvmCoinInfo from bank metadata
func DeriveCoinInfoFromMetadata(metadata banktypes.Metadata, evmDenom string) (*EvmCoinInfo, error) {
	// Validate that the base denom matches the evm_denom
	if metadata.Base != evmDenom {
		return nil, fmt.Errorf("metadata base denom (%s) does not match evm_denom (%s)", metadata.Base, evmDenom)
	}

	// Find the base and display denominations
	var baseDenomUnit *banktypes.DenomUnit
	var displayDenomUnit *banktypes.DenomUnit

	for _, unit := range metadata.DenomUnits {
		if unit.Denom == metadata.Base {
			baseDenomUnit = unit
		}
		if unit.Denom == metadata.Display {
			displayDenomUnit = unit
		}
	}

	if baseDenomUnit == nil {
		return nil, fmt.Errorf("base denom unit not found in metadata for denom: %s", metadata.Base)
	}
	if displayDenomUnit == nil {
		return nil, fmt.Errorf("display denom unit not found in metadata for denom: %s", metadata.Display)
	}

	// Base denom should have exponent 0
	if baseDenomUnit.Exponent != 0 {
		return nil, fmt.Errorf("base denom unit must have exponent 0, got: %d", baseDenomUnit.Exponent)
	}

	// Calculate decimals from display unit exponent
	decimals := Decimals(displayDenomUnit.Exponent)
	if err := decimals.Validate(); err != nil {
		return nil, fmt.Errorf("invalid decimals derived from metadata: %w", err)
	}

	// For the extended decimals, we need to ensure we have an 18-decimal variant
	extendedDecimals := EighteenDecimals

	// If the base decimals are already 18, use them as extended decimals
	if decimals == EighteenDecimals {
		extendedDecimals = decimals
	} else {
		// For non-18 decimal tokens, we require that there is an 18-decimal variant
		// This would typically be handled by the precisebank module
		// Check if there's an atto-variant or 18-decimal alias
		found18DecimalVariant := false
		for _, unit := range metadata.DenomUnits {
			if unit.Exponent == 18 {
				found18DecimalVariant = true
				break
			}
			// Check aliases for 18-decimal variants (like "atto" prefix)
			for _, alias := range unit.Aliases {
				if strings.HasPrefix(alias, "atto") || strings.Contains(alias, "18") {
					found18DecimalVariant = true
					break
				}
			}
			if found18DecimalVariant {
				break
			}
		}

		if !found18DecimalVariant {
			return nil, fmt.Errorf(
				"evm_denom %s requires an 18-decimal variant in bank metadata for EVM compatibility, but none found",
				evmDenom,
			)
		}
	}

	coinInfo := &EvmCoinInfo{
		DisplayDenom:     metadata.Display,
		Decimals:         decimals,
		ExtendedDecimals: extendedDecimals,
	}

	// Validate the derived coin info
	if err := coinInfo.Validate(); err != nil {
		return nil, fmt.Errorf("derived coin info is invalid: %w", err)
	}

	return coinInfo, nil
}
