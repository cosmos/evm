package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type sourceCallbackCtxKey struct{}

// WithSourceCallbackExecution marks a context as executing a source callback.
func WithSourceCallbackExecution(ctx sdk.Context) sdk.Context {
	return ctx.WithValue(sourceCallbackCtxKey{}, true)
}

// IsSourceCallbackExecution returns true when executing inside a source callback.
func IsSourceCallbackExecution(ctx sdk.Context) bool {
	v := ctx.Value(sourceCallbackCtxKey{})
	b, ok := v.(bool)
	return ok && b
}
