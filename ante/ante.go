package ante

import (
	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"

	errorsmod "cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"
	txsigning "cosmossdk.io/x/tx/signing"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// HandlerOptions defines the list of module keepers required to run the Cosmos EVM
// AnteHandler decorators.
type HandlerOptions struct {
	Cdc                    codec.BinaryCodec
	AccountKeeper          anteinterfaces.AccountKeeper
	BankKeeper             anteinterfaces.BankKeeper
	FeeMarketKeeper        anteinterfaces.FeeMarketKeeper
	EvmKeeper              anteinterfaces.EVMKeeper
	ExtensionOptionChecker ante.ExtensionOptionChecker
	SignModeHandler        *txsigning.HandlerMap
	SigGasConsumer         func(meter storetypes.GasMeter, sig signing.SignatureV2, params authtypes.Params) error
	MaxTxGasWanted         uint64
	TxFeeChecker           ante.TxFeeChecker
	PendingTxListener      PendingTxListener

	// Optional
	IBCKeeper      *ibckeeper.Keeper
	FeegrantKeeper ante.FeegrantKeeper
}

type HandlerOption func(options *HandlerOptions)

func WithIBCKeeper(ibcKeeper *ibckeeper.Keeper) HandlerOption {
	return func(options *HandlerOptions) {
		options.IBCKeeper = ibcKeeper
	}
}

func WithFeegrantKeeper(feegrantKeeper ante.FeegrantKeeper) HandlerOption {
	return func(options *HandlerOptions) {
		options.FeegrantKeeper = feegrantKeeper
	}
}

func CreateHandlerOptions(
	cdc codec.BinaryCodec,
	accountKeeper anteinterfaces.AccountKeeper,
	bankKeeper anteinterfaces.BankKeeper,
	feemarketKeeper anteinterfaces.FeeMarketKeeper,
	evmKeeper anteinterfaces.EVMKeeper,
	extensionOptionChecker ante.ExtensionOptionChecker,
	signModeHandler *txsigning.HandlerMap,
	signatureGasConsumer func(meter storetypes.GasMeter, sig signing.SignatureV2, params authtypes.Params) error,
	maxTxGasWanted uint64,
	txFeeChecker ante.TxFeeChecker,
	pendingTxListener PendingTxListener,
	opts ...HandlerOption,
) (HandlerOptions, error) {
	handlerOptions := HandlerOptions{
		Cdc:                    cdc,
		AccountKeeper:          accountKeeper,
		BankKeeper:             bankKeeper,
		FeeMarketKeeper:        feemarketKeeper,
		EvmKeeper:              evmKeeper,
		ExtensionOptionChecker: extensionOptionChecker,
		SignModeHandler:        signModeHandler,
		SigGasConsumer:         signatureGasConsumer,
		MaxTxGasWanted:         maxTxGasWanted,
		TxFeeChecker:           txFeeChecker,
		PendingTxListener:      pendingTxListener,
		IBCKeeper:              nil,
		FeegrantKeeper:         nil,
	}

	for _, opt := range opts {
		opt(&handlerOptions)
	}

	if err := handlerOptions.Validate(); err != nil {
		return HandlerOptions{}, err
	}

	return handlerOptions, nil

}

// Validate checks if the keepers are defined
func (options HandlerOptions) Validate() error {
	if options.Cdc == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "codec is required for AnteHandler")
	}
	if options.AccountKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "account keeper is required for AnteHandler")
	}
	if options.BankKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "bank keeper is required for AnteHandler")
	}
	if options.FeeMarketKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "fee market keeper is required for AnteHandler")
	}
	if options.EvmKeeper == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "evm keeper is required for AnteHandler")
	}
	if options.SigGasConsumer == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "signature gas consumer is required for AnteHandler")
	}
	if options.SignModeHandler == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "sign mode handler is required for AnteHandler")
	}
	if options.TxFeeChecker == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "tx fee checker is required for AnteHandler")
	}
	if options.PendingTxListener == nil {
		return errorsmod.Wrap(errortypes.ErrLogic, "pending tx listener is required for AnteHandler")
	}

	return nil
}

// NewAnteHandler returns an ante handler responsible for attempting to route an
// Ethereum or SDK transaction to an internal ante handler for performing
// transaction-level processing (e.g. fee payment, signature verification) before
// being passed onto it's respective handler.
func NewAnteHandler(options HandlerOptions) sdk.AnteHandler {
	return func(
		ctx sdk.Context, tx sdk.Tx, sim bool,
	) (newCtx sdk.Context, err error) {
		var anteHandler sdk.AnteHandler

		txWithExtensions, ok := tx.(ante.HasExtensionOptionsTx)
		if ok {
			opts := txWithExtensions.GetExtensionOptions()
			if len(opts) > 0 {
				switch typeURL := opts[0].GetTypeUrl(); typeURL {
				case "/cosmos.evm.vm.v1.ExtensionOptionsEthereumTx":
					// handle as *evmtypes.MsgEthereumTx
					anteHandler = newMonoEVMAnteHandler(options)
				case "/cosmos.evm.types.v1.ExtensionOptionDynamicFeeTx":
					// cosmos-sdk tx with dynamic fee extension
					anteHandler = newCosmosAnteHandler(options)
				default:
					return ctx, errorsmod.Wrapf(
						errortypes.ErrUnknownExtensionOptions,
						"rejecting tx with unsupported extension option: %s", typeURL,
					)
				}

				return anteHandler(ctx, tx, sim)
			}
		}

		// handle as totally normal Cosmos SDK tx
		switch tx.(type) {
		case sdk.Tx:
			anteHandler = newCosmosAnteHandler(options)
		default:
			return ctx, errorsmod.Wrapf(errortypes.ErrUnknownRequest, "invalid transaction type: %T", tx)
		}

		return anteHandler(ctx, tx, sim)
	}
}
