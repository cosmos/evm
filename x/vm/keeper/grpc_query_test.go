package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// buildTraceCtx must zero KV gas configs for pure-EVM simulation so SLOAD/SSTORE
// gas accounting remains opcode-driven and does not consume KV gas.
func TestBuildTraceCtxZeroesKVGasConfigWhenNotPreserving(t *testing.T) {
	parent := sdk.Context{}.
		WithKVGasConfig(storetypes.KVGasConfig()).
		WithTransientKVGasConfig(storetypes.TransientGasConfig())
	traceCtx := buildTraceCtx(parent, 1_000_000, false)
	require.Equal(t, storetypes.GasConfig{}, traceCtx.KVGasConfig())
	require.Equal(t, storetypes.GasConfig{}, traceCtx.TransientKVGasConfig())
}

// buildTraceCtx must preserve KV gas configs for native precompile recipients
// so keeper/store operations are charged during estimation and tracing.
func TestBuildTraceCtxPreservesKVGasConfigWhenRequested(t *testing.T) {
	kv := storetypes.KVGasConfig()
	transient := storetypes.TransientGasConfig()
	parent := sdk.Context{}.
		WithKVGasConfig(kv).
		WithTransientKVGasConfig(transient)
	traceCtx := buildTraceCtx(parent, 1_000_000, true)
	require.Equal(t, kv, traceCtx.KVGasConfig())
	require.Equal(t, transient, traceCtx.TransientKVGasConfig())
}
