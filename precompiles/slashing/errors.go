package slashing

import (
	cmn "github.com/cosmos/evm/precompiles/common"

	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
)

const (
	// SolidityErrSlashingInputInvalid is defined in ISlashing.sol.
	SolidityErrSlashingInputInvalid = "SlashingInputInvalid"

	SolidityErrSlashingNoValidatorForAddress        = "SlashingNoValidatorForAddress"
	SolidityErrSlashingMissingSelfDelegation        = "SlashingMissingSelfDelegation"
	SolidityErrSlashingSelfDelegationTooLowToUnjail = "SlashingSelfDelegationTooLowToUnjail"
	SolidityErrSlashingValidatorNotJailed           = "SlashingValidatorNotJailed"
	SolidityErrSlashingValidatorJailed              = "SlashingValidatorJailed"
)

var slashingErrorMappings = cmn.CosmosErrorMappings{
	cmn.NewCosmosErrorMapping(slashingtypes.ErrNoValidatorForAddress, SolidityErrSlashingNoValidatorForAddress),
	cmn.NewCosmosErrorMapping(slashingtypes.ErrMissingSelfDelegation, SolidityErrSlashingMissingSelfDelegation),
	cmn.NewCosmosErrorMapping(slashingtypes.ErrSelfDelegationTooLowToUnjail, SolidityErrSlashingSelfDelegationTooLowToUnjail),
	cmn.NewCosmosErrorMapping(slashingtypes.ErrValidatorNotJailed, SolidityErrSlashingValidatorNotJailed),
	cmn.NewCosmosErrorMapping(slashingtypes.ErrValidatorJailed, SolidityErrSlashingValidatorJailed),
}

func ErrorMappings() cmn.CosmosErrorMappings {
	return slashingErrorMappings.Clone()
}
