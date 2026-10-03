package tracer

import (
	"context"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	gethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	// Force-load native tracer engines so callTracer is registered.
	_ "github.com/ethereum/go-ethereum/eth/tracers/native"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type testTracer struct {
	hooks *tracing.Hooks
}

func (t *testTracer) Hooks() *tracing.Hooks { return t.hooks }

func TestManagerDispatchesBaseAndApplicationTracers(t *testing.T) {
	var calls []string
	manager := New(&tracing.Hooks{OnEnter: func(_ int, _ byte, _, _ common.Address, _ []byte, _ uint64, _ *big.Int) {
		calls = append(calls, "base")
	}})
	dispatcher := manager.Hooks()
	manager.InstallApplication(&testTracer{hooks: &tracing.Hooks{OnEnter: func(_ int, _ byte, _, _ common.Address, _ []byte, _ uint64, _ *big.Int) {
		calls = append(calls, "application-1")
	}}})
	manager.InstallApplication(&testTracer{hooks: &tracing.Hooks{OnEnter: func(_ int, _ byte, _, _ common.Address, _ []byte, _ uint64, _ *big.Int) {
		calls = append(calls, "application-2")
	}}})

	require.Same(t, dispatcher, manager.Hooks())
	dispatcher.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	require.Equal(t, []string{"base", "application-1", "application-2"}, calls)
}

func TestManagerUninstallsApplicationTracer(t *testing.T) {
	manager := New(nil)
	dispatcher := manager.Hooks()
	calls := 0
	uninstall := manager.InstallApplication(&testTracer{hooks: &tracing.Hooks{
		OnEnter: func(_ int, _ byte, _, _ common.Address, _ []byte, _ uint64, _ *big.Int) { calls++ },
	}})

	dispatcher.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	require.Equal(t, 1, calls)

	uninstall()
	uninstall()
	require.Same(t, dispatcher, manager.Hooks())
	dispatcher.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	require.Equal(t, 1, calls)
}

func TestManagerNestedHooksOmitTransactionBoundary(t *testing.T) {
	var txStarts, txEnds, enters int
	manager := New(&tracing.Hooks{
		OnTxStart: func(_ *tracing.VMContext, _ *ethtypes.Transaction, _ common.Address) { txStarts++ },
		OnTxEnd:   func(_ *ethtypes.Receipt, _ error) { txEnds++ },
	})
	manager.InstallApplication(&testTracer{hooks: &tracing.Hooks{OnEnter: func(_ int, _ byte, _, _ common.Address, _ []byte, _ uint64, _ *big.Int) {
		enters++
	}}})

	nested := manager.NestedHooks()
	require.Nil(t, nested.OnTxStart)
	require.Nil(t, nested.OnTxEnd)

	manager.Hooks().OnTxStart(nil, nil, common.Address{})
	manager.Hooks().OnTxEnd(nil, nil)
	nested.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	require.Equal(t, 1, txStarts)
	require.Equal(t, 1, txEnds)
	require.Equal(t, 1, enters)
}

func TestManagerNestedHooksAdjustDepthRelativeToParentFrame(t *testing.T) {
	var enters, exits, opcodes, faults []int
	manager := New(&tracing.Hooks{
		OnEnter: func(depth int, _ byte, _, _ common.Address, _ []byte, _ uint64, _ *big.Int) {
			enters = append(enters, depth)
		},
		OnExit: func(depth int, _ []byte, _ uint64, _ error, _ bool) { exits = append(exits, depth) },
		OnOpcode: func(_ uint64, _ byte, _, _ uint64, _ tracing.OpContext, _ []byte, depth int, _ error) {
			opcodes = append(opcodes, depth)
		},
		OnFault: func(_ uint64, _ byte, _, _ uint64, _ tracing.OpContext, depth int, _ error) {
			faults = append(faults, depth)
		},
	})

	root, nested := manager.Hooks(), manager.NestedHooks()
	root.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	root.OnEnter(1, 0, common.Address{}, common.Address{}, nil, 0, nil)
	nested.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	nested.OnOpcode(0, 0, 0, 0, nil, nil, 0, nil)
	nested.OnFault(0, 0, 0, 0, nil, 1, nil)
	nested.OnEnter(1, 0, common.Address{}, common.Address{}, nil, 0, nil)
	nested.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	nested.OnExit(0, nil, 0, nil, false)
	nested.OnExit(1, nil, 0, nil, false)
	nested.OnExit(0, nil, 0, nil, false)
	root.OnExit(1, nil, 0, nil, false)
	root.OnExit(0, nil, 0, nil, false)

	require.Equal(t, []int{0, 1, 2, 3, 4}, enters)
	require.Equal(t, []int{4, 3, 2, 1, 0}, exits)
	require.Equal(t, []int{2}, opcodes)
	require.Equal(t, []int{3}, faults)
}

func TestManagerNestedHooksPreserveCallTracerStack(t *testing.T) {
	callTracer, err := tracers.DefaultDirectory.New("callTracer", &tracers.Context{}, nil, params.MainnetChainConfig)
	require.NoError(t, err)

	manager := New(callTracer.Hooks)
	root, nested := manager.Hooks(), manager.NestedHooks()
	from, to := common.HexToAddress("0x1"), common.HexToAddress("0x2")
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{To: &to, Gas: 100_000})

	root.OnTxStart(nil, tx, from)
	root.OnEnter(0, byte(gethvm.CALL), from, to, nil, 100_000, big.NewInt(0))
	root.OnEnter(1, byte(gethvm.CALL), to, to, nil, 90_000, big.NewInt(0))
	nested.OnEnter(0, byte(gethvm.CALL), to, to, nil, 80_000, big.NewInt(0))
	nested.OnExit(0, nil, 1_000, nil, false)
	root.OnExit(1, nil, 2_000, nil, false)
	root.OnExit(0, nil, 3_000, nil, false)
	root.OnTxEnd(&ethtypes.Receipt{GasUsed: 3_000}, nil)

	_, err = callTracer.GetResult()
	require.NoError(t, err)
}

func TestManagerPreservesEveryHookField(t *testing.T) {
	hooksType := reflect.TypeOf(tracing.Hooks{})
	hooksValue := reflect.New(hooksType).Elem()
	for i := 0; i < hooksType.NumField(); i++ {
		hooksValue.Field(i).Set(reflect.MakeFunc(hooksType.Field(i).Type, func(_ []reflect.Value) []reflect.Value { return nil }))
	}

	dispatcher := reflect.ValueOf(New(hooksValue.Addr().Interface().(*tracing.Hooks)).Hooks()).Elem()
	for i := 0; i < hooksType.NumField(); i++ {
		require.Falsef(t, dispatcher.Field(i).IsNil(), "hook %s was not composed", hooksType.Field(i).Name)
	}
}

func TestManagerContextRoundTrip(t *testing.T) {
	ctx := sdk.Context{}.WithContext(context.Background())
	_, ok := FromContext(ctx)
	require.False(t, ok)

	manager := New(nil)
	ctx = WithManager(ctx, manager)
	actual, ok := FromContext(ctx)
	require.True(t, ok)
	require.Same(t, manager, actual)

	ctx = WithoutManager(ctx)
	_, ok = FromContext(ctx)
	require.False(t, ok)
}
