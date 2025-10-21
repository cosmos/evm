package evm

import (
	"math/big"

	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	"github.com/cosmos/evm/ante/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

// GasWantedDecorator keeps track of the gasWanted amount on the current block in transient store
// for BaseFee calculation.
// NOTE: This decorator does not perform any validation
type GasWantedDecorator struct {
	evmKeeper       anteinterfaces.EVMKeeper
	feeMarketKeeper anteinterfaces.FeeMarketKeeper
	feemarketParams *feemarkettypes.Params
}

// NewGasWantedDecorator creates a new NewGasWantedDecorator
func NewGasWantedDecorator(
	evmKeeper anteinterfaces.EVMKeeper,
	feeMarketKeeper anteinterfaces.FeeMarketKeeper,
	feemarketParams *feemarkettypes.Params,
) GasWantedDecorator {
	return GasWantedDecorator{
		evmKeeper,
		feeMarketKeeper,
		feemarketParams,
	}
}

func (gwd GasWantedDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	ethCfg := evmtypes.GetEthChainConfig()

	blockHeight := big.NewInt(ctx.BlockHeight())
	isLondon := ethCfg.IsLondon(blockHeight)

	if err := CheckGasWanted(ctx, gwd.feeMarketKeeper, tx, isLondon, gwd.feemarketParams); err != nil {
		return ctx, err
	}

	return next(ctx, tx, simulate)
}

func CheckGasWanted(ctx sdk.Context, feeMarketKeeper anteinterfaces.FeeMarketKeeper, tx sdk.Tx, isLondon bool, feemarketParams *feemarkettypes.Params) error {
	if !isLondon {
		return nil
	}

	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return nil
	}

	gasWanted := feeTx.GetGas()

	// return error if the tx gas is greater than the block limit (max gas)
	blockGasLimit := types.BlockGasLimit(ctx)
	if gasWanted > blockGasLimit {
		return errorsmod.Wrapf(
			errortypes.ErrOutOfGas,
			"tx gas (%d) exceeds block gas limit (%d)",
			gasWanted,
			blockGasLimit,
		)
	}

	if !feemarketParams.IsBaseFeeEnabled(ctx.BlockHeight()) {
		return nil
	}

	// Add total gasWanted to cumulative in block transientStore in FeeMarket module
	if _, err := feeMarketKeeper.AddTransientGasWanted(ctx, gasWanted); err != nil {
		return errorsmod.Wrapf(err, "failed to add gas wanted to transient store")
	}

	return nil
}
