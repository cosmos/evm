package testutil

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types/mocks"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// StatusRevert also carries gRPC NotFound to exercise terminal-vs-policy order.
type StatusRevert struct{}

func (StatusRevert) Error() string      { return "custom server revert" }
func (StatusRevert) RevertData() []byte { return []byte{0xde, 0xad, 0xbe, 0xef, 1} }
func (StatusRevert) GRPCStatus() *status.Status {
	return status.New(codes.NotFound, "custom server revert")
}

// StatusOutOfGas exercises transport wrappers that also carry an EVM sentinel.
type StatusOutOfGas struct{}

func (StatusOutOfGas) Error() string              { return "custom server out of gas" }
func (StatusOutOfGas) Unwrap() error              { return vm.ErrOutOfGas }
func (StatusOutOfGas) GRPCStatus() *status.Status { return status.New(codes.NotFound, "out of gas") }

// TestBoundaryAdapter exercises the actual adapter and native EVM error boundary.
func TestBoundaryAdapter(t *testing.T, adapter func(sdk.Context, error) error) {
	t.Helper()
	for _, input := range []error{nil, StatusOutOfGas{}, vm.ErrOutOfGas, fmt.Errorf("outer: %w", vm.ErrOutOfGas), StatusRevert{}, fmt.Errorf("outer: %w", StatusRevert{})} {
		t.Run(fmt.Sprint(input), func(t *testing.T) {
			ctx := sdktestutil.DefaultContext(storetypes.NewKVStoreKey("boundary"), storetypes.NewTransientStoreKey("boundary_t"))
			actual := adapter(ctx, input)
			require.Equal(t, input, actual)
			db := statedb.New(ctx, mocks.NewEVMKeeper(), statedb.NewEmptyTxConfig())
			evm := &vm.EVM{StateDB: db}
			contract := vm.NewContract(common.Address{}, common.Address{}, uint256.NewInt(0), 1_000_000, nil)
			var invoked bool
			output, err := (cmn.Precompile{}).RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
				invoked = true
				return nil, adapter(ctx, input)
			})
			require.True(t, invoked)
			switch {
			case input == nil:
				require.NoError(t, err)
			case errors.Is(input, vm.ErrOutOfGas):
				require.ErrorIs(t, actual, vm.ErrOutOfGas)
				require.Same(t, vm.ErrOutOfGas, err)
				require.Nil(t, output)
			default:
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				require.Equal(t, StatusRevert{}.RevertData(), output)
				require.Equal(t, output, evm.ReturnData())
			}
		})
	}
}

// TestBoundaryEquivalence compares full revert bytes
// against Cosmos mappings and the boundary fallback using the same ABI and registry.
func TestBoundaryEquivalence(t *testing.T, api abi.ABI, registry *cmn.CosmosErrorRegistry, method string, msg bool, adapter func(sdk.Context, error) error, inputs ...error) {
	t.Helper()
	for _, input := range inputs {
		t.Run(input.Error(), func(t *testing.T) {
			ctx := sdk.Context{}
			expected := cmn.QueryError(api, registry, method, input)
			if msg {
				translation := cmn.TranslateCosmosError(api, registry, input)
				expected = translation.Revert
				if translation.Kind == cmn.MappingKindInternal {
					expected = cmn.NewRevertWithSolidityError(api, cmn.SolidityErrMsgServerFailed, method, input.Error())
				}
			}
			actual := adapter(ctx, input)
			require.Equal(t, expected.(cmn.RevertDataCarrier).RevertData(), actual.(cmn.RevertDataCarrier).RevertData())
		})
	}
}
