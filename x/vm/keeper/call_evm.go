package keeper

import (
	"encoding/json"
	"math/big"
	"strconv"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/evm/server/config"
	"github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// CallEVM performs a smart contract method call using given args.
func (k Keeper) CallEVM(
	ctx sdk.Context,
	abi abi.ABI,
	from, contract common.Address,
	commit bool,
	method string,
	args ...interface{},
) (*types.MsgEthereumTxResponse, error) {
	data, err := abi.Pack(method, args...)
	if err != nil {
		return nil, errorsmod.Wrap(
			types.ErrABIPack,
			errorsmod.Wrap(err, "failed to create transaction data").Error(),
		)
	}

	resp, err := k.CallEVMWithData(ctx, from, &contract, data, commit)
	if err != nil {
		return nil, errorsmod.Wrapf(err, "contract call failed: method '%s', contract '%s'", method, contract)
	}
	return resp, nil
}

// CallEVMWithData performs a smart contract method call using contract data.
func (k Keeper) CallEVMWithData(
	ctx sdk.Context,
	from common.Address,
	contract *common.Address,
	data []byte,
	commit bool,
) (*types.MsgEthereumTxResponse, error) {
	nonce, err := k.accountKeeper.GetSequence(ctx, from.Bytes())
	if err != nil {
		return nil, err
	}

	gasCap := config.DefaultGasCap
	if commit {
		args, err := json.Marshal(types.TransactionArgs{
			From: &from,
			To:   contract,
			Data: (*hexutil.Bytes)(&data),
		})
		if err != nil {
			return nil, errorsmod.Wrapf(errortypes.ErrJSONMarshal, "failed to marshal tx args: %s", err.Error())
		}

		gasRes, err := k.EstimateGasInternal(ctx, &types.EthCallRequest{
			Args:   args,
			GasCap: config.DefaultGasCap,
		}, types.Internal)
		if err != nil {
			return nil, err
		}
		gasCap = gasRes.Gas
	}

	msg := ethtypes.NewMessage(
		from,
		contract,
		nonce,
		big.NewInt(0), // amount
		gasCap,        // gasLimit
		big.NewInt(0), // gasFeeCap
		big.NewInt(0), // gasTipCap
		big.NewInt(0), // gasPrice
		data,
		ethtypes.AccessList{}, // AccessList
		!commit,               // isFake
	)

	res, err := k.ApplyMessage(ctx, msg, types.NewNoOpTracer(), commit)
	if err != nil {
		return nil, err
	}

	if res.Failed() {
		return nil, errorsmod.Wrap(types.ErrVMExecution, res.VmError)
	}

	return res, nil
}

// NOTE: A DerivedTx is a MsgEthereumTx reconstructed from Tendermint ABCI events.
// These transactions are not submitted via Ethereum RPC but are derived from Cosmos-based messages
// to provide consistent EVM compatibility and traceability.

// DerivedEVMCall performs an internal EVM contract call using the given method and arguments.
// It ABI-encodes the method call, constructs the transaction data, and invokes the EVM.
//
// Returns (msg, err), where:
//   - msg contains the EVM execution result (including revert data if applicable)
//   - err is non-nil if the EVM execution failed or the contract call reverted.
//
// Note: If err != nil and msg != nil and msg.Failed() == true,
// the contract execution reverted (e.g. REVERT opcode was triggered).
func (k Keeper) DerivedEVMCall(
	ctx sdk.Context,
	abi abi.ABI,
	from, contract common.Address,
	value, gasLimit *big.Int,
	commit, gasless, isModuleSender bool,
	manualNonce *uint64,
	method string,
	args ...interface{},
) (*types.MsgEthereumTxResponse, error) {
	data, err := abi.Pack(method, args...)
	if err != nil {
		return nil, errorsmod.Wrap(
			types.ErrABIPack,
			errorsmod.Wrap(err, "failed to create transaction data").Error(),
		)
	}

	resp, err := k.DerivedEVMCallWithData(ctx, from, &contract, data, commit, gasless, isModuleSender, value, gasLimit, manualNonce)
	if err != nil {
		return nil, errorsmod.Wrapf(err, "contract call failed: method '%s', contract '%s'", method, contract)
	}
	return resp, nil
}

// DerivedEVMCallWithData performs an internal EVM contract call using raw call data.
//
// Parameters:
// - from: The sender address.
// - to: The contract address (nil for contract creation).
// - data: Raw EVM call data (ABI-encoded).
// - value: Amount of wei to send with the call.
// - gasLimit: Optional custom gas limit; if nil, gas estimation will be attempted (which may underpredict).
// - commit: Whether to persist state changes (true) or execute as a read-only simulation (false).
//
// Behavior:
//   - If err != nil and msg != nil and msg.Failed() == true, the contract execution reverted.
//     In such cases, msg.Ret contains the revert reason if available (from the REVERT opcode).
//
// Returns:
// - *types.MsgEthereumTxResponse: The result of EVM execution.
// - error: Non-nil if the call failed, including reverts.
func (k Keeper) DerivedEVMCallWithData(
	ctx sdk.Context,
	from common.Address,
	contract *common.Address,
	data []byte,
	commit, gasless, isModuleSender bool,
	value, gasLimit *big.Int,
	manualNonce *uint64,
) (*types.MsgEthereumTxResponse, error) {
	var nonce uint64
	if isModuleSender {
		if manualNonce == nil {
			return nil, errorsmod.Wrap(errortypes.ErrInvalidSequence, "manual nonce required for module sender")
		}
		nonce = *manualNonce
	} else {
		n, err := k.accountKeeper.GetSequence(ctx, from.Bytes())
		if err != nil {
			return nil, err
		}
		nonce = n
	}

	gasCap := config.DefaultGasCap
	if commit && gasLimit == nil {
		args, err := json.Marshal(types.TransactionArgs{
			From: &from,
			To:   contract,
			Data: (*hexutil.Bytes)(&data),
		})
		if err != nil {
			return nil, errorsmod.Wrapf(errortypes.ErrJSONMarshal, "failed to marshal tx args: %s", err.Error())
		}

		gasRes, err := k.EstimateGasInternal(ctx, &types.EthCallRequest{
			Args:   args,
			GasCap: config.DefaultGasCap,
		}, types.Internal)
		if err != nil {
			return nil, err
		}
		gasCap = gasRes.Gas
	}
	if gasLimit != nil {
		gasCap = gasLimit.Uint64()
	}

	msg := ethtypes.NewMessage(
		from,
		contract,
		nonce,
		value,         // amount
		gasCap,        // gasLimit
		big.NewInt(0), // gasFeeCap
		big.NewInt(0), // gasTipCap
		big.NewInt(0), // gasPrice
		data,
		ethtypes.AccessList{}, // AccessList
		!commit,               // isFake
	)
	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		Nonce:     msg.Nonce(),
		GasFeeCap: msg.GasFeeCap(),
		GasTipCap: msg.GasTipCap(),
		Gas:       msg.Gas(),
		To:        msg.To(),
		Value:     msg.Value(),
		Data:      msg.Data(),
	})

	cfg, err := k.EVMConfig(ctx, sdk.ConsAddress(ctx.BlockHeader().ProposerAddress))
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to load evm config")
	}
	txConfig := k.TxConfig(ctx, tx.Hash())

	// Create a cache context to revert state. The cache context is only committed when both tx and hooks executed successfully.
	// Didn't use `Snapshot` because the context stack has exponential complexity on certain operations,
	// thus restricted to be used only inside `ApplyMessage`.
	tmpCtx, commitState := ctx.CacheContext()

	// pass true to commit the StateDB
	res, err := k.ApplyMessageWithConfig(tmpCtx, msg, nil, true, cfg, txConfig)
	if err != nil {
		return nil, err
	}

	if !res.Failed() {
		commitState()
	}

	// Emit events and log for the transaction if it is committed
	if commit {
		ethTxHash := res.Hash
		gasUsed := res.GasUsed
		if gasless {
			gasUsed = 0
		}
		attrs := []sdk.Attribute{}
		attrs = append(attrs, []sdk.Attribute{
			sdk.NewAttribute(sdk.AttributeKeyAmount, value.String()),
			// add event for ethereum transaction hash format;
			sdk.NewAttribute(types.AttributeKeyEthereumTxHash, ethTxHash),
			// add event for index of valid ethereum tx; NOTE: default txindex for derivedTx
			sdk.NewAttribute(types.AttributeKeyTxIndex, strconv.FormatUint(types.DerivedTxIndex, 10)),
			// add event for eth tx gas used, we can't get it from cosmos tx result when it contains multiple eth tx msgs.
			sdk.NewAttribute(types.AttributeKeyTxGasUsed, strconv.FormatUint(gasUsed, 10)),
		}...)

		// recipient: contract address
		if contract != nil {
			attrs = append(attrs, sdk.NewAttribute(types.AttributeKeyRecipient, contract.Hex()))
		}
		if res.Failed() {
			attrs = append(attrs, sdk.NewAttribute(types.AttributeKeyEthereumTxFailed, res.VmError))
		}

		txLogAttrs := make([]sdk.Attribute, len(res.Logs))
		for i, log := range res.Logs {
			log.TxHash = ethTxHash
			value, err := json.Marshal(log)
			if err != nil {
				return nil, errorsmod.Wrap(err, "failed to encode log")
			}
			txLogAttrs[i] = sdk.NewAttribute(types.AttributeKeyTxLog, string(value))
		}

		// adding txData for more info in rpc methods in order to parse derived txs
		attrs = append(attrs, sdk.NewAttribute(types.AttributeKeyTxData, hexutil.Encode(msg.Data())))
		// adding nonce for more info in rpc methods in order to parse derived txs
		attrs = append(attrs, sdk.NewAttribute(types.AttributeKeyTxNonce, strconv.FormatUint(nonce, 10)))
		ctx.EventManager().EmitEvents(sdk.Events{
			sdk.NewEvent(
				types.EventTypeEthereumTx,
				attrs...,
			),
			sdk.NewEvent(
				types.EventTypeTxLog,
				txLogAttrs...,
			),
			sdk.NewEvent(
				sdk.EventTypeMessage,
				sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
				sdk.NewAttribute(sdk.AttributeKeySender, from.Hex()),
				sdk.NewAttribute(types.AttributeKeyTxType, strconv.FormatUint(types.DerivedTxType, 10)),
			),
		})

		logs := types.LogsToEthereum(res.Logs)
		var bloomReceipt ethtypes.Bloom
		if len(logs) > 0 {
			bloom := k.GetBlockBloomTransient(ctx)
			bloom.Or(bloom, big.NewInt(0).SetBytes(ethtypes.LogsBloom(logs)))
			bloomReceipt = ethtypes.BytesToBloom(bloom.Bytes())
			k.SetBlockBloomTransient(ctx, bloomReceipt.Big())
			k.SetLogSizeTransient(ctx, (k.GetLogSizeTransient(ctx))+uint64(len(logs)))
		}
	}

	if res.Failed() {
		return res, errorsmod.Wrapf(types.ErrVMExecution, "%s: ret 0x%x", res.VmError, res.Ret)
	}

	return res, nil
}
