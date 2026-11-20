package keeper

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cosmos/evm/server/config"
	evmtrace "github.com/cosmos/evm/trace"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
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

	// Cap the gas limit of this internal EVM call by the gas remaining on the
	// transaction's Cosmos gas meter.
	gasMeter := ctx.GasMeter()
	if remaining := gasMeter.GasRemaining(); remaining < gasLimit {
		gasLimit = remaining
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

	metered := types.MeteredInternalCalls(ctx)
	res, err := k.ApplyMessage(ctx, stateDB, msg, nil, commit, callFromPrecompile, true)
	if err != nil {
		if metered && (errors.Is(err, core.ErrIntrinsicGas) || errors.Is(err, core.ErrFloorDataGas)) {
			// This check happens before EVM execution. Report budget exhaustion
			// to the enclosing operation, including through helpers such as BalanceOf.
			const descriptor = "insufficient gas for internal EVM call"
			gasMeter.ConsumeGas(gasMeter.GasRemaining(), descriptor)
			panic(storetypes.ErrorOutOfGas{Descriptor: descriptor})
		}
		return nil, err
	}

	if metered {
		// The enclosing operation can still discard the conversion, including
		// after a successful internal call. Charge gas without state refunds.
		gasMeter.ConsumeGas(res.MaxUsedGas, "apply evm message")
	} else if res.Failed() {
		k.ResetGasMeterAndConsumeGas(ctx, gasMeter.Limit())
	} else {
		gasMeter.ConsumeGas(res.GasUsed, "apply evm message")
	}

	if res.Failed() {
		return res, errorsmod.Wrap(types.ErrVMExecution, res.VmError)
	}

	return res, nil
}
