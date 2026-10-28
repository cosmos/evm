package vm

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	gethvm "github.com/ethereum/go-ethereum/core/vm"

	precompilecommon "github.com/cosmos/evm/precompiles/common"
	vmkeeper "github.com/cosmos/evm/x/vm/keeper"
	"github.com/cosmos/evm/x/vm/statedb"
	vmtracer "github.com/cosmos/evm/x/vm/tracer"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type applicationHooksTracer struct {
	hooks *tracing.Hooks
}

func (t *applicationHooksTracer) Hooks() *tracing.Hooks { return t.hooks }

type nestedCallEVMPrecompile struct {
	precompilecommon.Precompile
	keeper  *vmkeeper.Keeper
	target  common.Address
	manager *vmtracer.Manager
}

type viewCallEVMPrecompile struct {
	precompilecommon.Precompile
	keeper        *vmkeeper.Keeper
	target        common.Address
	parentManager *vmtracer.Manager
}

func (p *viewCallEVMPrecompile) Name() string { return "view-call-evm-tracing-test" }

func (p *viewCallEVMPrecompile) RequiredGas(_ []byte) uint64 { return 0 }

func (p *viewCallEVMPrecompile) Run(evm *gethvm.EVM, contract *gethvm.Contract, _ bool) ([]byte, error) {
	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		p.parentManager, _ = vmtracer.FromContext(ctx)
		response, err := p.keeper.CallEVMViewWithData(
			ctx,
			p.Address(),
			&p.target,
			nil,
			new(big.Int).SetUint64(contract.Gas),
		)
		if err != nil {
			return nil, err
		}
		return response.Ret, nil
	})
}

func (p *nestedCallEVMPrecompile) Name() string { return "nested-call-evm-tracing-test" }

func (p *nestedCallEVMPrecompile) RequiredGas(_ []byte) uint64 { return 0 }

func (p *nestedCallEVMPrecompile) Run(evm *gethvm.EVM, contract *gethvm.Contract, _ bool) ([]byte, error) {
	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		p.manager, _ = vmtracer.FromContext(ctx)
		stateDB, ok := evm.StateDB.(*statedb.StateDB)
		if !ok {
			return nil, fmt.Errorf("unexpected StateDB type %T", evm.StateDB)
		}
		response, err := p.keeper.CallEVMWithData(
			ctx,
			stateDB,
			p.Address(),
			&p.target,
			nil,
			true,
			true,
			new(big.Int).SetUint64(contract.Gas),
		)
		if err != nil {
			return nil, err
		}
		return response.Ret, nil
	})
}

func (s *KeeperTestSuite) TestNestedCallEVMWithDataReusesApplicationTracing() {
	s.SetupTest()

	ctx := s.Network.GetContext()
	evmKeeper := s.Network.App.GetEVMKeeper()
	precompileAddress := common.HexToAddress("0x000000000000000000000000000000000000fffe")
	precompile := &nestedCallEVMPrecompile{
		Precompile: precompilecommon.Precompile{
			KvGasConfig:          ctx.KVGasConfig(),
			TransientKVGasConfig: ctx.TransientKVGasConfig(),
			ContractAddress:      precompileAddress,
		},
		keeper: evmKeeper,
		target: s.Keyring.GetAddr(1),
	}
	evmKeeper.RegisterStaticPrecompile(precompileAddress, precompile)
	params := evmKeeper.GetParams(ctx)
	params.ActiveStaticPrecompiles = append(params.ActiveStaticPrecompiles, precompileAddress.String())
	s.Require().NoError(evmKeeper.SetParams(ctx, params))

	type tracerCounts struct {
		txStarts int
		txEnds   int
		enters   int
	}
	var factoryCalls []string
	var factoryManagers []*vmtracer.Manager
	first, second := &tracerCounts{}, &tracerCounts{}
	newFactory := func(name string, counts *tracerCounts) vmtracer.ApplicationTracerFactory {
		return func(factoryCtx sdk.Context, _ vmtracer.ExecutionInfo) (sdk.Context, vmtracer.Tracer) {
			factoryCalls = append(factoryCalls, name)
			manager, _ := vmtracer.FromContext(factoryCtx)
			factoryManagers = append(factoryManagers, manager)
			return factoryCtx, &applicationHooksTracer{hooks: &tracing.Hooks{
				OnTxStart: func(_ *tracing.VMContext, _ *ethtypes.Transaction, _ common.Address) { counts.txStarts++ },
				OnTxEnd:   func(_ *ethtypes.Receipt, _ error) { counts.txEnds++ },
				OnEnter:   func(_ int, _ byte, _, _ common.Address, _ []byte, _ uint64, _ *big.Int) { counts.enters++ },
			}}
		}
	}
	evmKeeper.SetApplicationTracerFactories(
		newFactory("application-1", first),
		newFactory("application-2", second),
	)

	tx, err := s.Factory.GenerateSignedEthTx(s.Keyring.GetPrivKey(0), evmtypes.EvmTxArgs{
		To:       &precompileAddress,
		GasLimit: 500_000,
		GasPrice: big.NewInt(0),
	})
	s.Require().NoError(err)
	response, err := evmKeeper.ApplyTransaction(
		ctx.WithGasMeter(storetypes.NewGasMeter(1_000_000)),
		tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx).AsTransaction(),
	)

	s.Require().NoError(err)
	s.Require().False(response.Failed())
	s.Require().Equal([]string{"application-1", "application-2"}, factoryCalls)
	s.Require().Len(factoryManagers, 2)
	s.Require().Same(factoryManagers[0], factoryManagers[1])
	for _, counts := range []*tracerCounts{first, second} {
		s.Require().Equal(1, counts.txStarts)
		s.Require().Equal(1, counts.txEnds)
		s.Require().GreaterOrEqual(counts.enters, 2)
	}
	s.Require().NotNil(factoryManagers[0])
	s.Require().Same(factoryManagers[0], precompile.manager)
}

func (s *KeeperTestSuite) TestCallEVMViewWithDataStartsIndependentApplicationTracing() {
	s.SetupTest()

	ctx := s.Network.GetContext()
	evmKeeper := s.Network.App.GetEVMKeeper()
	precompileAddress := common.HexToAddress("0x000000000000000000000000000000000000fffd")
	target := s.Keyring.GetAddr(1)
	precompile := &viewCallEVMPrecompile{
		Precompile: precompilecommon.Precompile{
			KvGasConfig:          ctx.KVGasConfig(),
			TransientKVGasConfig: ctx.TransientKVGasConfig(),
			ContractAddress:      precompileAddress,
		},
		keeper: evmKeeper,
		target: target,
	}
	evmKeeper.RegisterStaticPrecompile(precompileAddress, precompile)
	params := evmKeeper.GetParams(ctx)
	params.ActiveStaticPrecompiles = append(params.ActiveStaticPrecompiles, precompileAddress.String())
	s.Require().NoError(evmKeeper.SetParams(ctx, params))

	var managers []*vmtracer.Manager
	var entered [][]common.Address
	evmKeeper.SetApplicationTracerFactories(func(factoryCtx sdk.Context, _ vmtracer.ExecutionInfo) (sdk.Context, vmtracer.Tracer) {
		manager, ok := vmtracer.FromContext(factoryCtx)
		s.Require().True(ok)
		managers = append(managers, manager)
		index := len(entered)
		entered = append(entered, nil)
		return factoryCtx, &applicationHooksTracer{hooks: &tracing.Hooks{
			OnEnter: func(_ int, _ byte, _, to common.Address, _ []byte, _ uint64, _ *big.Int) {
				entered[index] = append(entered[index], to)
			},
		}}
	})

	tx, err := s.Factory.GenerateSignedEthTx(s.Keyring.GetPrivKey(0), evmtypes.EvmTxArgs{
		To:       &precompileAddress,
		GasLimit: 500_000,
		GasPrice: big.NewInt(0),
	})
	s.Require().NoError(err)
	response, err := evmKeeper.ApplyTransaction(
		ctx.WithGasMeter(storetypes.NewGasMeter(1_000_000)),
		tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx).AsTransaction(),
	)

	s.Require().NoError(err)
	s.Require().False(response.Failed())
	s.Require().Len(managers, 2)
	s.Require().Same(managers[0], precompile.parentManager)
	s.Require().NotSame(managers[0], managers[1])
	s.Require().Contains(entered[0], precompileAddress)
	s.Require().NotContains(entered[0], target)
	s.Require().Contains(entered[1], target)
}
