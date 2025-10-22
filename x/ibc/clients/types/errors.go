package types

import (
	errorsmod "cosmossdk.io/errors"
)

// errors
var (
	ErrInvalidPrecompileAddress = errorsmod.Register(ModuleName, 2, "invalid precompile address")
)
