package tracer

import (
	"context"
	"math/big"
	"reflect"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	gethvm "github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
	_ "github.com/ethereum/go-ethereum/eth/tracers/native"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

type testTracer struct {
	hooks *tracing.Hooks
}

func (t *testTracer) Hooks() *tracing.Hooks {
	return t.hooks
}

func newTestTracer(hooks *tracing.Hooks) *testTracer {
	return &testTracer{hooks: hooks}
}

func TestManagerDispatchesScopesAndUninstalls(t *testing.T) {
	var calls []string
	base := &tracing.Hooks{
		OnEnter: func(_ int, _ byte, _, _ common.Address, _ []byte, _ uint64, _ *big.Int) {
			calls = append(calls, "base")
		},
	}
	manager := New(base)
	dispatcher := manager.Hooks()

	uninstallGlobal := manager.InstallApplication(newTestTracer(&tracing.Hooks{
		OnEnter: func(_ int, _ byte, _, _ common.Address, _ []byte, _ uint64, _ *big.Int) {
			calls = append(calls, "application-1")
		},
	}))
	uninstallSecond := manager.InstallApplication(newTestTracer(&tracing.Hooks{
		OnEnter: func(_ int, _ byte, _, _ common.Address, _ []byte, _ uint64, _ *big.Int) {
			calls = append(calls, "application")
		},
	}))

	require.Same(t, dispatcher, manager.Hooks())
	dispatcher.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	require.Equal(t, []string{"base", "application-1", "application"}, calls)

	calls = nil
	uninstallSecond()
	uninstallSecond()
	dispatcher.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	require.Equal(t, []string{"base", "application-1"}, calls)

	calls = nil
	uninstallGlobal()
	dispatcher.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	require.Equal(t, []string{"base"}, calls)
}

func TestManagerUpdatesOnlySubscribedCallbacks(t *testing.T) {
	manager := New(&tracing.Hooks{
		OnTxStart: func(_ *tracing.VMContext, _ *ethtypes.Transaction, _ common.Address) {},
	})
	dispatcher := manager.Hooks()
	require.NotNil(t, dispatcher.OnTxStart)
	require.Nil(t, dispatcher.OnOpcode)

	opcodes := 0
	uninstall := manager.InstallApplication(newTestTracer(&tracing.Hooks{
		OnOpcode: func(_ uint64, _ byte, _, _ uint64, _ tracing.OpContext, _ []byte, _ int, _ error) {
			opcodes++
		},
	}))
	require.Same(t, dispatcher, manager.Hooks())
	require.NotNil(t, dispatcher.OnOpcode)
	dispatcher.OnOpcode(0, 0, 0, 0, nil, nil, 0, nil)
	require.Equal(t, 1, opcodes)

	uninstall()
	require.Same(t, dispatcher, manager.Hooks())
	require.Nil(t, dispatcher.OnOpcode)
}

func TestManagerNestedHooksOmitTransactionBoundary(t *testing.T) {
	txStarts := 0
	txEnds := 0
	enters := 0
	manager := New(&tracing.Hooks{
		OnTxStart: func(_ *tracing.VMContext, _ *ethtypes.Transaction, _ common.Address) {
			txStarts++
		},
		OnTxEnd: func(_ *ethtypes.Receipt, _ error) {
			txEnds++
		},
	})
	nested := manager.NestedHooks()

	require.NotNil(t, manager.Hooks().OnTxStart)
	require.NotNil(t, manager.Hooks().OnTxEnd)
	require.Nil(t, nested.OnTxStart)
	require.Nil(t, nested.OnTxEnd)
	require.NotNil(t, nested.OnEnter)

	_ = manager.InstallApplication(newTestTracer(&tracing.Hooks{
		OnEnter: func(_ int, _ byte, _, _ common.Address, _ []byte, _ uint64, _ *big.Int) {
			enters++
		},
	}))
	require.Same(t, nested, manager.NestedHooks())
	require.NotNil(t, nested.OnEnter)

	manager.Hooks().OnTxStart(nil, nil, common.Address{})
	manager.Hooks().OnTxEnd(nil, nil)
	nested.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	require.Equal(t, 1, txStarts)
	require.Equal(t, 1, txEnds)
	require.Equal(t, 1, enters)
}

func TestManagerHasActiveCall(t *testing.T) {
	manager := New(nil)
	require.False(t, manager.HasActiveCall())

	manager.Hooks().OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
	require.True(t, manager.HasActiveCall())
	manager.Hooks().OnExit(0, nil, 0, nil, false)
	require.False(t, manager.HasActiveCall())
}

func TestManagerRemainsActiveAfterSameDepthChildReturns(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "success"},
		{name: "revert", err: gethvm.ErrExecutionReverted},
		{name: "out of gas", err: gethvm.ErrOutOfGas},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manager := New(nil)
			hooks := manager.Hooks()
			hooks.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
			hooks.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
			hooks.OnExit(0, nil, 0, tc.err, tc.err != nil)
			require.True(t, manager.HasActiveCall(), "the parent precompile is still running")
			hooks.OnExit(0, nil, 0, nil, false)
			require.False(t, manager.HasActiveCall())
		})
	}
}

func TestManagerRestoresDepthAfterReentrantCall(t *testing.T) {
	for _, tc := range []struct {
		name     string
		nestedVM bool
	}{
		{name: "same VM at nonzero depth"},
		{name: "nested VM at zero depth", nestedVM: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var enters, exits, opcodes, faults, applicationOpcodes []int
			manager := New(&tracing.Hooks{
				OnEnter: func(depth int, _ byte, _, _ common.Address, _ []byte, _ uint64, _ *big.Int) {
					enters = append(enters, depth)
				},
				OnExit: func(depth int, _ []byte, _ uint64, _ error, _ bool) {
					exits = append(exits, depth)
				},
				OnOpcode: func(_ uint64, _ byte, _, _ uint64, _ tracing.OpContext, _ []byte, depth int, _ error) {
					opcodes = append(opcodes, depth)
				},
				OnFault: func(_ uint64, _ byte, _, _ uint64, _ tracing.OpContext, depth int, _ error) {
					faults = append(faults, depth)
				},
			})
			root := manager.Hooks()
			root.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
			root.OnEnter(1, 0, common.Address{}, common.Address{}, nil, 0, nil)
			dispatcher, rawDepth, logicalDepth := root, 1, 1
			if tc.nestedVM {
				dispatcher, rawDepth, logicalDepth = manager.NestedHooks(), 0, 2
				dispatcher.OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
			}

			// A precompile reenters its VM at the same reported depth. Installing
			// and removing a application rebuilds the dispatcher while this call is open.
			dispatcher.OnEnter(rawDepth, 0, common.Address{}, common.Address{}, nil, 0, nil)
			uninstall := manager.InstallApplication(newTestTracer(&tracing.Hooks{
				OnOpcode: func(_ uint64, _ byte, _, _ uint64, _ tracing.OpContext, _ []byte, depth int, _ error) {
					applicationOpcodes = append(applicationOpcodes, depth)
				},
			}))
			dispatcher.OnOpcode(0, 0, 0, 0, nil, nil, rawDepth+1, nil)
			dispatcher.OnFault(0, 0, 0, 0, nil, rawDepth+1, gethvm.ErrInvalidJump)
			uninstall()
			dispatcher.OnExit(rawDepth, nil, 0, gethvm.ErrInvalidJump, true)
			require.True(t, manager.HasActiveCall())
			dispatcher.OnOpcode(0, 0, 0, 0, nil, nil, rawDepth+1, nil)

			// Starting another VM after that child failed must use the restored
			// parent call, with no residual offset from the previous call.
			manager.NestedHooks().OnEnter(0, 0, common.Address{}, common.Address{}, nil, 0, nil)
			manager.NestedHooks().OnOpcode(0, 0, 0, 0, nil, nil, 1, nil)
			manager.NestedHooks().OnExit(0, nil, 0, nil, false)
			if tc.nestedVM {
				dispatcher.OnExit(0, nil, 0, nil, false)
			}
			root.OnExit(1, nil, 0, nil, false)
			root.OnExit(0, nil, 0, nil, false)
			require.False(t, manager.HasActiveCall())
			require.Equal(t, []int{logicalDepth + 2, logicalDepth + 1, logicalDepth + 2}, opcodes)
			require.Equal(t, []int{logicalDepth + 2}, faults)
			require.Equal(t, []int{logicalDepth + 2}, applicationOpcodes)
			if tc.nestedVM {
				require.Equal(t, []int{0, 1, 2, 3, 3}, enters)
				require.Equal(t, []int{3, 3, 2, 1, 0}, exits)
			} else {
				require.Equal(t, []int{0, 1, 2, 2}, enters)
				require.Equal(t, []int{2, 2, 1, 0}, exits)
			}
		})
	}
}

func TestManagerNestedHooksAdjustDepthRelativeToParentCall(t *testing.T) {
	var enters, exits, opcodes, faults []int
	manager := New(&tracing.Hooks{
		OnEnter: func(depth int, _ byte, _, _ common.Address, _ []byte, _ uint64, _ *big.Int) {
			enters = append(enters, depth)
		},
		OnExit: func(depth int, _ []byte, _ uint64, _ error, _ bool) {
			exits = append(exits, depth)
		},
		OnOpcode: func(_ uint64, _ byte, _, _ uint64, _ tracing.OpContext, _ []byte, depth int, _ error) {
			opcodes = append(opcodes, depth)
		},
		OnFault: func(_ uint64, _ byte, _, _ uint64, _ tracing.OpContext, depth int, _ error) {
			faults = append(faults, depth)
		},
	})

	root := manager.Hooks()
	nested := manager.NestedHooks()
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
	root := manager.Hooks()
	nested := manager.NestedHooks()
	from := common.HexToAddress("0x1")
	to := common.HexToAddress("0x2")
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
		fieldType := hooksType.Field(i).Type
		hooksValue.Field(i).Set(reflect.MakeFunc(fieldType, func(_ []reflect.Value) []reflect.Value {
			return nil
		}))
	}

	base := hooksValue.Addr().Interface().(*tracing.Hooks)
	dispatcherValue := reflect.ValueOf(New(base).Hooks()).Elem()
	for i := 0; i < hooksType.NumField(); i++ {
		require.Falsef(t, dispatcherValue.Field(i).IsNil(), "hook %s was not composed", hooksType.Field(i).Name)
	}
}

func TestManagerUsesDispatchSnapshot(t *testing.T) {
	manager := New(nil)
	var calls []string
	var uninstallSecond func()

	manager.InstallApplication(newTestTracer(&tracing.Hooks{
		OnExit: func(_ int, _ []byte, _ uint64, _ error, _ bool) {
			calls = append(calls, "first")
			uninstallSecond()
		},
	}))
	uninstallSecond = manager.InstallApplication(newTestTracer(&tracing.Hooks{
		OnExit: func(_ int, _ []byte, _ uint64, _ error, _ bool) {
			calls = append(calls, "second")
		},
	}))

	manager.Hooks().OnExit(0, nil, 0, nil, false)
	require.Equal(t, []string{"first", "second"}, calls)

	calls = nil
	manager.Hooks().OnExit(0, nil, 0, nil, false)
	require.Equal(t, []string{"first"}, calls)
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
