package keeper

import (
	"context"
	"math/big"
	"testing"
	"time"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/vm/statedb"
	vmtracer "github.com/cosmos/evm/x/vm/tracer"
	"github.com/cosmos/evm/x/vm/tracer/application"
	"github.com/cosmos/evm/x/vm/types"
)

var (
	benchmarkTracingContext sdk.Context
	benchmarkTracingHooks   *tracing.Hooks
)

func BenchmarkPrepareTracing(b *testing.B) {
	for _, mode := range []string{"Disabled", "BaseOnly", "Collector"} {
		b.Run(mode, func(b *testing.B) {
			k := Keeper{}
			ctx := sdk.Context{}.WithContext(context.Background())
			var base *tracing.Hooks
			if mode == "BaseOnly" {
				base = &tracing.Hooks{OnEnter: func(int, byte, common.Address, common.Address, []byte, uint64, *big.Int) {}}
			}
			if mode == "Collector" {
				k.SetApplicationTracerFactories(application.TxTraceFactory)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkTracingContext, benchmarkTracingHooks = k.prepareTracingWithBase(ctx, core.Message{}, statedb.NewEmptyTxConfig(), true, base)
			}
		})
	}
}

// Keep storage mutations out of this benchmark to isolate the real EVM transfer
// callback's tracing overhead. StateDB balance mutations are measured separately.
type benchmarkTransferState struct {
	vm.StateDB
	balance uint256.Int
}

func (s *benchmarkTransferState) AddBalance(common.Address, *uint256.Int, tracing.BalanceChangeReason) uint256.Int {
	return s.balance
}

func (s *benchmarkTransferState) SubBalance(common.Address, *uint256.Int, tracing.BalanceChangeReason) uint256.Int {
	return s.balance
}

func BenchmarkNativeTransferTracing(b *testing.B) {
	configurator := types.NewEVMConfigurator()
	configurator.ResetTestConfig()
	require.NoError(b, types.SetChainConfig(types.DefaultChainConfig(0)))
	b.Cleanup(configurator.ResetTestConfig)

	for _, mode := range []string{"NoManager", "EmptyManager", "HookOnly"} {
		b.Run(mode, func(b *testing.B) {
			ctx := sdk.Context{}.WithContext(context.Background()).
				WithBlockHeader(cmtproto.Header{Height: 1, Time: time.Unix(1, 0)}).
				WithConsensusParams(cmtproto.ConsensusParams{Block: &cmtproto.BlockParams{MaxGas: 30_000_000}})
			if mode != "NoManager" {
				var base *tracing.Hooks
				if mode == "HookOnly" {
					base = &tracing.Hooks{OnEnter: func(int, byte, common.Address, common.Address, []byte, uint64, *big.Int) {}}
				}
				ctx = vmtracer.WithManager(ctx, vmtracer.New(base))
			}
			k := Keeper{}
			state := &benchmarkTransferState{}
			evm, restore := k.newEVMWithOverridePrecompiles(ctx, core.Message{}, &statedb.EVMConfig{Params: types.DefaultParams()}, nil, state, true)
			defer restore()
			from, to := common.Address{1}, common.Address{2}
			amount := uint256.NewInt(7)
			rules := &params.Rules{}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				evm.Context.Transfer(state, from, to, amount, rules)
			}
		})
	}
}

func TestNativeTransferJournalInstalledAtTransactionStart(t *testing.T) {
	configurator := types.NewEVMConfigurator()
	configurator.ResetTestConfig()
	require.NoError(t, types.SetChainConfig(types.DefaultChainConfig(0)))
	t.Cleanup(configurator.ResetTestConfig)

	collector := application.NewCollector()
	var manager *vmtracer.Manager
	manager = vmtracer.New(&tracing.Hooks{OnTxStart: func(*tracing.VMContext, *ethtypes.Transaction, common.Address) {
		manager.InstallApplication(collector)
		manager.SetCodeReader(func(common.Address) []byte { return nil })
	}})
	ctx := vmtracer.WithManager(sdk.Context{}.WithContext(context.Background()).
		WithBlockHeader(cmtproto.Header{Height: 1, Time: time.Unix(1, 0)}).
		WithConsensusParams(cmtproto.ConsensusParams{Block: &cmtproto.BlockParams{MaxGas: 30_000_000}}), manager)
	k := Keeper{}
	state := &benchmarkTransferState{}
	evm, restore := k.newEVMWithOverridePrecompiles(ctx, core.Message{}, &statedb.EVMConfig{Params: types.DefaultParams()}, manager.Hooks(), state, true)
	defer restore()
	from, to := common.Address{1}, common.Address{2}
	manager.Hooks().OnTxStart(nil, nil, from)
	manager.Hooks().OnEnter(0, byte(vm.CALL), from, to, nil, 100, big.NewInt(7))
	evm.Context.Transfer(state, from, to, uint256.NewInt(7), &params.Rules{})
	manager.Hooks().OnExit(0, nil, 0, nil, false)
	require.NoError(t, collector.Error())
	facts := collector.Observations()
	require.Len(t, facts, 3)
	require.Equal(t, vmtracer.NativeTransfer, facts[1].Kind)
	require.Equal(t, big.NewInt(7), facts[1].Amount)
}
