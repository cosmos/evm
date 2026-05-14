package keeper

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	vmtypes "github.com/cosmos/evm/x/vm/types"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestBuildTraceCtxEnforcesGasLimit(t *testing.T) {
	const gasLimit = uint64(1_000_000)
	meter := buildTraceCtx(sdk.Context{}, gasLimit, false).GasMeter()
	require.Equal(t, gasLimit, meter.Limit())
	require.NotPanics(t, func() { meter.ConsumeGas(gasLimit-1, "below") })

	defer func() {
		_, isOutOfGas := recover().(storetypes.ErrorOutOfGas)
		require.True(t, isOutOfGas, "expected ErrorOutOfGas past limit")
	}()

	meter.ConsumeGas(2, "exceeds")
}

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

// recoverMeterOOM converts cosmos meter OOM panics to vm.ErrOutOfGas;
// other paths are untouched.
func TestRecoverMeterOOM(t *testing.T) {
	got := func() (err error) {
		defer recoverMeterOOM(&err)
		panic(storetypes.ErrorOutOfGas{Descriptor: "ReadFlat"})
	}()
	require.True(t, errors.Is(got, vm.ErrOutOfGas), "expected vm.ErrOutOfGas, got %v", got)

	got = func() (err error) {
		defer recoverMeterOOM(&err)
		return nil
	}()
	require.NoError(t, got)

	sentinel := errors.New("sentinel")
	got = func() (err error) {
		defer recoverMeterOOM(&err)
		return sentinel
	}()
	require.Same(t, sentinel, got)

	require.PanicsWithValue(t, "boom", func() {
		var err error
		defer recoverMeterOOM(&err)
		panic("boom")
	})
}

// executable's two-defer pattern converts a cosmos meter OOM panic into
// (true, nil, nil) so the binary search treats it as "raise gas".
func TestExecutablePatternConvertsOOMToRaiseGas(t *testing.T) {
	run := func(shouldPanic bool) (vmError bool, rsp *vmtypes.MsgEthereumTxResponse, err error) {
		defer rewriteOutOfGasAsRaiseGas(&vmError, &rsp, &err)
		defer recoverMeterOOM(&err)
		if shouldPanic {
			panic(storetypes.ErrorOutOfGas{Descriptor: "ReadFlat"})
		}
		return false, &vmtypes.MsgEthereumTxResponse{}, nil
	}

	vmError, rsp, err := run(true)
	require.True(t, vmError)
	require.Nil(t, rsp)
	require.NoError(t, err)

	vmError, rsp, err = run(false)
	require.False(t, vmError)
	require.NotNil(t, rsp)
	require.NoError(t, err)
}

func TestRewriteOutOfGasAsRaiseGas(t *testing.T) {
	vmError := false
	rsp := &vmtypes.MsgEthereumTxResponse{}
	err := vm.ErrOutOfGas

	rewriteOutOfGasAsRaiseGas(&vmError, &rsp, &err)

	require.True(t, vmError)
	require.Nil(t, rsp)
	require.NoError(t, err)

	vmError = false
	rsp = &vmtypes.MsgEthereumTxResponse{}
	err = errors.New("other")

	rewriteOutOfGasAsRaiseGas(&vmError, &rsp, &err)

	require.False(t, vmError)
	require.NotNil(t, rsp)
	require.EqualError(t, err, "other")
}

func TestWrapOutOfGasAsInternal(t *testing.T) {
	err := vm.ErrOutOfGas
	wrapOutOfGasAsInternal(&err)
	require.Equal(t, codes.Internal, status.Code(err))
	require.Contains(t, err.Error(), vm.ErrOutOfGas.Error())

	err = errors.New("other")
	wrapOutOfGasAsInternal(&err)
	require.EqualError(t, err, "other")
}
