package evm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core"

	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	"github.com/cosmos/evm/utils"
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

	// check that caller has enough balance to cover asset transfer for **topmost** call
	// NOTE: here the gas consumed is from the context with the infinite gas meter
	convertedValue, err := utils.Uint256FromBigInt(msg.Value)
	if err != nil {
		return err
	}
	if msg.Value.Sign() > 0 && evmKeeper.GetAccount(ctx, msg.From).Balance.Cmp(convertedValue) < 0 {
		return errorsmod.Wrapf(
			errortypes.ErrInsufficientFunds,
			"failed to transfer %s from address %s using the EVM block context transfer function",
			msg.Value,
			msg.From,
		)
	}

	return nil
}
