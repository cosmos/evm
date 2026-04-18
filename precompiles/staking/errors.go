package staking

const (
	// ErrNoDelegationFound is raised when no delegation is found for the given delegator and validator addresses.
	ErrNoDelegationFound = "delegation with delegator %s not found for validator %s"
	// ErrCannotCallFromContract is raised when a function cannot be called from a smart contract.
	ErrCannotCallFromContract = "this method can only be called directly to the precompile, not from a smart contract"

	// Solidity custom error names defined in StakingI (and IPrecompile where shared).
	SolidityErrBondDenomQueryFailed           = "BondDenomQueryFailed"
	SolidityErrCannotCallFromContract         = "CannotCallFromContract"
	SolidityErrInvalidDescription             = "InvalidDescription"
	SolidityErrInvalidCommission              = "InvalidCommission"
	SolidityErrRedelegationsInputUnpackFailed = "RedelegationsInputUnpackFailed"
	SolidityErrInvalidRedelegationsQuery      = "InvalidRedelegationsQuery"
)
