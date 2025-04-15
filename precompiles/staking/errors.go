package staking

const (
	// ErrDifferentOriginFromDelegator is raised when the origin address is not the same as the delegator address.
	ErrDifferentOriginFromDelegator = "origin address %s is not the same as delegator address %s"
	// ErrDifferentCallerFromDelegator is raised when the caller address is not the same as the delegator address.
	ErrDifferentCallerFromDelegator = "caller address %s is not the same as delegator address %s"
	// ErrNoDelegationFound is raised when no delegation is found for the given delegator and validator addresses.
	ErrNoDelegationFound = "delegation with delegator %s not found for validator %s"
	// ErrDifferentOriginFromValidator is raised when the origin address is not the same as the validator address.
	ErrDifferentOriginFromValidator = "origin address %s is not the same as validator operator address %s"
	// ErrCannotCallFromContract is raised when a function cannot be called from a smart contract.
	ErrCannotCallFromContract = "this method can only be called directly to the precompile, not from a smart contract"
)
