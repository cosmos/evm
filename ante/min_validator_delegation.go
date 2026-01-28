package ante

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// EpixMintKeeper defines the expected interface for the epixmint keeper
type EpixMintKeeper interface {
	GetMinValidatorSelfDelegation(ctx context.Context) math.Int
}

// MinValidatorSelfDelegationDecorator checks that MsgCreateValidator transactions
// meet the network-wide minimum self-delegation requirement
type MinValidatorSelfDelegationDecorator struct {
	epixMintKeeper EpixMintKeeper
}

// NewMinValidatorSelfDelegationDecorator creates a new MinValidatorSelfDelegationDecorator
func NewMinValidatorSelfDelegationDecorator(epixMintKeeper EpixMintKeeper) MinValidatorSelfDelegationDecorator {
	return MinValidatorSelfDelegationDecorator{
		epixMintKeeper: epixMintKeeper,
	}
}

// AnteHandle checks if MsgCreateValidator meets minimum self-delegation requirements
func (mvsd MinValidatorSelfDelegationDecorator) AnteHandle(
	ctx sdk.Context,
	tx sdk.Tx,
	simulate bool,
	next sdk.AnteHandler,
) (sdk.Context, error) {
	// Get the minimum self-delegation from epixmint params
	minSelfDelegation := mvsd.epixMintKeeper.GetMinValidatorSelfDelegation(ctx)

	// Check each message in the transaction
	for _, msg := range tx.GetMsgs() {
		// Only check MsgCreateValidator messages
		createValMsg, ok := msg.(*stakingtypes.MsgCreateValidator)
		if !ok {
			continue
		}

		// Check if the self-delegation amount meets the minimum requirement
		if createValMsg.Value.Amount.LT(minSelfDelegation) {
			return ctx, errorsmod.Wrapf(
				errortypes.ErrInsufficientFunds,
				"validator self-delegation of %s is below the network minimum of %s",
				createValMsg.Value.Amount.String(),
				minSelfDelegation.String(),
			)
		}

		// Also enforce the per-validator min_self_delegation field to be at least the network minimum
		if createValMsg.MinSelfDelegation.LT(minSelfDelegation) {
			return ctx, errorsmod.Wrapf(
				errortypes.ErrInvalidRequest,
				"validator min_self_delegation of %s must be at least the network minimum of %s",
				createValMsg.MinSelfDelegation.String(),
				minSelfDelegation.String(),
			)
		}
	}

	return next(ctx, tx, simulate)
}
