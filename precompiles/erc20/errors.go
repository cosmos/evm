package erc20

// Errors that have formatted information are defined here as a string.
const (
	ErrIntegerOverflow     = "amount %s causes integer overflow"
	ErrNoAllowanceForToken = "allowance for token %s does not exist"
)

// Solidity custom error names from ERC20I (OpenZeppelin IERC20Errors + IPrecompile + precompile-only).
const (
	SolidityErrERC20InsufficientBalance   = "ERC20InsufficientBalance"
	SolidityErrERC20InsufficientAllowance   = "ERC20InsufficientAllowance"
	SolidityErrERC20InvalidSender           = "ERC20InvalidSender"
	SolidityErrERC20InvalidReceiver         = "ERC20InvalidReceiver"
	SolidityErrERC20InvalidApprover         = "ERC20InvalidApprover"
	SolidityErrERC20InvalidSpender          = "ERC20InvalidSpender"
	SolidityErrERC20CannotReceiveFunds      = "ERC20CannotReceiveFunds"
)
