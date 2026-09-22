package keeper

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cosmos/evm/server/config"
	evmtrace "github.com/cosmos/evm/trace"
	"github.com/cosmos/evm/x/vm/statedb"
	vmtracer "github.com/cosmos/evm/x/vm/tracer"
	"github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// CallEVM performs a smart contract method call using given args.
// Note: if you call this from a precompile context, ensure that
// you use the existing stateDB.
func (k Keeper) CallEVM(ctx sdk.Context, stateDB *statedb.StateDB, abi abi.ABI, from, contract common.Address, commit, callFromPrecompile bool, gasCap *big.Int, method string, args ...interface{}) (_ *types.MsgEthereumTxResponse, err error) {
	ctx, span := ctx.StartSpan(tracer, "CallEVM", trace.WithAttributes(
		attribute.String("from", from.Hex()),
		attribute.String("contract", contract.Hex()),
		attribute.String("method", method),
		attribute.Bool("commit", commit),
	))
	defer func() { evmtrace.EndSpanErr(span, err) }()
	data, err := abi.Pack(method, args...)
	if err != nil {
		return nil, errorsmod.Wrap(
			types.ErrABIPack,
			errorsmod.Wrap(err, "failed to create transaction data").Error(),
		)
	}

	resp, err := k.CallEVMWithData(ctx, stateDB, from, &contract, data, commit, callFromPrecompile, gasCap)
	if err != nil {
		return resp, errorsmod.Wrapf(err, "contract call failed: method '%s', contract '%s'", method, contract)
	}
	return resp, nil
}

// CallEVMWithData performs a smart contract method call using contract data.
// Note: if you call this from a precompile context, ensure that
// you use the existing stateDB.
func (k Keeper) CallEVMWithData(ctx sdk.Context, stateDB *statedb.StateDB, from common.Address, contract *common.Address, data []byte, commit bool, callFromPrecompile bool, gasCap *big.Int) (_ *types.MsgEthereumTxResponse, err error) {
	contractAddr := ""
	if contract != nil {
		contractAddr = contract.Hex()
	}
	ctx, span := ctx.StartSpan(tracer, "CallEVMWithData", trace.WithAttributes(
		attribute.String("from", from.Hex()),
		attribute.String("contract", contractAddr),
		attribute.Bool("commit", commit),
		attribute.Int("data_size", len(data)),
	))
	defer func() { evmtrace.EndSpanErr(span, err) }()
	nonce, err := k.accountKeeper.GetSequence(ctx, from.Bytes())
	if err != nil {
		return nil, err
	}

	gasLimit := config.DefaultGasCap
	if gasCap != nil && gasCap.Sign() > 0 {
		if gasCap.BitLen() <= 64 {
			provided := gasCap.Uint64()
			if provided < gasLimit {
				gasLimit = provided
			}
		}
	}

	msg := core.Message{
		From:       from,
		To:         contract,
		Nonce:      nonce,
		Value:      big.NewInt(0),
		GasLimit:   gasLimit,
		GasPrice:   big.NewInt(0),
		GasTipCap:  big.NewInt(0),
		GasFeeCap:  big.NewInt(0),
		Data:       data,
		AccessList: ethtypes.AccessList{},
	}
	txConfig := statedb.NewEmptyTxConfig()
	ctx, tracingHooks := k.prepareCallEVMTracing(ctx, stateDB, msg, txConfig, commit)

	res, err := k.ApplyMessage(ctx, stateDB, msg, tracingHooks, commit, callFromPrecompile, true)
	if err != nil {
		return nil, err
	}

	if res.Failed() {
		k.ResetGasMeterAndConsumeGas(ctx, ctx.GasMeter().Limit())
		return res, errorsmod.Wrap(types.ErrVMExecution, res.VmError)
	}

	ctx.GasMeter().ConsumeGas(res.GasUsed, "apply evm message")

	return res, nil
}

// CallEVMViewWithData performs an isolated, non-committing EVM call and caps the
// execution at the caller's remaining gas. The isolated call starts its own
// application-tracing lifecycle instead of inheriting the caller's manager.
func (k *Keeper) CallEVMViewWithData(ctx sdk.Context, from common.Address, contract *common.Address, data []byte, gasCap *big.Int) (_ *types.MsgEthereumTxResponse, err error) {
	remainingGas := ctx.GasMeter().GasRemaining()
	if remainingGas == 0 {
		return nil, errorsmod.Wrap(sdkerrors.ErrOutOfGas, "no gas remaining for EVM view call")
	}

	effectiveGasCap := new(big.Int).SetUint64(min(remainingGas, config.DefaultGasCap))
	if gasCap != nil && gasCap.Sign() > 0 && gasCap.Cmp(effectiveGasCap) < 0 {
		effectiveGasCap.Set(gasCap)
	}
	limitedByParent := effectiveGasCap.IsUint64() && effectiveGasCap.Uint64() == remainingGas

	execCtx := vmtracer.WithoutManager(buildTraceCtx(ctx, remainingGas))
	stateDB := statedb.New(execCtx, k, statedb.NewEmptyTxConfig())

	res, err := k.CallEVMWithData(execCtx, stateDB, from, contract, data, false, false, effectiveGasCap)
	if err != nil {
		parentOutOfGas := limitedByParent &&
			(res != nil && res.VmError == vm.ErrOutOfGas.Error() ||
				res == nil && (errors.Is(err, core.ErrIntrinsicGas) || errors.Is(err, core.ErrFloorDataGas)))
		if parentOutOfGas {
			ctx.GasMeter().ConsumeGas(remainingGas, "apply evm message")
			return res, errorsmod.Wrap(sdkerrors.ErrOutOfGas, err.Error())
		}
		if res != nil {
			ctx.GasMeter().ConsumeGas(res.GasUsed, "apply evm message")
		}
		return res, err
	}

	ctx.GasMeter().ConsumeGas(res.GasUsed, "apply evm message")
	return res, nil
}
