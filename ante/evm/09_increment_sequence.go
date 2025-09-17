package evm

import (
	"math"

	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	"github.com/cosmos/evm/mempool"

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
	accountNonce := account.GetSequence()
	// we merged the accountNonce verification to accountNonce increment, so when tx includes multiple messages
	// with same sender, they'll be accepted.
	if txNonce > accountNonce {
		return errorsmod.Wrapf(
			mempool.ErrNonceGap,
			"tx nonce: %d, account accountNonce: %d", txNonce, accountNonce,
		)
	}
	if txNonce < accountNonce {
		return errorsmod.Wrapf(
			mempool.ErrNonceLow,
			"invalid nonce; got %d, expected %d", txNonce, accountNonce,
		)
	}

	// EIP-2681 / state safety: refuse to overflow beyond 2^64-1.
	if accountNonce == math.MaxUint64 {
		return errorsmod.Wrap(
			errortypes.ErrInvalidSequence,
			"nonce overflow: increment beyond 2^64-1 violates EIP-2681",
		)
	}

	accountNonce++

	if err := account.SetSequence(accountNonce); err != nil {
		return errorsmod.Wrapf(err, "failed to set sequence to %d", accountNonce)
	}

	accountKeeper.SetAccount(ctx, account)
	return nil
}
