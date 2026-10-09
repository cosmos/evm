package common_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/x/vm/statedb"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// noopKeeper satisfies statedb.Keeper; RunNativeAction only needs the store
// keys to build its cache context.
type noopKeeper struct {
	key *storetypes.KVStoreKey
}

var _ statedb.Keeper = noopKeeper{}

func (noopKeeper) GetAccount(sdk.Context, common.Address) *statedb.Account { return nil }
func (noopKeeper) GetState(sdk.Context, common.Address, common.Hash) common.Hash {
	return common.Hash{}
}
func (noopKeeper) GetCode(sdk.Context, common.Hash) []byte             { return nil }
func (noopKeeper) GetCodeHash(sdk.Context, common.Address) common.Hash { return common.Hash{} }
func (noopKeeper) ForEachStorage(sdk.Context, common.Address, func(common.Hash, common.Hash) bool) {
}
func (noopKeeper) SetAccount(sdk.Context, common.Address, statedb.Account) error { return nil }
func (noopKeeper) DeleteState(sdk.Context, common.Address, common.Hash)          {}
func (noopKeeper) SetState(sdk.Context, common.Address, common.Hash, []byte)     {}
func (noopKeeper) DeleteCode(sdk.Context, []byte)                                {}
func (noopKeeper) SetCode(sdk.Context, []byte, []byte)                           {}
func (noopKeeper) DeleteAccount(sdk.Context, common.Address) error               { return nil }
func (k noopKeeper) KVStoreKeys() map[string]storetypes.StoreKey {
	return map[string]storetypes.StoreKey{k.key.Name(): k.key}
}

func setupNativeAction(t *testing.T, gas uint64) (cmn.Precompile, *vm.EVM, *vm.Contract) {
	t.Helper()
	key := storetypes.NewKVStoreKey(t.Name())
	tkey := storetypes.NewTransientStoreKey(t.Name() + "_t")
	ctx := testutil.DefaultContext(key, tkey).WithEventManager(sdk.NewEventManager())
	db := statedb.New(ctx, noopKeeper{key: key}, statedb.NewEmptyTxConfig())

	evm := vm.NewEVM(vm.BlockContext{BlockNumber: big.NewInt(1), Time: 1}, db, params.TestChainConfig, vm.Config{})
	p := cmn.Precompile{
		KvGasConfig:          storetypes.KVGasConfig(),
		TransientKVGasConfig: storetypes.TransientGasConfig(),
		ContractAddress:      common.HexToAddress("0x0000000000000000000000000000000000000801"),
	}
	contract := vm.NewContract(common.Address{1}, p.ContractAddress, uint256.NewInt(0), gas, nil)
	return p, evm, contract
}

// SDK gas consumed before an ordinary error must still be charged, while the
// error is still returned as a revert.
func TestRunNativeActionChargesGasOnError(t *testing.T) {
	const gasLimit, workGas = 100_000, 5_000
	p, evm, contract := setupNativeAction(t, gasLimit)

	bz, err := p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		ctx.GasMeter().ConsumeGas(workGas, "work before failing")
		return nil, errors.New("invalid input")
	})

	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.NotEmpty(t, bz, "revert reason should be preserved")
	require.Equal(t, uint64(gasLimit-workGas), contract.Gas)
}

func TestRunNativeActionChargesGasOnSuccess(t *testing.T) {
	const gasLimit, workGas = 100_000, 5_000
	p, evm, contract := setupNativeAction(t, gasLimit)

	bz, err := p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		ctx.GasMeter().ConsumeGas(workGas, "work")
		return []byte{1}, nil
	})

	require.NoError(t, err)
	require.Equal(t, []byte{1}, bz)
	require.Equal(t, uint64(gasLimit-workGas), contract.Gas)
}
