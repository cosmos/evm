package types

import (
	"cosmossdk.io/errors"
)

// x/topholders module sentinel errors
var (
	ErrInvalidAddress   = errors.Register(ModuleName, 1, "invalid address")
	ErrInvalidBalance   = errors.Register(ModuleName, 2, "invalid balance")
	ErrTooManyHolders   = errors.Register(ModuleName, 3, "too many holders")
	ErrInvalidRank      = errors.Register(ModuleName, 4, "invalid rank")
	ErrInvalidSorting   = errors.Register(ModuleName, 5, "invalid sorting")
	ErrCacheNotFound    = errors.Register(ModuleName, 6, "cache not found")
	ErrUpdateInProgress = errors.Register(ModuleName, 7, "update in progress")
)
