package evm

import (
	"math"

	anteinterfaces "github.com/cosmos/evm/ante/interfaces"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

// IncrementNonce increments the sequence of the account.
func IncrementNonce(
	ctx sdk.Context,
	accountKeeper anteinterfaces.AccountKeeper,
	account sdk.AccountI,
	txNonce uint64,
) error {
	nonce := account.GetSequence()
	// we merged the nonce verification to nonce increment, so when tx includes multiple messages
	// with same sender, they'll be accepted.
	if txNonce != nonce {
		return errorsmod.Wrapf(
			errortypes.ErrInvalidSequence,
			"invalid nonce; got %d, expected %d", txNonce, nonce,
		)
	}

	// EIP-2681 / state safety: refuse to overflow beyond 2^64-1.
	if nonce == math.MaxUint64 {
		return errorsmod.Wrap(
			errortypes.ErrInvalidSequence,
			"nonce overflow: increment beyond 2^64-1 violates EIP-2681",
		)
	}

	nonce++

	if err := account.SetSequence(nonce); err != nil {
		return errorsmod.Wrapf(err, "failed to set sequence to %d", nonce)
	}

	accountKeeper.SetAccount(ctx, account)
	return nil
}
