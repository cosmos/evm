package keeper

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	evmtrace "github.com/cosmos/evm/trace"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EVMConfig creates the EVMConfig based on current state
func (k *Keeper) EVMConfig(ctx sdk.Context, proposerAddress sdk.ConsAddress) (_ *statedb.EVMConfig, err error) {
	ctx, span := ctx.StartSpan(tracer, "EVMConfig", trace.WithAttributes(attribute.String("proposer", proposerAddress.String())))
	defer func() { evmtrace.EndSpanErr(span, err) }()
	params := k.GetParams(ctx)
	feemarketParams := k.feeMarketWrapper.GetParams(ctx)

	// get the coinbase address from the block proposer
	coinbase, err := k.GetCoinbaseAddress(ctx, proposerAddress)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to obtain coinbase address")
	}

	baseFee := k.GetBaseFee(ctx)

	return &statedb.EVMConfig{
		Params:          params,
		FeeMarketParams: feemarketParams,
		CoinBase:        coinbase,
		BaseFee:         baseFee,
	}, nil
}

// TxConfig loads `TxConfig` from current transient storage
func (k *Keeper) TxConfig(ctx sdk.Context, txHash common.Hash) statedb.TxConfig {
	return statedb.NewTxConfig(
		txHash,              // TxHash
		uint(ctx.TxIndex()), //#nosec G115 // TxIndex
	)
}

// VMConfig creates an EVM configuration from the debug setting and the extra EIPs enabled on the
// module parameters. The config generated uses the default JumpTable from the EVM.
func (k Keeper) VMConfig(ctx sdk.Context, _ core.Message, cfg *statedb.EVMConfig, tracingHooks *tracing.Hooks) vm.Config {
	ctx, span := ctx.StartSpan(tracer, "VMConfig", trace.WithAttributes(
		attribute.Bool("enable_preimage_recording", cfg.EnablePreimageRecording),
		attribute.Int("extra_eips_count", len(cfg.Params.ExtraEIPs)),
	))
	defer span.End()
	noBaseFee := true
	if types.IsLondon(types.GetEthChainConfig(), ctx.BlockHeight()) {
		noBaseFee = cfg.FeeMarketParams.NoBaseFee
	}

	return vm.Config{
		EnablePreimageRecording: cfg.EnablePreimageRecording,
		Tracer:                  tracingHooks,
		NoBaseFee:               noBaseFee,
		ExtraEips:               cfg.Params.EIPs(),
	}
}
