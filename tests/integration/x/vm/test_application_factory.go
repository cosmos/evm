package vm

import (
	"encoding/json"
	"errors"
	"math/big"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"

	"github.com/cosmos/evm/x/vm/statedb"
	vmtracer "github.com/cosmos/evm/x/vm/tracer"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func (s *KeeperTestSuite) TestApplicationTracerFactoryError() {
	for _, route := range []string{
		"ApplyTransaction", "CallEVMWithData", "CallEVMViewWithData",
		"EthCall", "EstimateGas", "TraceTx", "TraceTxPredecessor",
	} {
		s.Run(route, func() {
			s.SetupTest()
			ctx := s.Network.GetContext().WithGasMeter(storetypes.NewGasMeter(1_000_000))
			keeper := s.Network.App.GetEVMKeeper()
			sender, recipient := s.Keyring.GetAddr(0), s.Keyring.GetAddr(1)
			before := keeper.GetBalance(ctx, recipient)
			nonceBefore := keeper.GetNonce(ctx, sender)
			tx, err := s.Factory.GenerateSignedEthTx(s.Keyring.GetPrivKey(0), evmtypes.EvmTxArgs{
				To: &recipient, Amount: big.NewInt(7), GasLimit: 100_000, GasPrice: big.NewInt(0),
			})
			s.Require().NoError(err)
			msg := tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
			args, err := json.Marshal(evmtypes.TransactionArgs{From: &sender, To: &recipient})
			s.Require().NoError(err)
			callReq := &evmtypes.EthCallRequest{
				Args: args, GasCap: 100_000, ProposerAddress: ctx.BlockHeader().ProposerAddress,
			}
			traceReq := getDefaultTraceTxRequest(s.Network)
			traceReq.Msg = msg
			traceReq.TraceConfig.Tracer = "callTracer"
			if route == "TraceTxPredecessor" {
				traceReq.Predecessors = []*evmtypes.MsgEthereumTx{msg}
			}

			factoryErr := errors.New("application tracer initialization failed")
			factoryCalls, subsequentCalls, hookCalls := 0, 0, 0
			keeper.SetApplicationTracerFactories(
				func(ctx sdk.Context, _ vmtracer.ExecutionInfo) (sdk.Context, vmtracer.Tracer, error) {
					return ctx, newApplicationHooksTracer(&tracing.Hooks{
						OnEnter: func(int, byte, common.Address, common.Address, []byte, uint64, *big.Int) { hookCalls++ },
					}), nil
				},
				func(ctx sdk.Context, _ vmtracer.ExecutionInfo) (sdk.Context, vmtracer.Tracer, error) {
					factoryCalls++
					return sdk.Context{}, nil, factoryErr
				},
				func(ctx sdk.Context, _ vmtracer.ExecutionInfo) (sdk.Context, vmtracer.Tracer, error) {
					subsequentCalls++
					return ctx, nil, nil
				},
			)

			switch route {
			case "ApplyTransaction":
				_, err = keeper.ApplyTransaction(ctx, msg.AsTransaction())
			case "CallEVMWithData":
				db := statedb.New(ctx, keeper, statedb.NewEmptyTxConfig())
				_, err = keeper.CallEVMWithData(ctx, db, sender, &recipient, nil, true, false, big.NewInt(100_000))
			case "CallEVMViewWithData":
				_, err = keeper.CallEVMViewWithData(ctx, sender, &recipient, nil, big.NewInt(100_000))
			case "EthCall":
				_, err = keeper.EthCall(ctx, callReq)
			case "EstimateGas":
				_, err = keeper.EstimateGas(ctx, callReq)
			case "TraceTx", "TraceTxPredecessor":
				_, err = keeper.TraceTx(ctx, traceReq)
			}
			s.Require().ErrorContains(err, factoryErr.Error())
			if route == "ApplyTransaction" || route == "CallEVMWithData" || route == "CallEVMViewWithData" {
				s.Require().ErrorIs(err, factoryErr)
			}
			s.Require().Positive(factoryCalls)
			s.Require().Zero(subsequentCalls)
			s.Require().Zero(hookCalls)
			s.Require().Equal(before, keeper.GetBalance(ctx, recipient))
			s.Require().Equal(nonceBefore, keeper.GetNonce(ctx, sender))
		})
	}
}
