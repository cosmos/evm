package types

import (
	"fmt"

	"github.com/cosmos/evm/utils"
)

// Validate performs a basic validation of a GenesisAccount fields.
func (ga GenesisAccount) Validate() error {
	if err := utils.ValidateAddress(ga.Address); err != nil {
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
