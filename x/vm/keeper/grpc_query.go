package keeper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	ethparams "github.com/ethereum/go-ethereum/params"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	rpctypes "github.com/cosmos/evm/rpc/types"
	"github.com/cosmos/evm/utils"
	evmante "github.com/cosmos/evm/x/vm/ante"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ types.QueryServer = Keeper{}

const (
	defaultTraceTimeout = 5 * time.Second
	// maxTracePredecessors is the maximum amount of transaction predecessors to be included in a trace Tx request.
	// This limit is chosen as a sensible default to prevent unbounded predecessor iteration.
	maxTracePredecessors = 10_000

	maxPredecessorGas = uint64(50_000_000)
)

// Account implements the Query/Account gRPC method. The method returns the
// *spendable* balance of the account in 18 decimals representation.
func (k Keeper) Account(c context.Context, req *types.QueryAccountRequest) (*types.QueryAccountResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := utils.ValidateAddress(req.Address); err != nil {
		return nil, status.Error(
			codes.InvalidArgument, err.Error(),
		)
	}

	addr := common.HexToAddress(req.Address)

	ctx := sdk.UnwrapSDKContext(c)
	acct := k.GetAccountOrEmpty(ctx, addr)

	return &types.QueryAccountResponse{
		Balance:  acct.Balance.String(),
		CodeHash: common.BytesToHash(acct.CodeHash).Hex(),
		Nonce:    acct.Nonce,
	}, nil
}

func (k Keeper) CosmosAccount(c context.Context, req *types.QueryCosmosAccountRequest) (*types.QueryCosmosAccountResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := utils.ValidateAddress(req.Address); err != nil {
		return nil, status.Error(
			codes.InvalidArgument, err.Error(),
		)
	}

	ctx := sdk.UnwrapSDKContext(c)

	ethAddr := common.HexToAddress(req.Address)
	cosmosAddr := sdk.AccAddress(ethAddr.Bytes())

	account := k.accountKeeper.GetAccount(ctx, cosmosAddr)
	res := types.QueryCosmosAccountResponse{
		CosmosAddress: cosmosAddr.String(),
	}

	if account != nil {
		res.Sequence = account.GetSequence()
		res.AccountNumber = account.GetAccountNumber()
	}

	return &res, nil
}

// ValidatorAccount implements the Query/Balance gRPC method
func (k Keeper) ValidatorAccount(c context.Context, req *types.QueryValidatorAccountRequest) (*types.QueryValidatorAccountResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	consAddr, err := sdk.ConsAddressFromBech32(req.ConsAddress)
	if err != nil {
		return nil, status.Error(
			codes.InvalidArgument, err.Error(),
		)
	}

	ctx := sdk.UnwrapSDKContext(c)

	validator, err := k.stakingKeeper.GetValidatorByConsAddr(ctx, consAddr)
	if err != nil {
		return nil, fmt.Errorf("error while getting validator %s. %w", consAddr.String(), err)
	}

	valAddr := validator.GetOperator()
	addrBz, err := k.stakingKeeper.ValidatorAddressCodec().StringToBytes(valAddr)
	if err != nil {
		return nil, fmt.Errorf("error while getting validator %s. %w", consAddr.String(), err)
	}

	res := types.QueryValidatorAccountResponse{
		AccountAddress: sdk.AccAddress(addrBz).String(),
	}

	account := k.accountKeeper.GetAccount(ctx, addrBz)
	if account != nil {
		res.Sequence = account.GetSequence()
		res.AccountNumber = account.GetAccountNumber()
	}

	return &res, nil
}

// Balance implements the Query/Balance gRPC method. The method returns the 18
// decimal representation of the account's *spendable* balance.
func (k Keeper) Balance(c context.Context, req *types.QueryBalanceRequest) (*types.QueryBalanceResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := utils.ValidateAddress(req.Address); err != nil {
		return nil, status.Error(
			codes.InvalidArgument,
			types.ErrZeroAddress.Error(),
		)
	}

	ctx := sdk.UnwrapSDKContext(c)

	balanceInt := k.SpendableCoin(ctx, common.HexToAddress(req.Address))

	return &types.QueryBalanceResponse{
		Balance: balanceInt.String(),
	}, nil
}

// Storage implements the Query/Storage gRPC method
func (k Keeper) Storage(c context.Context, req *types.QueryStorageRequest) (*types.QueryStorageResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := utils.ValidateAddress(req.Address); err != nil {
		return nil, status.Error(
			codes.InvalidArgument,
			types.ErrZeroAddress.Error(),
		)
	}

	ctx := sdk.UnwrapSDKContext(c)

	address := common.HexToAddress(req.Address)
	key := common.HexToHash(req.Key)

	state := k.GetState(ctx, address, key)
	stateHex := state.Hex()

	return &types.QueryStorageResponse{
		Value: stateHex,
	}, nil
}

// Code implements the Query/Code gRPC method
func (k Keeper) Code(c context.Context, req *types.QueryCodeRequest) (*types.QueryCodeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if err := utils.ValidateAddress(req.Address); err != nil {
		return nil, status.Error(
			codes.InvalidArgument,
			types.ErrZeroAddress.Error(),
		)
	}

	ctx := sdk.UnwrapSDKContext(c)

	address := common.HexToAddress(req.Address)
	acct := k.GetAccountWithoutBalance(ctx, address)

	var code []byte
	if acct != nil && acct.HasCodeHash() {
		code = k.GetCode(ctx, common.BytesToHash(acct.CodeHash))
	}

	return &types.QueryCodeResponse{
		Code: code,
	}, nil
}

// Params implements the Query/Params gRPC method
func (k Keeper) Params(c context.Context, _ *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	params := k.GetParams(ctx)

	return &types.QueryParamsResponse{
		Params: params,
	}, nil
}

// EthCall implements eth_call rpc api.
func (k Keeper) EthCall(c context.Context, req *types.EthCallRequest) (*types.MsgEthereumTxResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	var overrides *rpctypes.StateOverride
	if len(req.Overrides) > 0 {
		overrides = new(rpctypes.StateOverride)
		if err := json.Unmarshal(req.Overrides, overrides); err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid state overrides format: %s", err.Error()))
		}
	}

	ctx := sdk.UnwrapSDKContext(c)

	var args types.TransactionArgs
	err := json.Unmarshal(req.Args, &args)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	cfg, err := k.EVMConfig(ctx, GetProposerAddress(ctx, req.ProposerAddress))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// ApplyMessageWithConfig expect correct nonce set in msg
	nonce := k.GetNonce(ctx, args.GetFrom())
	args.Nonce = (*hexutil.Uint64)(&nonce)

	if err := args.CallDefaults(req.GasCap, cfg.BaseFee, types.GetEthChainConfig().ChainID); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	msg := args.ToMessage(cfg.BaseFee, false, false)
	txConfig := statedb.NewEmptyTxConfig()

	// pass false to not commit StateDB
	res, err := k.ApplyMessageWithConfig(ctx, *msg, nil, false, cfg, txConfig, false, overrides)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return res, nil
}

// EstimateGas implements eth_estimateGas rpc api.
func (k Keeper) EstimateGas(c context.Context, req *types.EthCallRequest) (*types.EstimateGasResponse, error) {
	return k.EstimateGasInternal(c, req, types.RPC)
}

// EstimateGasInternal returns the gas estimation for the corresponding request.
// This function is called from the RPC client (eth_estimateGas) and internally.
// When called from the RPC client, we need to reset the gas meter before
// simulating the transaction to have
// an accurate gas estimation for EVM extensions transactions.
func (k Keeper) EstimateGasInternal(c context.Context, req *types.EthCallRequest, fromType types.CallType) (*types.EstimateGasResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	var overrides *rpctypes.StateOverride
	if len(req.Overrides) > 0 {
		overrides = new(rpctypes.StateOverride)
		if err := json.Unmarshal(req.Overrides, overrides); err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid state overrides format: %s", err.Error()))
		}
	}

	ctx := sdk.UnwrapSDKContext(c)

	if req.GasCap < ethparams.TxGas {
		return nil, status.Errorf(codes.InvalidArgument, "gas cap cannot be lower than %d", ethparams.TxGas)
	}

	var args types.TransactionArgs
	err := json.Unmarshal(req.Args, &args)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Binary search the gas requirement, as it may be higher than the amount used
	var (
		lo     uint64
		hi     uint64
		gasCap uint64
	)

	// Determine the highest gas limit can be used during the estimation.
	if args.Gas != nil && uint64(*args.Gas) >= ethparams.TxGas {
		hi = uint64(*args.Gas)
	} else {
		// Query block gas limit
		params := ctx.ConsensusParams()
		if params.Block != nil && params.Block.MaxGas > 0 {
			hi = uint64(params.Block.MaxGas) //nolint:gosec // G115 // won't exceed uint64
		} else {
			hi = req.GasCap
		}
	}

	// Recap the highest gas allowance with specified gascap.
	if req.GasCap != 0 && hi > req.GasCap {
		hi = req.GasCap
	}

	gasCap = hi
	cfg, err := k.EVMConfig(ctx, GetProposerAddress(ctx, req.ProposerAddress))
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load evm config")
	}

	// ApplyMessageWithConfig expect correct nonce set in msg
	nonce := k.GetNonce(ctx, args.GetFrom())
	args.Nonce = (*hexutil.Uint64)(&nonce)

	txConfig := statedb.NewEmptyTxConfig()

	if args.Gas == nil {
		args.Gas = new(hexutil.Uint64)
	}
	if err := args.CallDefaults(req.GasCap, cfg.BaseFee, types.GetEthChainConfig().ChainID); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// convert the tx args to an ethereum message
	msg := args.ToMessage(cfg.BaseFee, true, true)

	// Recap the highest gas limit with account's available balance.
	if msg.GasFeeCap.BitLen() != 0 {
		baseDenom := types.GetEVMCoinDenom()

		balance := k.bankWrapper.SpendableCoin(ctx, sdk.AccAddress(args.From.Bytes()), baseDenom)
		available := balance.Amount
		transfer := "0"
		if args.Value != nil {
			if args.Value.ToInt().Cmp(available.BigInt()) >= 0 {
				return nil, core.ErrInsufficientFundsForTransfer
			}
			available = available.Sub(sdkmath.NewIntFromBigInt(args.Value.ToInt()))
			transfer = args.Value.String()
		}
		allowance := available.Quo(sdkmath.NewIntFromBigInt(msg.GasFeeCap))

		// If the allowance is larger than maximum uint64, skip checking
		if allowance.IsUint64() && hi > allowance.Uint64() {
			k.Logger(ctx).Debug("Gas estimation capped by limited funds", "original", hi, "balance", balance,
				"sent", transfer, "maxFeePerGas", msg.GasFeeCap.String(), "fundable", allowance)

			hi = allowance.Uint64()
			if hi < ethparams.TxGas {
				return nil, core.ErrInsufficientFunds
			}
		}
	}

	// NOTE: the errors from the executable below should be consistent with go-ethereum,
	// so we don't wrap them with the gRPC status code

	// create a helper to check if a gas allowance results in an executable transaction
	executable := func(gas uint64) (vmError bool, rsp *types.MsgEthereumTxResponse, err error) {
		// update the message with the new gas value
		msg.GasLimit = gas

		tmpCtx := ctx
		if fromType == types.RPC {
			tmpCtx, _ = ctx.CacheContext()

			acct := k.GetAccount(tmpCtx, msg.From)

			from := msg.From
			if acct == nil {
				acc := k.accountKeeper.NewAccountWithAddress(tmpCtx, from[:])
				k.accountKeeper.SetAccount(tmpCtx, acc)
				acct = statedb.NewEmptyAccount()
			}
			// When submitting a transaction, the `EthIncrementSenderSequence` ante handler increases the account nonce
			acct.Nonce = nonce + 1
			err = k.SetAccount(tmpCtx, from, *acct)
			if err != nil {
				return true, nil, err
			}
			// resetting the gasMeter after increasing the sequence to have an accurate gas estimation on EVM extensions transactions
			tmpCtx = buildTraceCtx(tmpCtx, msg.GasLimit)
		}
		// pass "commit" as false to avoid committing StateDB
		// pass "internal" as true to avoid applying feemarketParams.MinGasMultiplier for gas estimation.
		rsp, err = k.ApplyMessageWithConfig(tmpCtx, *msg, nil, false, cfg, txConfig, true, overrides)
		if err != nil {
			if errors.Is(err, core.ErrIntrinsicGas) || errors.Is(err, core.ErrFloorDataGas) {
				return true, nil, nil // Special case, raise gas limit
			}
			return true, nil, err // Bail out
		}
		return len(rsp.VmError) > 0, rsp, nil
	}

	// Adapted from go-ethereum gas estimator for early short-circuit and optimistic bounds:
	// https://github.com/ethereum/go-ethereum/blob/v1.16.2/eth/gasestimator/gasestimator.go

	// If the transaction is a plain value transfer, short circuit estimation and
	// directly try 21000. Returning 21000 without any execution is dangerous as
	// some tx field combos might bump the price up even for plain transfers (e.g.
	// unused access list items). Ever so slightly wasteful, but safer overall.
	if len(msg.Data) == 0 && msg.To != nil {
		acct := k.GetAccountWithoutBalance(ctx, *msg.To)
		if acct == nil || !acct.HasCodeHash() {
			failed, _, err := executable(ethparams.TxGas)
			if err == nil && !failed {
				return &types.EstimateGasResponse{Gas: ethparams.TxGas}, nil
			}
		}
	}

	// We first execute the transaction at the highest allowable gas limit, since if this fails we
	// can return error immediately.
	failed, result, err := executable(hi)
	if err != nil {
		return nil, err
	}
	if failed {
		// Preserve Cosmos error semantics when the cap is reached
		if hi == gasCap {
			if result != nil && result.VmError != vm.ErrOutOfGas.Error() {
				if result.VmError == vm.ErrExecutionReverted.Error() {
					return &types.EstimateGasResponse{
						Ret:     result.Ret,
						VmError: result.VmError,
					}, nil
				}
				return nil, errors.New(result.VmError)
			}
			return nil, fmt.Errorf("gas required exceeds allowance (%d)", gasCap)
		}
		// If no larger allowance is available, fail fast
		return nil, fmt.Errorf("gas required exceeds allowance (%d)", hi)
	}
	// For almost any transaction, the gas consumed by the unconstrained execution
	// above lower-bounds the gas limit required for it to succeed. One exception
	// is those that explicitly check gas remaining in order to execute within a
	// given limit, but we probably don't want to return the lowest possible gas
	// limit for these cases anyway.
	lo = result.GasUsed - 1

	// There's a fairly high chance for the transaction to execute successfully
	// with gasLimit set to the first execution's usedGas + gasRefund. Explicitly
	// check that gas amount and use as a limit for the binary search.
	optimisticGasLimit := (result.MaxUsedGas + ethparams.CallStipend) * 64 / 63
	if optimisticGasLimit < hi {
		failed, _, err = executable(optimisticGasLimit)
		if err != nil {
			return nil, err
		}
		if failed {
			lo = optimisticGasLimit
		} else {
			hi = optimisticGasLimit
		}
	}

	// Binary search for the smallest gas limit that allows the tx to execute successfully.
	hi, err = types.BinSearch(lo, hi, executable)
	if err != nil {
		return nil, err
	}

	return &types.EstimateGasResponse{Gas: hi}, nil
}

// TraceTx configures a new tracer according to the provided configuration, and
// executes the given message in the provided environment. The return value will
// be tracer-dependent.
func (k Keeper) TraceTx(c context.Context, req *types.QueryTraceTxRequest) (*types.QueryTraceTxResponse, error) {
	if req == nil || req.Msg == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if req.TraceConfig != nil && req.TraceConfig.Limit < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "output limit cannot be negative, got %d", req.TraceConfig.Limit)
	}

	if len(req.Predecessors) > maxTracePredecessors {
		return nil, status.Errorf(codes.InvalidArgument, "too many predecessors, got %d: limit %d", len(req.Predecessors), maxTracePredecessors)
	}

	// get the context of block beginning
	requestedHeight := req.BlockNumber
	if requestedHeight < 1 {
		// In Ethereum, the genesis block height is 0, but in CometBFT, the genesis block height is 1.
		// So here we set the minimum requested height to 1.
		requestedHeight = 1
	}

	ctx := sdk.UnwrapSDKContext(c)
	// the caller sets the `ctx.BlockHeight()` to be `requestedHeight - 1`, so we can get the context of block beginning
	if requestedHeight > ctx.BlockHeight()+1 {
		return nil, status.Errorf(codes.FailedPrecondition, "requested height [%d] must be less than or equal to current height [%d]", requestedHeight, ctx.BlockHeight())
	}
	// TODO: ideally, this query should validate that the block hash, predecessor transactions, and main trace tx actually existed in the block requested.
	// These fields should not be defined by a user.
	// For now, since the backend uses this query to run its traces, we cannot do much here. This needs to be refactored
	// so that backend isn't using this method over gRPC, but directly.
	// see: https://linear.app/cosmoslabs/issue/EVM-149/some-xvm-queries-are-meant-to-be-internal
	ctx = ctx.WithBlockHeight(requestedHeight)
	ctx = ctx.WithBlockTime(req.BlockTime)
	ctx = ctx.WithHeaderHash(common.Hex2Bytes(req.BlockHash))

	// to get the base fee we only need the block max gas in the consensus params
	ctx = ctx.WithConsensusParams(tmproto.ConsensusParams{
		Block: &tmproto.BlockParams{MaxGas: req.BlockMaxGas},
	})

	cfg, err := k.EVMConfig(ctx, GetProposerAddress(ctx, req.ProposerAddress))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load evm config: %s", err.Error())
	}

	// compute and use base fee of the height that is being traced
	baseFee := k.feeMarketWrapper.CalculateBaseFee(ctx)
	if baseFee != nil {
		cfg.BaseFee = baseFee
	}

	signer := ethtypes.MakeSigner(types.GetEthChainConfig(), big.NewInt(ctx.BlockHeight()), uint64(ctx.BlockTime().Unix())) //#nosec G115 -- int overflow is not a concern here
	txConfig := statedb.NewEmptyTxConfig()

	// gas used at this point corresponds to GetProposerAddress & CalculateBaseFee
	// need to reset gas meter per transaction to be consistent with tx execution
	// and avoid stacking the gas used of every predecessor in the same gas meter

	ctx = evmante.BuildEvmExecutionCtx(ctx).
		WithGasMeter(storetypes.NewGasMeter(maxPredecessorGas))
	for i, tx := range req.Predecessors {
		ethTx := tx.AsTransaction()
		msg, err := core.TransactionToMessage(ethTx, signer, cfg.BaseFee)
		if err != nil {
			continue
		}
		msg.GasLimit = min(msg.GasLimit, maxPredecessorGas)
		txConfig.TxHash = ethTx.Hash()
		txConfig.TxIndex = uint(i) //nolint:gosec // G115 // won't exceed uint64

		ctx = buildTraceCtx(ctx, msg.GasLimit)
		// we ignore the error here. this endpoint, ideally, is called internally from the ETH backend, which will call this query
		// using all previous txs in the trace transaction's block. some of those _could_ be invalid transactions.
		rsp, _ := k.ApplyMessageWithConfig(ctx, *msg, nil, true, cfg, txConfig, false, nil)
		if rsp != nil {
			ctx.GasMeter().ConsumeGas(rsp.GasUsed, "evm predecessor tx")
		}
	}

	tx := req.Msg.AsTransaction()
	txConfig.TxHash = tx.Hash()
	if len(req.Predecessors) > 0 {
		txConfig.TxIndex++
	}

	result, err := k.traceTx(ctx, cfg, txConfig, signer, tx, req.TraceConfig, false)
	if err != nil {
		// error will be returned with detail status from traceTx
		return nil, err
	}

	resultData, err := json.Marshal(result)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryTraceTxResponse{
		Data: resultData,
	}, nil
}

// TraceBlock configures a new tracer according to the provided configuration, and
// executes the given message in the provided environment for all the transactions in the queried block.
// The return value will be tracer dependent.
func (k Keeper) TraceBlock(c context.Context, req *types.QueryTraceBlockRequest) (*types.QueryTraceBlockResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if req.TraceConfig != nil && req.TraceConfig.Limit < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "output limit cannot be negative, got %d", req.TraceConfig.Limit)
	}

	// get the context of block beginning
	contextHeight := req.BlockNumber
	if contextHeight < 1 {
		// In Ethereum, the genesis block height is 0, but in CometBFT, the genesis block height is 1.
		// So here we set the minimum requested height to 1.
		contextHeight = 1
	}

	ctx := sdk.UnwrapSDKContext(c)
	ctx = ctx.WithBlockHeight(contextHeight)
	ctx = ctx.WithBlockTime(req.BlockTime)
	ctx = ctx.WithHeaderHash(common.Hex2Bytes(req.BlockHash))

	// to get the base fee we only need the block max gas in the consensus params
	ctx = ctx.WithConsensusParams(tmproto.ConsensusParams{
		Block: &tmproto.BlockParams{MaxGas: req.BlockMaxGas},
	})

	cfg, err := k.EVMConfig(ctx, GetProposerAddress(ctx, req.ProposerAddress))
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load evm config")
	}

	// compute and use base fee of height that is being traced
	baseFee := k.feeMarketWrapper.CalculateBaseFee(ctx)
	if baseFee != nil {
		cfg.BaseFee = baseFee
	}

	signer := ethtypes.MakeSigner(types.GetEthChainConfig(), big.NewInt(ctx.BlockHeight()), uint64(ctx.BlockTime().Unix())) //#nosec G115 -- int overflow is not a concern here
	txsLength := len(req.Txs)
	results := make([]*types.TxTraceResult, 0, txsLength)

	txConfig := statedb.NewEmptyTxConfig()

	for i, tx := range req.Txs {
		result := types.TxTraceResult{}
		ethTx := tx.AsTransaction()
		txConfig.TxHash = ethTx.Hash()
		txConfig.TxIndex = uint(i) //nolint:gosec // G115 // won't exceed uint64
		traceResult, err := k.traceTx(ctx, cfg, txConfig, signer, ethTx, req.TraceConfig, true)
		if err != nil {
			result.Error = err.Error()
		} else {
			result.Result = traceResult
		}
		results = append(results, &result)
	}

	resultData, err := json.Marshal(results)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryTraceBlockResponse{
		Data: resultData,
	}, nil
}

// TraceCall configures a new tracer according to the provided configuration, and
// executes the given call in the provided environment. The return value will
// be tracer dependent.
func (k Keeper) TraceCall(c context.Context, req *types.QueryTraceCallRequest) (*types.QueryTraceCallResponse, error) {
	if req == nil || req.Args == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if req.TraceConfig != nil && req.TraceConfig.Limit < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "output limit cannot be negative, got %d", req.TraceConfig.Limit)
	}

	// get the context of block beginning
	requestedHeight := req.BlockNumber
	if requestedHeight < 1 {
		// In Ethereum, the genesis block height is 0, but in CometBFT, the genesis block height is 1.
		// So here we set the minimum requested height to 1.
		requestedHeight = 1
	}

	ctx := sdk.UnwrapSDKContext(c)
	// the caller sets the `ctx.BlockHeight()` to be `requestedHeight - 1`, so we can get the context of block beginning
	if requestedHeight > ctx.BlockHeight()+1 {
		return nil, status.Errorf(codes.FailedPrecondition, "requested height [%d] must be less than or equal to current height [%d]", requestedHeight, ctx.BlockHeight())
	}

	ctx = ctx.WithBlockHeight(requestedHeight)
	ctx = ctx.WithBlockTime(req.BlockTime)
	ctx = ctx.WithHeaderHash(common.Hex2Bytes(req.BlockHash))

	cfg, err := k.EVMConfig(ctx, GetProposerAddress(ctx, req.ProposerAddress))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to load evm config: %s", err.Error())
	}

	// compute and use base fee of the height that is being traced
	baseFee := k.feeMarketWrapper.CalculateBaseFee(ctx)
	if baseFee != nil {
		cfg.BaseFee = baseFee
	}

	// Get empty tx config
	txConfig := statedb.NewEmptyTxConfig()

	// Get transaction msg from args
	var args types.TransactionArgs
	err = json.Unmarshal(req.Args, &args)
	if err != nil {
		return nil, err
	}
	nonce := k.GetNonce(ctx, args.GetFrom())
	args.Nonce = (*hexutil.Uint64)(&nonce)
	// Fill in default values for missing transaction fields (gas, gasPrice, value, etc.)
	if err := args.CallDefaults(req.GasCap, baseFee, types.GetEthChainConfig().ChainID); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	msg := args.ToMessage(baseFee, true, true)

	// trace call
	result, err := k.traceTxWithMsg(ctx, cfg, txConfig, msg, req.GetTraceConfig(), false)
	if err != nil {
		// error will be returned with detail status from traceTx
		return nil, err
	}

	resultData, err := json.Marshal(result)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryTraceCallResponse{
		Data: resultData,
	}, nil
}

// traceTx do trace on one transaction, it returns a tuple: (traceResult, error).
func (k *Keeper) traceTx(
	ctx sdk.Context,
	cfg *statedb.EVMConfig,
	txConfig statedb.TxConfig,
	signer ethtypes.Signer,
	tx *ethtypes.Transaction,
	traceConfig *types.TraceConfig,
	commitMessage bool,
) (*any, error) {
	msg, err := core.TransactionToMessage(tx, signer, cfg.BaseFee)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return k.traceTxWithMsg(ctx, cfg, txConfig, msg, traceConfig, commitMessage)
}

// traceTxWithMsg do trace on one Ethereum message, it returns a tuple: (traceResult, error).
func (k *Keeper) traceTxWithMsg(
	ctx sdk.Context,
	cfg *statedb.EVMConfig,
	txConfig statedb.TxConfig,
	msg *core.Message,
	traceConfig *types.TraceConfig,
	commitMessage bool,
) (*interface{}, error) {
	// Assemble the structured logger or the JavaScript tracer
	var (
		tracer           *tracers.Tracer
		overrides        *ethparams.ChainConfig
		jsonTracerConfig json.RawMessage
		err              error
		timeout          = defaultTraceTimeout
	)

	if traceConfig == nil {
		traceConfig = &types.TraceConfig{}
	}

	if traceConfig != nil && traceConfig.TracerJsonConfig != "" {
		// ignore error. default to no traceConfig
		_ = json.Unmarshal([]byte(traceConfig.TracerJsonConfig), &jsonTracerConfig)
	}

	if traceConfig.Overrides != nil {
		overrides = traceConfig.Overrides.EthereumConfig(types.GetEthChainConfig().ChainID)
	}

	logConfig := logger.Config{
		EnableMemory:     traceConfig.EnableMemory,
		DisableStorage:   traceConfig.DisableStorage,
		DisableStack:     traceConfig.DisableStack,
		EnableReturnData: traceConfig.EnableReturnData,
		Limit:            int(traceConfig.Limit),
		Overrides:        overrides,
	}

	sLogger := logger.NewStructLogger(&logConfig)
	tracer = &tracers.Tracer{
		Hooks:     sLogger.Hooks(),
		GetResult: sLogger.GetResult,
		Stop:      sLogger.Stop,
	}

	tCtx := &tracers.Context{
		BlockHash: common.BytesToHash(ctx.HeaderHash()),
		TxIndex:   int(txConfig.TxIndex), //#nosec G115 -- int overflow is not a concern here
		TxHash:    txConfig.TxHash,
	}

	if traceConfig.Tracer != "" {
		var cfg json.RawMessage
		if traceConfig.TracerJsonConfig != "" {
			cfg = json.RawMessage(traceConfig.TracerJsonConfig)
		}
		if tracer, err = tracers.DefaultDirectory.New(traceConfig.Tracer, tCtx, cfg,
			types.GetEthChainConfig()); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	// Define a meaningful timeout of a single transaction trace
	if traceConfig.Timeout != "" {
		if timeout, err = time.ParseDuration(traceConfig.Timeout); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "timeout value: %s", err.Error())
		}
	}

	// Handle timeouts and RPC cancellations
	deadlineCtx, cancel := context.WithTimeout(ctx.Context(), timeout)
	defer cancel()

	go func() {
		<-deadlineCtx.Done()
		if errors.Is(deadlineCtx.Err(), context.DeadlineExceeded) {
			tracer.Stop(errors.New("execution timeout"))
		}
	}()

	// Build EVM execution context
	ctx = buildTraceCtx(ctx, msg.GasLimit)
	_, err = k.ApplyMessageWithConfig(ctx, *msg, tracer.Hooks, commitMessage, cfg, txConfig, false, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var result interface{}
	result, err = tracer.GetResult()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &result, nil
}

// BaseFee implements the Query/BaseFee gRPC method
func (k Keeper) BaseFee(c context.Context, _ *types.QueryBaseFeeRequest) (*types.QueryBaseFeeResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)

	baseFee := k.GetBaseFee(ctx)

	res := &types.QueryBaseFeeResponse{}
	if baseFee != nil {
		aux := sdkmath.NewIntFromBigInt(baseFee)
		res.BaseFee = &aux
	}

	return res, nil
}

// GlobalMinGasPrice implements the Query/GlobalMinGasPrice gRPC method
func (k Keeper) GlobalMinGasPrice(c context.Context, _ *types.QueryGlobalMinGasPriceRequest) (*types.QueryGlobalMinGasPriceResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	minGasPrice := k.GetMinGasPrice(ctx).TruncateInt()
	return &types.QueryGlobalMinGasPriceResponse{MinGasPrice: minGasPrice}, nil
}

// Config implements the Query/Config gRPC method
func (k Keeper) Config(_ context.Context, _ *types.QueryConfigRequest) (*types.QueryConfigResponse, error) {
	config := types.GetChainConfig()
	config.Denom = types.GetEVMCoinDenom()
	config.Decimals = uint64(types.GetEVMCoinDecimals())

	return &types.QueryConfigResponse{Config: config}, nil
}

// buildTraceCtx builds a context for simulating or tracing transactions by:
// 1. assigning a new infinite gas meter with the provided gasLimit
// 2. calling BuildEvmExecutionCtx to set up gas configs consistent with Ethereum transaction execution.
func buildTraceCtx(ctx sdk.Context, gasLimit uint64) sdk.Context {
	return evmante.BuildEvmExecutionCtx(ctx).
		WithGasMeter(types.NewInfiniteGasMeterWithLimit(gasLimit))
}
