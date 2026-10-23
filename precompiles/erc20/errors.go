package erc20

import (
	cmn "github.com/cosmos/evm/precompiles/common"
	erc20types "github.com/cosmos/evm/x/erc20/types"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// Errors that have formatted information are defined here as a string.
const (
	ErrIntegerOverflow     = "amount %s causes integer overflow"
	ErrNoAllowanceForToken = "allowance for token %s does not exist"
)

// Solidity custom error names from ERC20I (OpenZeppelin IERC20Errors + IPrecompile + precompile-only).
const (
	SolidityErrERC20InsufficientBalance   = "ERC20InsufficientBalance"
	SolidityErrERC20InsufficientAllowance = "ERC20InsufficientAllowance"
	SolidityErrERC20InvalidSender         = "ERC20InvalidSender"
	SolidityErrERC20InvalidReceiver       = "ERC20InvalidReceiver"
	SolidityErrERC20InvalidApprover       = "ERC20InvalidApprover"
	SolidityErrERC20InvalidSpender        = "ERC20InvalidSpender"
	SolidityErrERC20CannotReceiveFunds    = "ERC20CannotReceiveFunds"
	SolidityErrBankSendDisabled           = "BankSendDisabled"
	SolidityErrERC20TokenPairNotFound     = "ERC20TokenPairNotFound" //nolint:gosec // Solidity custom-error name, not a credential.
	SolidityErrERC20TokenPairDisabled     = "ERC20TokenPairDisabled" //nolint:gosec // Solidity custom-error name, not a credential.
)

var erc20ErrorMappings = cmn.CosmosErrorMappings{
	cmn.NewCosmosErrorMapping(banktypes.ErrSendDisabled, SolidityErrBankSendDisabled),
	cmn.NewCosmosErrorMapping(erc20types.ErrTokenPairNotFound, SolidityErrERC20TokenPairNotFound),
	cmn.NewCosmosErrorMapping(erc20types.ErrERC20TokenPairDisabled, SolidityErrERC20TokenPairDisabled),
}

func ErrorMappings() cmn.CosmosErrorMappings {
	return erc20ErrorMappings.Clone()
}
