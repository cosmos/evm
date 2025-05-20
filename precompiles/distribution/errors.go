package distribution

const (
	// ErrDifferentValidator is raised when the origin address is not the same as the validator address.
	ErrDifferentValidator = "origin address %s is not the same as validator address %s"
	// ErrInvalidAmount is raised when the given sdk coins amount is invalid
	ErrInvalidAmount = "invalid amount %s"
)
