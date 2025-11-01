package keeper

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EVMConfig creates the EVMConfig based on current state
func (k *Keeper) EVMConfig(ctx sdk.Context, proposerAddress sdk.ConsAddress) (*statedb.EVMConfig, error) {
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
func (k Keeper) VMConfig(ctx sdk.Context, _ core.Message, cfg *statedb.EVMConfig, tracer *tracing.Hooks) vm.Config {
	noBaseFee := true
	if types.IsLondon(types.GetEthChainConfig(), ctx.BlockHeight()) {
		noBaseFee = cfg.FeeMarketParams.NoBaseFee
	}

	return vm.Config{
		EnablePreimageRecording: cfg.EnablePreimageRecording,
		Tracer:                  tracer,
		NoBaseFee:               noBaseFee,
		ExtraEips:               cfg.Params.EIPs(),
	}
}
