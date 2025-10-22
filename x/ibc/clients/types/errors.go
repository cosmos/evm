package types

import (
	errorsmod "cosmossdk.io/errors"
)

// errors
var (
	ErrInvalidPrecompileAddress = errorsmod.Register(ModuleName, 2, "invalid precompile address")
	ErrClientNotFound           = errorsmod.Register(ModuleName, 3, "client not found")
)
