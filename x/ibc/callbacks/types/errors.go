package types

import (
	errorsmod "cosmossdk.io/errors"
)

// EVM sentinel callback errors
var (
	ErrInvalidReceiverAddress = errorsmod.Register(ModuleName, 1, "invalid receiver address")
	ErrCallbackFailed         = errorsmod.Register(ModuleName, 2, "callback failed")
	ErrInvalidCalldata        = errorsmod.Register(ModuleName, 3, "invalid calldata in callback data")
)
