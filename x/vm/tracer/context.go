package tracer

import sdk "github.com/cosmos/cosmos-sdk/types"

type managerContextKey struct{}

// WithManager stores the manager in an SDK context so nested native EVM calls
// can reuse the enclosing execution's tracing lifecycle.
func WithManager(ctx sdk.Context, manager *Manager) sdk.Context {
	return ctx.WithValue(managerContextKey{}, manager)
}

// WithoutManager returns a context that does not inherit its parent's manager.
func WithoutManager(ctx sdk.Context) sdk.Context {
	return ctx.WithValue(managerContextKey{}, (*Manager)(nil))
}

// FromContext returns the tracing manager associated with an EVM execution.
func FromContext(ctx sdk.Context) (*Manager, bool) {
	manager, ok := ctx.Value(managerContextKey{}).(*Manager)
	return manager, ok && manager != nil
}
