package keeper

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"

	cmttypes "github.com/cometbft/cometbft/types"

	antetypes "github.com/cosmos/evm/ante/types"
	rpctypes "github.com/cosmos/evm/rpc/types"
	"github.com/cosmos/evm/utils"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	consensustypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
)

// NewEVMWithOverridePrecompiles creates a new EVM instance with opcode hooks and optionally overrides
// the precompiles call hook. If overridePrecompiles is true, the EVM will use the keeper's static precompiles
// for call hooks; otherwise, it will use the recipient-specific precompile hook.
// This is useful for scenarios such as eth_call, state overrides, or testing where custom precompile logic is needed.
// The function sets up the block context, transaction context, and VM configuration before returning the EVM instance.
func (k *Keeper) NewEVMWithOverridePrecompiles(
	ctx sdk.Context,
	msg core.Message,
	cfg *statedb.EVMConfig,
	tracer *tracing.Hooks,
	stateDB vm.StateDB,
	overridePrecompiles bool,
) *vm.EVM {
	ctx = k.SetConsensusParamsInCtx(ctx)
	blockCtx := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     k.GetHashFn(ctx),
		Coinbase:    cfg.CoinBase,
		GasLimit:    antetypes.BlockGasLimit(ctx),
		BlockNumber: big.NewInt(ctx.BlockHeight()),
		Time:        uint64(ctx.BlockHeader().Time.Unix()), //#nosec G115 -- int overflow is not a concern here
		Difficulty:  big.NewInt(0),                         // unused. Only required in PoW context
		BaseFee:     cfg.BaseFee,
		Random:      &common.MaxHash, // need to be different than nil to signal it is after the merge and pick up the right opcodes
	}

	ethCfg := types.GetEthChainConfig()
	txCtx := core.NewEVMTxContext(&msg)
	if tracer == nil {
		tracer = k.Tracer(ctx, msg, ethCfg)
	}
	vmConfig := k.VMConfig(ctx, msg, cfg, tracer)

	signer := msg.From
	accessControl := types.NewRestrictedPermissionPolicy(&cfg.Params.AccessControl, signer)

	// Set hooks for the EVM opcodes
	evmHooks := types.NewDefaultOpCodesHooks()
	evmHooks.AddCreateHooks(
		accessControl.GetCreateHook(signer),
	)
	evmHooks.AddCallHooks(
		accessControl.GetCallHook(signer),
	)
	if overridePrecompiles {
		evmHooks.AddCallHooks(
			k.GetPrecompilesCallHook(ctx),
		)
	} else {
		evmHooks.AddCallHooks(
			k.GetPrecompileRecipientCallHook(ctx),
		)
	}
	return vm.NewEVMWithHooks(evmHooks, blockCtx, txCtx, stateDB, ethCfg, vmConfig)
}

// NewEVM generates a go-ethereum VM from the provided Message fields and the chain parameters
// (ChainConfig and module Params). It additionally sets the validator operator address as the
// coinbase address to make it available for the COINBASE opcode, even though there is no
// beneficiary of the coinbase transaction (since we're not mining).
//
// NOTE: the RANDOM opcode is currently not supported since it requires
// RANDAO implementation. See https://github.com/evmos/ethermint/pull/1520#pullrequestreview-1200504697
// for more information.
func (k *Keeper) NewEVM(
	ctx sdk.Context,
	msg core.Message,
	cfg *statedb.EVMConfig,
	tracer *tracing.Hooks,
	stateDB vm.StateDB,
) *vm.EVM {
	return k.NewEVMWithOverridePrecompiles(
		ctx,
		msg,
		cfg,
		tracer,
		stateDB,
		true,
	)
}

// GetHashFn implements vm.GetHashFunc for Ethermint. It handles 3 cases:
//  1. The requested height matches the current height from context (and thus same epoch number)
//  2. The requested height is from an previous height from the same chain epoch
//  3. The requested height is from a height greater than the latest one
func (k Keeper) GetHashFn(ctx sdk.Context) vm.GetHashFunc {
	return func(height uint64) common.Hash {
		h, err := utils.SafeInt64(height)
		if err != nil {
			k.Logger(ctx).Error("failed to cast height to int64", "error", err)
			return common.Hash{}
		}

		switch {
		case ctx.BlockHeight() == h:
			// Case 1: The requested height matches the one from the context so we can retrieve the header
			// hash directly from the context.
			// Note: The headerHash is only set at begin block, it will be nil in case of a query context
			headerHash := ctx.HeaderHash()
			if len(headerHash) != 0 {
				return common.BytesToHash(headerHash)
			}

			// only recompute the hash if not set (eg: checkTxState)
			contextBlockHeader := ctx.BlockHeader()
			header, err := cmttypes.HeaderFromProto(&contextBlockHeader)
			if err != nil {
				k.Logger(ctx).Error("failed to cast CometBFT header from proto", "error", err)
				return common.Hash{}
			}

			headerHash = header.Hash()
			return common.BytesToHash(headerHash)

		case ctx.BlockHeight() > h:
			// Case 2: The requested height is historical, query EIP-2935 contract storage for that
			// see: https://github.com/cosmos/evm/issues/406
			return k.GetHeaderHash(ctx, height)
		default:
			// Case 3: The requested height is greater than the latest one, return empty hash
			return common.Hash{}
		}
	}
}

// logsBloom returns the bloom bytes for the given logs
func logsBloom(logs []*ethtypes.Log) []byte {
	var bin ethtypes.Bloom
	for _, log := range logs {
		bin.Add(log.Address.Bytes())
		for _, b := range log.Topics {
			bin.Add(b[:])
		}
	}
	return bin[:]
}

func calculateCumulativeGasFromEthResponse(meter storetypes.GasMeter, res *types.MsgEthereumTxResponse) uint64 {
	cumulativeGasUsed := res.GasUsed
	if meter != nil {
		limit := meter.Limit()
		cumulativeGasUsed += meter.GasConsumed()
		if cumulativeGasUsed > limit {
			cumulativeGasUsed = limit
		}
	}
	return cumulativeGasUsed
}

// ApplyTransaction runs and attempts to perform a state transition with the given transaction (i.e Message), that will
// only be persisted (committed) to the underlying KVStore if the transaction does not fail.
//
// # Gas tracking
//
// Ethereum consumes gas according to the EVM opcodes instead of general reads and writes to store. Because of this, the
// state transition needs to ignore the SDK gas consumption mechanism defined by the GasKVStore and instead consume the
// amount of gas used by the VM execution. The amount of gas used is tracked by the EVM and returned in the execution
// result.
//
// Prior to the execution, the starting tx gas meter is saved and replaced with an infinite gas meter in a new context
// to ignore the SDK gas consumption config values (read, write, has, delete).
// After the execution, the gas used from the message execution will be added to the starting gas consumed, taking into
// consideration the amount of gas returned. Finally, the context is updated with the EVM gas consumed value prior to
// returning.
//
// For relevant discussion see: https://github.com/cosmos/cosmos-sdk/discussions/9072
func (k *Keeper) ApplyTransaction(ctx sdk.Context, tx *ethtypes.Transaction) (*types.MsgEthereumTxResponse, error) {
	cfg, err := k.EVMConfig(ctx, ctx.BlockHeader().ProposerAddress)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to load evm config")
	}
	txConfig := k.TxConfig(ctx, tx.Hash())

	// get the signer according to the chain rules from the config and block height
	signer := ethtypes.MakeSigner(types.GetEthChainConfig(), big.NewInt(ctx.BlockHeight()), uint64(ctx.BlockTime().Unix())) //#nosec G115 -- int overflow is not a concern here
	msg, err := core.TransactionToMessage(tx, signer, cfg.BaseFee)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to return ethereum transaction as core message")
	}

	// create a cache context to revert state. The cache context is only committed when both tx and hooks executed successfully.
	// Didn't use `Snapshot` because the context stack has exponential complexity on certain operations,
	// thus restricted to be used only inside `ApplyMessage`.
	tmpCtx, commitFn := ctx.CacheContext()

	// pass true to commit the StateDB
	res, err := k.ApplyMessageWithConfig(tmpCtx, *msg, nil, true, cfg, txConfig, false, nil)
	if err != nil {
		// when a transaction contains multiple msg, as long as one of the msg fails
		// all gas will be deducted. so is not msg.Gas()
		k.ResetGasMeterAndConsumeGas(tmpCtx, tmpCtx.GasMeter().Limit())
		return nil, errorsmod.Wrap(err, "failed to apply ethereum core message")
	}

	ethLogs := types.LogsToEthereum(res.Logs)
	// Compute block bloom filter
	if len(ethLogs) > 0 {
		k.SetTxBloom(tmpCtx, new(big.Int).SetBytes(logsBloom(ethLogs)))
	}

	var contractAddr common.Address
	if msg.To == nil {
		contractAddr = crypto.CreateAddress(msg.From, msg.Nonce)
	}

	receipt := &ethtypes.Receipt{
		Type:              tx.Type(),
		PostState:         nil,
		CumulativeGasUsed: calculateCumulativeGasFromEthResponse(ctx.GasMeter(), res),
		Logs:              ethLogs,
		TxHash:            txConfig.TxHash,
		ContractAddress:   contractAddr,
		GasUsed:           res.GasUsed,
		BlockHash:         common.BytesToHash(ctx.HeaderHash()),
		BlockNumber:       big.NewInt(ctx.BlockHeight()),
		TransactionIndex:  uint(ctx.TxIndex()), //#nosec G115
	}

	if res.Failed() {
		receipt.Status = ethtypes.ReceiptStatusFailed

		// If the tx failed we discard the old context and create a new one, so
		// PostTxProcessing can persist data even if the tx fails.
		tmpCtx, commitFn = ctx.CacheContext()
	} else {
		receipt.Status = ethtypes.ReceiptStatusSuccessful
	}

	signerAddr, err := signer.Sender(tx)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to extract sender address from ethereum transaction")
	}

	eventsLen := len(tmpCtx.EventManager().Events())

	// Only call PostTxProcessing if there are hooks set, to avoid calling commitFn unnecessarily
	if !k.HasHooks() {
		// If there are no hooks, we can commit the state immediately if the tx is successful
		if commitFn != nil && !res.Failed() {
			commitFn()
		}
	} else {
		// Note: PostTxProcessing hooks currently do not charge for gas
		// and function similar to EndBlockers in abci, but for EVM transactions.
		// It will persist data even if the tx fails.
		err = k.PostTxProcessing(tmpCtx, signerAddr, *msg, receipt)
		if err != nil {
			// If hooks returns an error, revert the whole tx.
			res.VmError = errorsmod.Wrap(err, "failed to execute post transaction processing").Error()
			k.Logger(ctx).Error("tx post processing failed", "error", err)
			// If the tx failed in post processing hooks, we should clear all log-related data
			// to match EVM behavior where transaction reverts clear all effects including logs
			res.Logs = nil
			receipt.Logs = nil
			receipt.Bloom = ethtypes.Bloom{} // Clear bloom filter
		} else {
			if commitFn != nil {
				commitFn()
			}

			// Since the post-processing can alter the log, we need to update the result
			if res.Failed() {
				res.Logs = nil
				receipt.Logs = nil
				receipt.Bloom = ethtypes.Bloom{}
			} else {
				res.Logs = types.NewLogsFromEth(receipt.Logs)
			}

			events := tmpCtx.EventManager().Events()
			if len(events) > eventsLen {
				ctx.EventManager().EmitEvents(events[eventsLen:])
			}
		}
	}

	// refund gas to match the Ethereum gas consumption instead of the default SDK one.
	remainingGas := uint64(0)
	if msg.GasLimit > res.GasUsed {
		remainingGas = msg.GasLimit - res.GasUsed
	}
	if err = k.RefundGas(ctx, *msg, remainingGas, types.GetEVMCoinDenom()); err != nil {
		return nil, errorsmod.Wrapf(err, "failed to refund leftover gas to sender %s", msg.From)
	}

	totalGasUsed, err := k.AddTransientGasUsed(ctx, res.GasUsed)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to add transient gas used")
	}

	// reset the gas meter for current cosmos transaction
	k.ResetGasMeterAndConsumeGas(ctx, totalGasUsed)
	return res, nil
}

// ApplyMessage calls ApplyMessageWithConfig with an empty TxConfig.
func (k *Keeper) ApplyMessage(ctx sdk.Context, msg core.Message, tracer *tracing.Hooks, commit bool, internal bool) (*types.MsgEthereumTxResponse, error) {
	cfg, err := k.EVMConfig(ctx, ctx.BlockHeader().ProposerAddress)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to load evm config")
	}

	txConfig := statedb.NewEmptyTxConfig()
	return k.ApplyMessageWithConfig(ctx, msg, tracer, commit, cfg, txConfig, internal, nil)
}

// ApplyMessageWithConfig computes the new state by applying the given message against the existing state.
// If the message fails, the VM execution error with the reason will be returned to the client
// and the transaction won't be committed to the store.
//
// # Reverted state
//
// The snapshot and rollback are supported by the `statedb.StateDB`.
//
// # Different Callers
//
// It's called in three scenarios:
// 1. `ApplyTransaction`, in the transaction processing flow.
// 2. `EthCall/EthEstimateGas` grpc query handler.
// 3. Called by other native modules directly.
//
// # Prechecks and Preprocessing
//
// All relevant state transition prechecks for the MsgEthereumTx are performed on the AnteHandler,
// prior to running the transaction against the state. The prechecks run are the following:
//
// 1. the nonce of the message caller is correct
// 2. caller has enough balance to cover transaction fee(gaslimit * gasprice)
// 3. the amount of gas required is available in the block
// 4. the purchased gas is enough to cover intrinsic usage
// 5. there is no overflow when calculating intrinsic gas
// 6. caller has enough balance to cover asset transfer for **topmost** call
//
// The preprocessing steps performed by the AnteHandler are:
//
// 1. set up the initial access list (iff fork > Berlin)
//
// # Tracer parameter
//
// It should be a `vm.Tracer` object or nil, if pass `nil`, it'll create a default one based on keeper options.
//
// # Commit parameter
//
// If commit is true, the `StateDB` will be committed, otherwise discarded.
func (k *Keeper) ApplyMessageWithConfig(
	ctx sdk.Context,
	msg core.Message,
	tracer *tracing.Hooks,
	commit bool,
	cfg *statedb.EVMConfig,
	txConfig statedb.TxConfig,
	internal bool,
	overrides *rpctypes.StateOverride,
) (*types.MsgEthereumTxResponse, error) {
	var (
		ret          []byte // return bytes from evm execution
		vmErr        error  // vm errors do not effect consensus and are therefore not assigned to err
		floorDataGas uint64
	)

	stateDB := statedb.New(ctx, k, txConfig)
	ethCfg := types.GetEthChainConfig()
	evm := k.NewEVMWithOverridePrecompiles(ctx, msg, cfg, tracer, stateDB, overrides == nil)
	// Gas limit suffices for the floor data cost (EIP-7623)
	rules := ethCfg.Rules(evm.Context.BlockNumber, true, evm.Context.Time)
	if overrides != nil {
		precompiles := vm.ActivePrecompiledContracts(rules)
		if err := overrides.Apply(stateDB, precompiles); err != nil {
			return nil, errorsmod.Wrap(err, "failed to apply state override")
		}
		evm.WithPrecompiles(precompiles)
	}

	leftoverGas := msg.GasLimit

	// Allow the tracer captures the tx level events, mainly the gas consumption.
	vmCfg := evm.Config
	if vmCfg.Tracer != nil {
		vmCfg.Tracer.OnTxStart(
			evm.GetVMContext(),
			ethtypes.NewTx(&ethtypes.LegacyTx{To: msg.To, Data: msg.Data, Value: msg.Value, Gas: msg.GasLimit}),
			msg.From,
		)
		defer func() {
			if vmCfg.Tracer.OnTxEnd != nil {
				vmCfg.Tracer.OnTxEnd(&ethtypes.Receipt{GasUsed: msg.GasLimit - leftoverGas}, vmErr)
			}
		}()
	}

	sender := vm.AccountRef(msg.From)
	contractCreation := msg.To == nil
	isLondon := ethCfg.IsLondon(evm.Context.BlockNumber)

	intrinsicGas, err := k.GetEthIntrinsicGas(ctx, msg, ethCfg, contractCreation)
	if err != nil {
		// should have already been checked on Ante Handler
		return nil, errorsmod.Wrap(err, "intrinsic gas failed")
	}

	// Should check again even if it is checked on Ante Handler, because eth_call don't go through Ante Handler.
	if leftoverGas < intrinsicGas {
		// eth_estimateGas will check for this exact error
		return nil, errorsmod.Wrap(core.ErrIntrinsicGas, "apply message")
	}
	if rules.IsPrague {
		floorDataGas, err = core.FloorDataGas(msg.Data)
		if err != nil {
			return nil, err
		}
		if msg.GasLimit < floorDataGas {
			return nil, fmt.Errorf("%w: have %d, want %d", core.ErrFloorDataGas, msg.GasLimit, floorDataGas)
		}
	}
	leftoverGas -= intrinsicGas

	// access list preparation is moved from ante handler to here, because it's needed when `ApplyMessage` is called
	// under contexts where ante handlers are not run, for example `eth_call` and `eth_estimateGas`.
	stateDB.Prepare(rules, msg.From, common.Address{}, msg.To, evm.ActivePrecompiles(), msg.AccessList)

	convertedValue, err := utils.Uint256FromBigInt(msg.Value)
	if err != nil {
		return nil, err
	}

	if contractCreation {
		// take over the nonce management from evm:
		// - reset sender's nonce to msg.Nonce() before calling evm.
		// - increase sender's nonce by one no matter the result.
		stateDB.SetNonce(sender.Address(), msg.Nonce, tracing.NonceChangeEoACall)
		ret, _, leftoverGas, vmErr = evm.Create(sender.Address(), msg.Data, leftoverGas, convertedValue)
		stateDB.SetNonce(sender.Address(), msg.Nonce+1, tracing.NonceChangeContractCreator)
	} else {
		// Apply EIP-7702 authorizations.
		if msg.SetCodeAuthorizations != nil {
			for _, auth := range msg.SetCodeAuthorizations {
				// Note errors are ignored, we simply skip invalid authorizations here.
				if err := k.applyAuthorization(&auth, stateDB, ethCfg.ChainID); err != nil {
					k.Logger(ctx).Debug("failed to apply authorization", "error", err, "authorization", auth)
				}
			}
		}

		// Perform convenience warming of sender's delegation target. Although the
		// sender is already warmed in Prepare(..), it's possible a delegation to
		// the account was deployed during this transaction. To handle correctly,
		// simply wait until the final state of delegations is determined before
		// performing the resolution and warming.
		if addr, ok := ethtypes.ParseDelegation(stateDB.GetCode(*msg.To)); ok {
			stateDB.AddAddressToAccessList(addr)
		}
		ret, leftoverGas, vmErr = evm.Call(sender.Address(), *msg.To, msg.Data, leftoverGas, convertedValue)
	}

	refundQuotient := params.RefundQuotient

	// After EIP-3529: refunds are capped to gasUsed / 5
	if isLondon {
		refundQuotient = params.RefundQuotientEIP3529
	}

	// calculate gas refund
	if msg.GasLimit < leftoverGas {
		return nil, errorsmod.Wrap(types.ErrGasOverflow, "apply message")
	}
	// refund gas
	maxUsedGas := msg.GasLimit - leftoverGas
	refund := GasToRefund(stateDB.GetRefund(), maxUsedGas, refundQuotient)

	// update leftoverGas and temporaryGasUsed with refund amount
	leftoverGas += refund
	temporaryGasUsed := maxUsedGas - refund
	if rules.IsPrague {
		// After EIP-7623: Data-heavy transactions pay the floor gas.
		if temporaryGasUsed < floorDataGas {
			prev := leftoverGas
			leftoverGas = msg.GasLimit - floorDataGas
			temporaryGasUsed = floorDataGas
			if vmCfg.Tracer != nil && vmCfg.Tracer.OnGasChange != nil {
				vmCfg.Tracer.OnGasChange(prev, leftoverGas, tracing.GasChangeTxDataFloor)
			}
		}
		if maxUsedGas < floorDataGas {
			maxUsedGas = floorDataGas
		}
	}

	// EVM execution error needs to be available for the JSON-RPC client
	var vmError string
	if vmErr != nil {
		vmError = vmErr.Error()
	}

	// The dirty states in `StateDB` is either committed or discarded after return
	if commit {
		if err := stateDB.Commit(); err != nil {
			return nil, errorsmod.Wrap(err, "failed to commit stateDB")
		}
	}

	// calculate a minimum amount of gas to be charged to sender if GasLimit
	// is considerably higher than GasUsed to stay more aligned with CometBFT gas mechanics
	// for more info https://github.com/evmos/ethermint/issues/1085
	gasLimit := math.LegacyNewDecFromInt(math.NewIntFromUint64(msg.GasLimit)) //#nosec G115 -- int overflow is not a concern here -- msg gas is not exceeding int64 max value
	minGasMultiplier := cfg.FeeMarketParams.MinGasMultiplier
	if minGasMultiplier.IsNil() {
		// in case we are executing eth_call on a legacy block, returns a zero value.
		minGasMultiplier = math.LegacyZeroDec()
	}
	minimumGasUsed := gasLimit.Mul(minGasMultiplier)

	if !minimumGasUsed.TruncateInt().IsUint64() {
		return nil, errorsmod.Wrapf(types.ErrGasOverflow, "minimumGasUsed(%s) is not a uint64", minimumGasUsed.TruncateInt().String())
	}

	if msg.GasLimit < leftoverGas {
		return nil, errorsmod.Wrapf(types.ErrGasOverflow, "message gas limit < leftover gas (%d < %d)", msg.GasLimit, leftoverGas)
	}

	gasUsed := math.LegacyNewDec(int64(temporaryGasUsed)) //#nosec G115 -- int overflow is not a concern here
	if !internal {
		gasUsed = math.LegacyMaxDec(gasUsed, minimumGasUsed)
	}
	// reset leftoverGas, to be used by the tracer
	leftoverGas = msg.GasLimit - gasUsed.TruncateInt().Uint64()

	// if the execution reverted, we return the revert reason as the return data
	if vmError == vm.ErrExecutionReverted.Error() {
		ret = evm.Interpreter().ReturnData()
	}
	return &types.MsgEthereumTxResponse{
		GasUsed:        gasUsed.TruncateInt().Uint64(),
		MaxUsedGas:     maxUsedGas,
		VmError:        vmError,
		Ret:            ret,
		Logs:           types.NewLogsFromEth(stateDB.Logs()),
		Hash:           txConfig.TxHash.Hex(),
		BlockHash:      ctx.HeaderHash(),
		BlockTimestamp: evm.Context.Time,
	}, nil
}

// SetConsensusParamsInCtx will return the original context if consensus params already exist in it, otherwise, it will
// query the consensus params from the consensus params keeper and then set it in context.
func (k *Keeper) SetConsensusParamsInCtx(ctx sdk.Context) sdk.Context {
	cp := ctx.ConsensusParams()
	if cp.Block != nil {
		return ctx
	}

	res, err := k.consensusKeeper.Params(ctx, &consensustypes.QueryParamsRequest{})
	if err != nil {
		return ctx
	}
	return ctx.WithConsensusParams(*res.Params)
}

// applyAuthorization applies an EIP-7702 code delegation to the state.
func (k *Keeper) applyAuthorization(auth *ethtypes.SetCodeAuthorization, state vm.StateDB, chainID *big.Int) error {
	authority, err := k.validateAuthorization(auth, state, chainID)
	if err != nil {
		return err
	}

	// If the account already exists in state, refund the new account cost
	// charged in the intrinsic calculation.
	if state.Exist(authority) {
		state.AddRefund(params.CallNewAccountGas - params.TxAuthTupleGas)
	}

	// Update nonce and account code.
	state.SetNonce(authority, auth.Nonce+1, tracing.NonceChangeAuthorization)
	if auth.Address == (common.Address{}) {
		// Delegation to zero address means clear.
		state.SetCode(authority, nil)
		return nil
	}

	// Otherwise install delegation to auth.Address.
	state.SetCode(authority, ethtypes.AddressToDelegation(auth.Address))

	return nil
}

// validateAuthorization validates an EIP-7702 authorization against the state.
func (k *Keeper) validateAuthorization(auth *ethtypes.SetCodeAuthorization, state vm.StateDB, chainID *big.Int) (authority common.Address, err error) {
	// Verify chain ID is null or equal to current chain ID.
	if !auth.ChainID.IsZero() && auth.ChainID.CmpBig(chainID) != 0 {
		return authority, core.ErrAuthorizationWrongChainID
	}
	// Limit nonce to 2^64-1 per EIP-2681.
	if auth.Nonce+1 < auth.Nonce {
		return authority, core.ErrAuthorizationNonceOverflow
	}
	// Validate signature values and recover authority.
	authority, err = auth.Authority()
	if err != nil {
		return authority, fmt.Errorf("%w: %v", core.ErrAuthorizationInvalidSignature, err)
	}
	// Check the authority account
	//  1) doesn't have code or has exisiting delegation
	//  2) matches the auth's nonce
	//
	// Note it is added to the access list even if the authorization is invalid.
	state.AddAddressToAccessList(authority)
	code := state.GetCode(authority)
	if _, ok := ethtypes.ParseDelegation(code); len(code) != 0 && !ok {
		return authority, core.ErrAuthorizationDestinationHasCode
	}
	if have := state.GetNonce(authority); have != auth.Nonce {
		return authority, core.ErrAuthorizationNonceMismatch
	}
	return authority, nil
}
