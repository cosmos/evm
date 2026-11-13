package vm

import (
	"errors"
	"fmt"
	"math/big"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	"github.com/cosmos/evm/x/vm/statedb"
	vmtracer "github.com/cosmos/evm/x/vm/tracer"
	vmglobaltracer "github.com/cosmos/evm/x/vm/tracer/application"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func newApplicationHooksTracer(hooks *tracing.Hooks) vmtracer.Tracer {
	return &applicationHooksTracer{hooks: hooks}
}

func (s *KeeperTestSuite) TestCollectorActualMovementAndObservationRejection() {
	for _, reject := range []bool{false, true} {
		s.Run(fmt.Sprintf("reject=%v", reject), func() {
			s.SetupTest()
			ctx := s.Network.GetContext()
			keeper := s.Network.App.GetEVMKeeper()
			var collector vmglobaltracer.Collector
			rejection := errors.New("observation policy rejected")
			keeper.SetApplicationTracerFactories(func(factoryCtx sdk.Context, _ vmtracer.ExecutionInfo) (sdk.Context, vmtracer.Tracer) {
				collector = vmglobaltracer.NewCollector()
				if reject {
					_, err := collector.Observe(func(e vmtracer.Observation) error {
						if e.Kind == vmtracer.NativeTransfer {
							return rejection
						}
						return nil
					})
					s.Require().NoError(err)
				}
				return factoryCtx, collector
			})
			sender, recipient := s.Keyring.GetAddr(0), s.Keyring.GetAddr(1)
			before := keeper.GetBalance(ctx, recipient)
			tx, err := s.Factory.GenerateSignedEthTx(s.Keyring.GetPrivKey(0), evmtypes.EvmTxArgs{
				To:       &recipient,
				Amount:   big.NewInt(7),
				GasLimit: 100_000,
				GasPrice: big.NewInt(0),
			})
			s.Require().NoError(err)
			response, err := keeper.ApplyTransaction(ctx.WithGasMeter(storetypes.NewGasMeter(1_000_000)), tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx).AsTransaction())
			s.Require().NotNil(collector)
			facts := collector.Observations()
			s.Require().NotEmpty(facts)
			transfers := nativeObservations(collector.Observations())
			s.Require().Len(transfers, 1)
			s.Require().Equal(sender, transfers[0].From)
			s.Require().Equal(recipient, transfers[0].To)
			s.Require().Equal(int64(7), transfers[0].Amount.Int64())
			balances := 0
			for _, e := range facts {
				if e.Kind == vmtracer.BalanceChange {
					balances++
				}
			}
			s.Require().Equal(2, balances)
			if reject {
				s.Require().ErrorIs(err, rejection)
				s.Require().Equal(before, keeper.GetBalance(ctx, recipient))
			} else {
				s.Require().NoError(err)
				s.Require().False(response.Failed())
				s.Require().NoError(collector.Error())
				s.Require().True(transfers[0].Status.Success)
			}
		})
	}
}

func (s *KeeperTestSuite) TestCollectorRetainsChildMovementAfterParentRevert() {
	s.SetupTest()
	ctx := s.Network.GetContext()
	keeper := s.Network.App.GetEVMKeeper()
	sender, recipient := s.Keyring.GetAddr(0), s.Keyring.GetAddr(1)
	parent := common.HexToAddress("0x000000000000000000000000000000000000eeee")
	// CALL recipient with value 7, consume its success flag, then REVERT.
	code := common.FromHex("0x6000600060006000600773")
	code = append(code, recipient.Bytes()...)
	code = append(code, common.FromHex("0x61fffff15060006000fd")...)
	db := statedb.New(ctx, keeper, statedb.NewEmptyTxConfig())
	db.SetCode(parent, code, tracing.CodeChangeUnspecified)
	s.Require().NoError(db.Commit())
	var collector vmglobaltracer.Collector
	keeper.SetApplicationTracerFactories(func(factoryCtx sdk.Context, _ vmtracer.ExecutionInfo) (sdk.Context, vmtracer.Tracer) {
		collector = vmglobaltracer.NewCollector()
		return factoryCtx, collector
	})
	before := keeper.GetBalance(ctx, recipient)
	tx, err := s.Factory.GenerateSignedEthTx(s.Keyring.GetPrivKey(0), evmtypes.EvmTxArgs{
		To:       &parent,
		Amount:   big.NewInt(9),
		GasLimit: 200_000,
		GasPrice: big.NewInt(0),
	})
	s.Require().NoError(err)
	response, err := keeper.ApplyTransaction(ctx.WithGasMeter(storetypes.NewGasMeter(1_000_000)), tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx).AsTransaction())
	s.Require().NoError(err)
	s.Require().True(response.Failed())
	s.Require().NoError(collector.Error())
	transfers := nativeObservations(collector.Observations())
	s.Require().Len(transfers, 2)
	s.Require().Equal(sender, transfers[0].From)
	s.Require().Equal(parent, transfers[0].To)
	s.Require().Equal(parent, transfers[1].From)
	s.Require().Equal(recipient, transfers[1].To)
	s.Require().Equal(int64(7), transfers[1].Amount.Int64())
	s.Require().NotEqual(transfers[0].ObservationID, transfers[1].ObservationID)
	for _, transfer := range transfers {
		s.Require().True(transfer.Status.Reverted)
		s.Require().False(transfer.Status.Success)
	}
	childSucceeded := false
	for _, e := range collector.Observations() {
		if e.Kind == vmtracer.CallExit && e.CallID == transfers[1].CallID {
			s.Require().NoError(e.Err)
			s.Require().False(e.Reverted)
			s.Require().True(e.Status.Reverted)
			childSucceeded = true
		}
	}
	s.Require().True(childSucceeded)
	s.Require().Equal(before, keeper.GetBalance(ctx, recipient))
}

func (s *KeeperTestSuite) TestCollectorTokenRequestAndRawLog() {
	s.SetupTest()
	sender := s.Keyring.GetKey(0)
	recipient := s.Keyring.GetAddr(1)
	token, err := deployErc20Contract(sender, s.Factory)
	s.Require().NoError(err)
	s.Require().NoError(s.Network.NextBlock())
	var collector vmglobaltracer.Collector
	s.Network.App.GetEVMKeeper().SetApplicationTracerFactories(func(ctx sdk.Context, _ vmtracer.ExecutionInfo) (sdk.Context, vmtracer.Tracer) {
		collector = vmglobaltracer.NewCollector()
		return ctx, collector
	})
	_, err = executeTransferCall(transferParams{
		senderKey:     sender,
		contractAddr:  token,
		recipientAddr: recipient,
	}, s.Factory)
	s.Require().NoError(err)
	s.Require().NoError(collector.Error())
	var enter, request, emission *vmtracer.Observation
	for _, e := range collector.Observations() {
		switch e.Kind {
		case vmtracer.CallEnter:
			if e.Target == token {
				observation := e
				enter = &observation
			}
		case vmtracer.ERC20Call:
			observation := e
			request = &observation
		case vmtracer.Log:
			if len(e.Log.Topics) == 3 && e.Log.Address == token {
				observation := e
				emission = &observation
			}
		case vmtracer.NativeTransfer:
			s.FailNow("token log must not become a native transfer")
		}
	}
	s.Require().NotNil(enter)
	s.Require().NotNil(request)
	s.Require().NotNil(emission)
	s.Require().Equal(enter.CallID, request.CallID)
	s.Require().Equal(enter.CallID, emission.CallID)
	s.Require().Equal(enter.Input, request.Input)
	s.Require().Equal(sender.Addr, request.Caller)
	s.Require().Equal(token, request.Target)
	s.Require().Less(enter.ObservationID, request.ObservationID)
	s.Require().Less(request.ObservationID, emission.ObservationID)
	s.Require().Equal(common.BytesToHash(sender.Addr.Bytes()), emission.Log.Topics[1])
	s.Require().Equal(common.BytesToHash(recipient.Bytes()), emission.Log.Topics[2])
	s.Require().Equal(int64(1000), new(big.Int).SetBytes(emission.Log.Data).Int64())
	s.Require().True(emission.Status.Success)
}

func nativeObservations(events []vmtracer.Observation) []vmtracer.Observation {
	var out []vmtracer.Observation
	for _, e := range events {
		if e.Kind == vmtracer.NativeTransfer {
			out = append(out, e)
		}
	}
	return out
}

func (s *KeeperTestSuite) TestCollectorEnforcementCoverageAndFinalization() {
	for _, mode := range []string{"budget", "observer", "passive", "txend-reject", "txend-accept"} {
		s.Run(mode, func() {
			s.SetupTest()
			ctx := s.Network.GetContext()
			keeper := s.Network.App.GetEVMKeeper()
			recipient := common.HexToAddress("0x000000000000000000000000000000000000eeef")
			// Write 1 to storage slot 0, then STOP.
			db := statedb.New(ctx, keeper, statedb.NewEmptyTxConfig())
			db.SetCode(recipient, common.FromHex("0x600160005500"), tracing.CodeChangeUnspecified)
			s.Require().NoError(db.Commit())
			before := keeper.GetBalance(ctx, recipient)
			rejection := errors.New("final observation policy rejected")
			var collector vmglobaltracer.Collector
			var manager *vmtracer.Manager
			observed, ended := false, 0
			keeper.SetApplicationTracerFactories(func(factoryCtx sdk.Context, _ vmtracer.ExecutionInfo) (sdk.Context, vmtracer.Tracer) {
				manager, _ = vmtracer.FromContext(factoryCtx)
				s.Require().NotNil(manager)
				budget := uint64(100)
				if mode == "budget" || mode == "passive" {
					budget = 3
				}
				var err error
				collector, err = vmglobaltracer.NewCollectorWithBudget(budget)
				s.Require().NoError(err)
				if mode == "budget" || mode == "observer" {
					_, err = collector.Observe(func(e vmtracer.Observation) error {
						if e.Kind == vmtracer.NativeTransfer {
							observed = true
							return rejection
						}
						return nil
					})
					s.Require().NoError(err)
				}
				manager.InstallApplication(newApplicationHooksTracer(&tracing.Hooks{
					OnTxEnd: func(receipt *ethtypes.Receipt, _ error) {
						ended++
						s.Require().Positive(receipt.GasUsed)
						// No economic or storage effects may be committed yet.
						s.Require().Equal(before, keeper.GetBalance(ctx, recipient))
						s.Require().Equal(common.Hash{}, statedb.New(ctx, keeper, statedb.NewEmptyTxConfig()).GetState(recipient, common.Hash{}))
						if mode == "txend-reject" {
							manager.LatchError(rejection)
						}
					},
				}))
				return factoryCtx, collector
			})
			tx, err := s.Factory.GenerateSignedEthTx(s.Keyring.GetPrivKey(0), evmtypes.EvmTxArgs{
				To:       &recipient,
				Amount:   big.NewInt(7),
				GasLimit: 100_000,
				GasPrice: big.NewInt(0),
			})
			s.Require().NoError(err)
			response, err := keeper.ApplyTransaction(ctx.WithGasMeter(storetypes.NewGasMeter(1_000_000)), tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx).AsTransaction())
			s.Require().Equal(1, ended)
			switch mode {
			case "budget":
				s.Require().ErrorContains(err, "budget")
				s.Require().False(observed)
			case "observer", "txend-reject":
				s.Require().ErrorIs(err, rejection)
			default:
				s.Require().NoError(err)
				s.Require().False(response.Failed())
			}
			after := statedb.New(ctx, keeper, statedb.NewEmptyTxConfig())
			if mode == "passive" || mode == "txend-accept" {
				s.Require().Equal(new(uint256.Int).Add(before, uint256.NewInt(7)), keeper.GetBalance(ctx, recipient))
				s.Require().Equal(common.BigToHash(big.NewInt(1)), after.GetState(recipient, common.Hash{}))
			} else {
				s.Require().Equal(before, keeper.GetBalance(ctx, recipient))
				s.Require().Equal(common.Hash{}, after.GetState(recipient, common.Hash{}))
			}
			if mode == "observer" {
				s.Require().True(observed)
			}
		})
	}
}

func (s *KeeperTestSuite) TestCallEVMChargesGasOnPolicyRejection() {
	for _, view := range []bool{false, true} {
		var acceptedGas uint64
		for _, reject := range []bool{false, true} {
			s.Run(fmt.Sprintf("view=%t/reject=%t", view, reject), func() {
				s.SetupTest()
				ctx := s.Network.GetContext().
					WithGasMeter(storetypes.NewGasMeter(1_000_000)).
					WithKVGasConfig(storetypes.GasConfig{}).
					WithTransientKVGasConfig(storetypes.GasConfig{})
				keeper := s.Network.App.GetEVMKeeper()
				sender := s.Keyring.GetAddr(0)
				recipient := common.HexToAddress("0x000000000000000000000000000000000000eeef")
				db := statedb.New(ctx, keeper, statedb.NewEmptyTxConfig())
				// Write 1 to storage slot 0, then STOP.
				db.SetCode(recipient, common.FromHex("0x600160005500"), tracing.CodeChangeUnspecified)
				s.Require().NoError(db.Commit())
				rejection := errors.New("final policy rejected")
				var executionGas uint64
				ends := 0
				keeper.SetApplicationTracerFactories(func(factoryCtx sdk.Context, _ vmtracer.ExecutionInfo) (sdk.Context, vmtracer.Tracer) {
					manager, ok := vmtracer.FromContext(factoryCtx)
					s.Require().True(ok)
					return factoryCtx, newApplicationHooksTracer(&tracing.Hooks{
						OnTxEnd: func(receipt *ethtypes.Receipt, _ error) {
							ends++
							executionGas = receipt.GasUsed
							if reject {
								manager.LatchError(rejection)
							}
						},
					})
				})
				beforeGas := ctx.GasMeter().GasConsumed()
				var response *evmtypes.MsgEthereumTxResponse
				var err error
				if view {
					response, err = keeper.CallEVMViewWithData(ctx, sender, &recipient, nil, big.NewInt(100_000))
				} else {
					response, err = keeper.CallEVMWithData(ctx, statedb.New(ctx, keeper, statedb.NewEmptyTxConfig()), sender, &recipient, nil, true, false, big.NewInt(100_000))
				}
				s.Require().NotNil(response)
				s.Require().Equal(1, ends)
				s.Require().Positive(executionGas)
				s.Require().Equal(executionGas, response.GasUsed)
				s.Require().Equal(executionGas, ctx.GasMeter().GasConsumed()-beforeGas)
				if reject {
					s.Require().Equal(acceptedGas, executionGas)
					s.Require().ErrorIs(err, rejection)
					s.Require().Empty(response.Logs)
				} else {
					acceptedGas = executionGas
					s.Require().NoError(err)
				}
				expected := common.Hash{}
				if !view && !reject {
					expected = common.BigToHash(big.NewInt(1))
				}
				s.Require().Equal(expected, statedb.New(ctx, keeper, statedb.NewEmptyTxConfig()).GetState(recipient, common.Hash{}))
			})
		}
	}
}
