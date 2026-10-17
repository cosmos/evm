package common_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types/mocks"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// gasTestPrecompile is a minimal stateful precompile built on cmn.Precompile
// whose native action is supplied by the test.
type gasTestPrecompile struct {
	cmn.Precompile
	action cmn.NativeAction
}

func (p *gasTestPrecompile) RequiredGas(input []byte) uint64 {
	return p.Precompile.RequiredGas(input, true)
}

func (p *gasTestPrecompile) Name() string { return "gasTestPrecompile" }

func (p *gasTestPrecompile) Run(evm *vm.EVM, contract *vm.Contract, _ bool) ([]byte, error) {
	return p.RunNativeAction(evm, contract, p.action)
}

// TestRunNativeActionChargesSDKGasOnAllExitPaths checks the property that the
// EVM gas consumed by a precompile call frame is at least the SDK gas consumed
// by its native action, regardless of whether the action succeeds, runs out of
// gas, or returns an error (which surfaces to the caller as a revert).
func TestRunNativeActionChargesSDKGasOnAllExitPaths(t *testing.T) {
	const (
		suppliedGas = uint64(200_000)
		sdkWork     = uint64(50_000)
	)

	addr := common.HexToAddress("0x0000000000000000000000000000000000000c0d")
	caller := common.HexToAddress("0x00000000000000000000000000000000000000aa")
	input := []byte{0x01, 0x02, 0x03, 0x04}

	testCases := []struct {
		name        string
		action      cmn.NativeAction
		expErr      error
		minConsumed uint64
	}{
		{
			name: "success",
			action: func(ctx sdk.Context) ([]byte, error) {
				ctx.GasMeter().ConsumeGas(sdkWork, "test work")
				return nil, nil
			},
			expErr:      nil,
			minConsumed: sdkWork,
		},
		{
			name: "action returns error after consuming SDK gas",
			action: func(ctx sdk.Context) ([]byte, error) {
				ctx.GasMeter().ConsumeGas(sdkWork, "test work")
				return nil, errors.New("action failed")
			},
			expErr:      vm.ErrExecutionReverted,
			minConsumed: sdkWork,
		},
		{
			name: "action runs out of SDK gas",
			action: func(ctx sdk.Context) ([]byte, error) {
				ctx.GasMeter().ConsumeGas(suppliedGas+1, "test work")
				return nil, nil
			},
			expErr:      vm.ErrOutOfGas,
			minConsumed: suppliedGas,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			storeKey := storetypes.NewKVStoreKey("test")
			tKey := storetypes.NewTransientStoreKey("test_t")
			ctx := sdktestutil.DefaultContext(storeKey, tKey)
			stateDB := statedb.New(ctx, mocks.NewEVMKeeper(), statedb.NewEmptyTxConfig())

			p := &gasTestPrecompile{
				Precompile: cmn.Precompile{
					KvGasConfig:          storetypes.KVGasConfig(),
					TransientKVGasConfig: storetypes.TransientGasConfig(),
					ContractAddress:      addr,
				},
				action: tc.action,
			}

			blockCtx := vm.BlockContext{
				CanTransfer: core.CanTransfer,
				Transfer:    core.Transfer,
				BlockNumber: big.NewInt(1),
				Time:        1,
				Difficulty:  big.NewInt(0),
				BaseFee:     big.NewInt(0),
			}
			evm := vm.NewEVM(blockCtx, stateDB, params.TestChainConfig, vm.Config{})
			evm.SetPrecompiles(vm.PrecompiledContracts{addr: p})

			_, leftOver, err := evm.Call(caller, addr, input, suppliedGas, uint256.NewInt(0))
			if tc.expErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.expErr)
			}

			consumed := suppliedGas - leftOver
			require.GreaterOrEqual(t, consumed, tc.minConsumed,
				"EVM gas consumed by the precompile frame (%d) must cover SDK gas consumed by the action (%d); RequiredGas=%d",
				consumed, tc.minConsumed, p.RequiredGas(input))
		})
	}
}
