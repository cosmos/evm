package evm

import (
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/txpool"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
)

const AcceptedTxType = 0 |
	1<<ethtypes.LegacyTxType |
	1<<ethtypes.AccessListTxType |
	1<<ethtypes.DynamicFeeTxType |
	1<<ethtypes.SetCodeTxType

// MonoDecorator is a single decorator that handles all the prechecks for
// ethereum transactions.
type MonoDecorator struct {
	accountKeeper   anteinterfaces.AccountKeeper
	feeMarketKeeper anteinterfaces.FeeMarketKeeper
	evmKeeper       anteinterfaces.EVMKeeper
	maxGasWanted    uint64
	evmParams       *evmtypes.Params
	feemarketParams *feemarkettypes.Params
}

// NewEVMMonoDecorator creates the 'mono' decorator, that is used to run the ante handle logic
// for EVM transactions on the chain.
//
// This runs all the default checks for EVM transactions enable through Cosmos EVM.
// Any partner chains can use this in their ante handler logic and build additional EVM
// decorators using the returned DecoratorUtils
func NewEVMMonoDecorator(
	accountKeeper anteinterfaces.AccountKeeper,
	feeMarketKeeper anteinterfaces.FeeMarketKeeper,
	evmKeeper anteinterfaces.EVMKeeper,
	maxGasWanted uint64,
	evmParams *evmtypes.Params,
	feemarketParams *feemarkettypes.Params,
) MonoDecorator {
	return MonoDecorator{
		accountKeeper:   accountKeeper,
		feeMarketKeeper: feeMarketKeeper,
		evmKeeper:       evmKeeper,
		maxGasWanted:    maxGasWanted,
		evmParams:       evmParams,
		feemarketParams: feemarketParams,
	}
}

// AnteHandle handles the entire decorator chain using a mono decorator.
func (md MonoDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	// 0. Basic validation of the transaction
	var txFeeInfo *txtypes.Fee
	if !ctx.IsReCheckTx() {
		// NOTE: txFeeInfo is associated with the Cosmos stack, not the EVM. For
		// this reason, the fee is represented in the original decimals and
		// should be converted later when used.
		txFeeInfo, err = ValidateTx(tx)
		if err != nil {
			return ctx, err
		}
	}

	evmDenom := evmtypes.GetEVMCoinDenom()

	// 1. setup ctx
	ctx, err = SetupContextAndResetTransientGas(ctx, tx)
	if err != nil {
		return ctx, err
	}

	// 2. get utils
	decUtils, err := NewMonoDecoratorUtils(ctx, md.evmKeeper, md.evmParams, md.feemarketParams)
	if err != nil {
		return ctx, err
	}

	// NOTE: the protocol does not support multiple EVM messages currently so
	// this loop will complete after the first message.
	msgs := tx.GetMsgs()
	if len(msgs) != 1 {
		return ctx, errorsmod.Wrapf(errortypes.ErrInvalidRequest, "expected 1 message, got %d", len(msgs))
	}
	msgIndex := 0

	ethMsg, ethTx, err := evmtypes.UnpackEthMsg(msgs[msgIndex])
	if err != nil {
		return ctx, err
	}

	// call go-ethereum transaction validation
	header := ethtypes.Header{
		GasLimit:   ethTx.Gas(),
		BaseFee:    decUtils.BaseFee,
		Number:     big.NewInt(ctx.BlockHeight()),
		Time:       uint64(ctx.BlockTime().Unix()), //nolint:gosec
		Difficulty: big.NewInt(0),
	}

	chainConfig := evmtypes.GetEthChainConfig()

	if err := txpool.ValidateTransaction(ethTx, &header, decUtils.Signer, &txpool.ValidationOptions{
		Config:  chainConfig,
		Accept:  AcceptedTxType,
		MaxSize: math.MaxUint64, // tx size is checked in cometbft
		MinTip:  new(big.Int),
	}); err != nil {
		return ctx, err
	}

	feeAmt := ethMsg.GetFee()
	gas := ethTx.Gas()
	fee := sdkmath.LegacyNewDecFromBigInt(feeAmt)
	gasLimit := sdkmath.LegacyNewDecFromBigInt(new(big.Int).SetUint64(gas))

	// TODO: computation for mempool and global fee can be made using only
	// the price instead of the fee. This would save some computation.
	//
	// 2. mempool inclusion fee
	if ctx.IsCheckTx() && !simulate {
		// FIX: Mempool dec should be converted
		if err := CheckMempoolFee(fee, decUtils.MempoolMinGasPrice, gasLimit, decUtils.Rules.IsLondon); err != nil {
			return ctx, err
		}
	}

	if ethTx.Type() >= ethtypes.DynamicFeeTxType && decUtils.BaseFee != nil {
		// If the base fee is not empty, we compute the effective gas price
		// according to current base fee price. The gas limit is specified
		// by the user, while the price is given by the minimum between the
		// max price paid for the entire tx, and the sum between the price
		// for the tip and the base fee.
		feeAmt = ethMsg.GetEffectiveFee(decUtils.BaseFee)
		fee = sdkmath.LegacyNewDecFromBigInt(feeAmt)
	}

	// 3. min gas price (global min fee)
	if err := CheckGlobalFee(fee, decUtils.GlobalMinGasPrice, gasLimit); err != nil {
		return ctx, err
	}

	// 4. validate msg contents
	if err := ValidateMsg(
		decUtils.EvmParams,
		ethTx,
	); err != nil {
		return ctx, err
	}

	// 5. signature verification
	if err := SignatureVerification(
		ethMsg,
		ethTx,
		decUtils.Signer,
	); err != nil {
		return ctx, err
	}

	from := ethMsg.GetFrom()
	fromAddr := common.BytesToAddress(from)

	// 6. account balance verification
	// We get the account with the balance from the EVM keeper because it is
	// using a wrapper of the bank keeper as a dependency to scale all
	// balances to 18 decimals.
	account := md.evmKeeper.GetAccount(ctx, fromAddr)
	if err := VerifyAccountBalance(
		ctx,
		md.evmKeeper,
		md.accountKeeper,
		account,
		fromAddr,
		ethTx,
	); err != nil {
		return ctx, err
	}

	// 7. can transfer
	coreMsg := ethMsg.AsMessage(decUtils.BaseFee)
	if err := CanTransfer(
		ctx,
		md.evmKeeper,
		*coreMsg,
		decUtils.BaseFee,
		decUtils.EvmParams,
		decUtils.Rules.IsLondon,
	); err != nil {
		return ctx, err
	}

	// 8. gas consumption
	msgFees, err := evmkeeper.VerifyFee(
		ethTx,
		evmDenom,
		decUtils.BaseFee,
		decUtils.Rules.IsHomestead,
		decUtils.Rules.IsIstanbul,
		decUtils.Rules.IsShanghai,
		ctx.IsCheckTx(),
	)
	if err != nil {
		return ctx, err
	}

	err = ConsumeFeesAndEmitEvent(
		ctx,
		md.evmKeeper,
		msgFees,
		from,
	)
	if err != nil {
		return ctx, err
	}

	gasWanted := UpdateCumulativeGasWanted(
		ctx,
		gas,
		md.maxGasWanted,
		decUtils.GasWanted,
	)
	decUtils.GasWanted = gasWanted

	minPriority := GetMsgPriority(
		ethTx,
		decUtils.MinPriority,
		decUtils.BaseFee,
	)
	decUtils.MinPriority = minPriority

	// Update the fee to be paid for the tx adding the fee specified for the
	// current message.
	decUtils.TxFee.Add(decUtils.TxFee, ethMsg.GetFee())

	// Update the transaction gas limit adding the gas specified in the
	// current message.
	decUtils.TxGasLimit += gas

	// 9. increment sequence
	acc := md.accountKeeper.GetAccount(ctx, from)
	if acc == nil {
		// safety check: shouldn't happen
		return ctx, errorsmod.Wrapf(
			errortypes.ErrUnknownAddress,
			"account %s does not exist",
			from,
		)
	}

	if err := IncrementNonce(ctx, md.accountKeeper, acc, ethTx.Nonce()); err != nil {
		return ctx, err
	}

	// Emit event unconditionally - ctx.TxIndex() will be valid during block execution
	EmitTxHashEvent(ctx, ethMsg, uint64(ctx.TxIndex())) // #nosec G115 -- no overlfow here

	if err := CheckTxFee(txFeeInfo, decUtils.TxFee, decUtils.TxGasLimit); err != nil {
		return ctx, err
	}

	ctx, err = CheckBlockGasLimit(ctx, decUtils.GasWanted, decUtils.MinPriority)
	if err != nil {
		return ctx, err
	}

	return next(ctx, tx, simulate)
}
