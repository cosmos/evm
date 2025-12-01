package evm

import (
	"math"
	"math/big"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"

	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DecoratorUtils contain a bunch of relevant variables used for a variety of checks
// throughout the verification of an Ethereum transaction.
type DecoratorUtils struct {
	EvmParams          evmtypes.Params
	Rules              params.Rules
	Signer             ethtypes.Signer
	BaseFee            *big.Int
	MempoolMinGasPrice sdkmath.LegacyDec
	GlobalMinGasPrice  sdkmath.LegacyDec
	TxGasLimit         uint64
	GasWanted          uint64
	MinPriority        int64
	TxFee              *big.Int
}

// NewMonoDecoratorUtils returns a new DecoratorUtils instance.
//
// These utilities are extracted once at the beginning of the ante handle process,
// and are used throughout the entire decorator chain.
// This avoids redundant calls to the keeper and thus improves speed of transaction processing.
//
// All prices, fees and balances are converted into 18 decimals here
// to be correctly used in the EVM.
func NewMonoDecoratorUtils(
	ctx sdk.Context,
	ek anteinterfaces.EVMKeeper,
	evmParams *evmtypes.Params,
	feemarketParams *feemarkettypes.Params,
) (*DecoratorUtils, error) {
	ethCfg := evmtypes.GetEthChainConfig()
	evmDenom := evmtypes.GetEVMCoinDenom()
	blockHeight := big.NewInt(ctx.BlockHeight())
	rules := ethCfg.Rules(blockHeight, true, uint64(ctx.BlockTime().Unix())) //#nosec G115 -- int overflow is not a concern here
	baseFee := evmtypes.GetBaseFee(ctx.BlockHeight(), ethCfg, feemarketParams)

	if rules.IsLondon && baseFee == nil {
		return nil, errorsmod.Wrap(
			evmtypes.ErrInvalidBaseFee,
			"base fee is supported but evm block context value is nil",
		)
	}

	globalMinGasPrice := feemarketParams.MinGasPrice

	// Mempool gas price should be scaled to the 18 decimals representation.
	// If it is already a 18 decimal token, this is a no-op.
	mempoolMinGasPrice := evmtypes.ConvertAmountTo18DecimalsLegacy(ctx.MinGasPrices().AmountOf(evmDenom))

	return &DecoratorUtils{
		EvmParams:          *evmParams,
		Rules:              rules,
		Signer:             ethtypes.MakeSigner(ethCfg, blockHeight, uint64(ctx.BlockTime().Unix())), //#nosec G115 -- int overflow is not a concern here
		BaseFee:            baseFee,
		MempoolMinGasPrice: mempoolMinGasPrice,
		GlobalMinGasPrice:  globalMinGasPrice,
		GasWanted:          0,
		MinPriority:        int64(math.MaxInt64),
		// TxGasLimit and TxFee are set to zero because they are updated
		// summing up the values of all messages contained in a tx.
		TxGasLimit: 0,
		TxFee:      big.NewInt(0),
	}, nil
}
