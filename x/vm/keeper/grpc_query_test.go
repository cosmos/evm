package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// buildTraceCtx must zero KV gas configs so simulation matches deliverTx
// (whose ante also zeroes KV via BuildEvmExecutionCtx). SLOAD/SSTORE gas
// accounting then remains opcode-driven and does not consume KV gas, and
// pre-precompile cosmos-sdk store ops (e.g. stateDB.FlushToCacheCtx) are
// free — same as on real chain.
func TestBuildTraceCtxZeroesKVGasConfig(t *testing.T) {
	parent := sdk.Context{}.
		WithKVGasConfig(storetypes.KVGasConfig()).
		WithTransientKVGasConfig(storetypes.TransientGasConfig())
	traceCtx := buildTraceCtx(parent, 1_000_000)
	require.Equal(t, storetypes.GasConfig{}, traceCtx.KVGasConfig())
	require.Equal(t, storetypes.GasConfig{}, traceCtx.TransientKVGasConfig())
}
