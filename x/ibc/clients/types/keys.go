package types

import "cosmossdk.io/collections"

const ModuleName = "evmclients"

var (
	ParamsKey            = collections.NewPrefix(0)
	ClientPrecompilesKey = collections.NewPrefix(1)
	PrecompilesKey       = collections.NewPrefix(2)
)
