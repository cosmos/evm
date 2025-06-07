package types

import (
	errorsmod "cosmossdk.io/errors"
)

// EVM sentinel callback errors
var (
	ErrInvalidReceiverAddress = errorsmod.Register(ModuleName, 1, "invalid receiver address")
	ErrCallbackFailed         = errorsmod.Register(ModuleName, 2, "callback failed")
	ErrInvalidCalldata        = errorsmod.Register(ModuleName, 3, "invalid calldata in callback data")
	ErrContractHasNoCode      = errorsmod.Register(ModuleName, 4, "contract has no code")
	ErrTokenPairNotFound      = errorsmod.Register(ModuleName, 5, "token not registered")
	ErrNumberOverflow         = errorsmod.Register(ModuleName, 6, "number overflow")
	ErrAllowanceFailed        = errorsmod.Register(ModuleName, 7, "allowance failed")
	ErrEVMCallFailed          = errorsmod.Register(ModuleName, 8, "evm call failed")
	ErrOutOfGas               = errorsmod.Register(ModuleName, 9, "out of gas")
)
