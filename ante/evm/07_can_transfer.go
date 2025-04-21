package evm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"

	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

// CanTransfer checks if the sender is allowed to transfer funds according to the EVM block
func CanTransfer(
	ctx sdk.Context,
	evmKeeper anteinterfaces.EVMKeeper,
	msg core.Message,
	baseFee *big.Int,
	ethCfg *params.ChainConfig,
	params evmtypes.Params,
	isLondon bool,
) error {
	if isLondon && msg.GasFeeCap.Cmp(baseFee) < 0 {
		return errorsmod.Wrapf(
			errortypes.ErrInsufficientFee,
			"max fee per gas less than block base fee (%s < %s)",
			msg.GasFeeCap, baseFee,
		)
	}

	// NOTE: pass in an empty coinbase address and nil tracer as we don't need them for the check below
	cfg := &statedb.EVMConfig{
		ChainConfig: ethCfg,
		Params:      params,
		CoinBase:    common.Address{},
		BaseFee:     baseFee,
	}

	stateDB := statedb.New(ctx, evmKeeper, statedb.NewEmptyTxConfig(common.BytesToHash(ctx.HeaderHash())))
	evm := evmKeeper.NewEVM(ctx, msg, cfg, nil, stateDB)

	// check that caller has enough balance to cover asset transfer for **topmost** call
	// NOTE: here the gas consumed is from the context with the infinite gas meter
	if msg.Value.Sign() > 0 && !evm.Context.CanTransfer(stateDB, msg.From, uint256.MustFromBig(msg.Value)) {
		return errorsmod.Wrapf(
			errortypes.ErrInsufficientFunds,
			"failed to transfer %s from address %s using the EVM block context transfer function",
			msg.Value,
			msg.From,
		)
	}

	return nil
}
