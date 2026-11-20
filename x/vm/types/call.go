package types

import sdk "github.com/cosmos/cosmos-sdk/types"

type CallType int

const (
	// RPC call type is used on requests to eth_estimateGas rpc API endpoint
	RPC CallType = iota + 1
	// Internal call type is used in case of smart contract methods calls
	Internal
)

// MaxPrecompileCalls is the maximum number of precompile
// calls within a transaction. We want to limit this because
// for each precompile tx we're creating a cached context
const MaxPrecompileCalls uint64 = 20

type meteredInternalCallsKey struct{}

// WithMeteredInternalCalls enables measured gas charging for internal EVM calls,
// including failed calls. The caller must provide a bounded gas meter and
// discard any cached state when the enclosing operation fails.
func WithMeteredInternalCalls(ctx sdk.Context) sdk.Context {
	return ctx.WithValue(meteredInternalCallsKey{}, true)
}

// MeteredInternalCalls reports whether the caller opted into measured gas
// charging for its bounded operation.
func MeteredInternalCalls(ctx sdk.Context) bool {
	enabled, _ := ctx.Value(meteredInternalCallsKey{}).(bool)
	return enabled
}
