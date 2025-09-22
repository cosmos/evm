package types

import (
	"fmt"

	"github.com/cosmos/evm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
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
	if metadata.Base != evmDenom {
		return nil, fmt.Errorf("metadata base denom (%s) does not match evm_denom (%s)", metadata.Base, evmDenom)
	}
	if len(metadata.DenomUnits) != 2 {
		return nil, fmt.Errorf("metadata must have exactly 2 denom units, got: %d", len(metadata.DenomUnits))
	}

	foundBase := false
	displayDenom := ""
	baseDecimals := Decimals(0)
	baseDecimalsInferred := Decimals(0)
	extendedDecimals := EighteenDecimals

	for _, unit := range metadata.DenomUnits {
		if unit.Denom == metadata.Base && unit.Exponent == 0 {
			if sdk.ValidateDenom(unit.Denom) != nil {
				return nil, fmt.Errorf("invalid base denom: %s", unit.Denom)
			}
			displayDenom = unit.Denom[1:]
			dec, err := DecimalsFromSIPrefix(unit.Denom[:1])
			if err != nil {
				return nil, fmt.Errorf("invalid base denom: %s, %w", unit.Denom, err)
			}
			baseDecimalsInferred = dec
			foundBase = true
			continue
		}

		if sdk.ValidateDenom(unit.Denom) != nil {
			return nil, fmt.Errorf("invalid extended denom: %s", unit.Denom)
		}
		dd := unit.Denom[1:]
		if dd != displayDenom {
			return nil, fmt.Errorf("display denom mismatch: %s != %s", dd, displayDenom)
		}
		decInferred, err := DecimalsFromSIPrefix(unit.Denom[:1])
		if err != nil {
			return nil, fmt.Errorf("invalid extended denom: %s, %w", unit.Denom, err)
		}
		baseDecs := EighteenDecimals - Decimals(unit.Exponent)
		if baseDecs != decInferred {
			return nil, fmt.Errorf("extended decimals mismatch: %s != %s", extendedDecimals, decInferred)
		}
		baseDecimals = decInferred
		if baseDecimalsInferred != baseDecimals {
			return nil, fmt.Errorf("base decimals mismatch: %s != %s", baseDecimalsInferred, baseDecimals)
		}
	}

	if baseDecimals == 0 || extendedDecimals == 0 || displayDenom == "" || !foundBase {
		return nil, fmt.Errorf(
			"invalid base or extended denom: %s, %s, %s, %t",
			baseDecimals, extendedDecimals, displayDenom, foundBase,
		)
	}

	coinInfo := &EvmCoinInfo{
		DisplayDenom:     displayDenom,
		Decimals:         baseDecimals,
		ExtendedDecimals: extendedDecimals,
	}
	if err := coinInfo.Validate(); err != nil {
		return nil, fmt.Errorf("derived coin info is invalid: %w", err)
	}

	return coinInfo, nil
}
