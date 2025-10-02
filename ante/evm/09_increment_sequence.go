package evm

import (
	"math"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/evm/mempool"
)

// IncrementNonce increments the sequence of the account.
func (md MonoDecorator) IncrementNonce(
	ctx sdk.Context,
	account sdk.AccountI,
	tx sdk.Tx,
	txNonce uint64,
) error {
	utx, ok := tx.(sdk.TxWithUnordered)
	isUnordered := ok && utx.GetUnordered()
	unorderedEnabled := md.accountKeeper.UnorderedTransactionsEnabled()

	if isUnordered && !unorderedEnabled {
		return errorsmod.Wrap(sdkerrors.ErrNotSupported, "unordered transactions are not enabled")
	}

	accountNonce := account.GetSequence()

	if isUnordered {
		if err := md.verifyUnorderedNonce(ctx, account, utx); err != nil {
			return err
		}
	} else {
		// We've merged the accountNonce verification to accountNonce increment, so
		// when tx includes multiple messages with same sender, they'll be accepted.
		if txNonce > accountNonce {
			return errorsmod.Wrapf(
				mempool.ErrNonceGap,
				"tx nonce: %d, account accountNonce: %d", txNonce, accountNonce,
			)
		}

		if txNonce < accountNonce {
			return errorsmod.Wrapf(
				errortypes.ErrInvalidSequence,
				"invalid nonce; got %d, expected %d", txNonce, accountNonce,
			)
		}
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

	md.accountKeeper.SetAccount(ctx, account)
	return nil
}

// verifyUnorderedNonce verifies the unordered nonce of an unordered transaction.
// This checks that:
// 1. The unordered transaction's timeout timestamp is set.
// 2. The unordered transaction's timeout timestamp is not in the past.
// 3. The unordered transaction's timeout timestamp is not more than the max TTL.
// 4. The unordered transaction's nonce has not been used previously.
//
// If all the checks above pass, the nonce is marked as used for each signer of
// the transaction.
func (md MonoDecorator) verifyUnorderedNonce(ctx sdk.Context, account sdk.AccountI, unorderedTx sdk.TxWithUnordered) error {
	blockTime := ctx.BlockTime()
	timeoutTimestamp := unorderedTx.GetTimeoutTimeStamp()

	if timeoutTimestamp.IsZero() || timeoutTimestamp.Unix() == 0 {
		return errorsmod.Wrap(
			sdkerrors.ErrInvalidRequest,
			"unordered transaction must have timeout_timestamp set",
		)
	}

	if timeoutTimestamp.Before(blockTime) {
		return errorsmod.Wrap(
			sdkerrors.ErrInvalidRequest,
			"unordered transaction has a timeout_timestamp that has already passed",
		)
	}

	if timeoutTimestamp.After(blockTime.Add(md.maxTxTimeoutDuration)) {
		return errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"unordered tx ttl exceeds %s",
			md.maxTxTimeoutDuration.String(),
		)
	}

	ctx.GasMeter().ConsumeGas(md.unorderedTxGasCost, "unordered tx")

	execMode := ctx.ExecMode()
	if execMode == sdk.ExecModeSimulate {
		return nil
	}

	err := md.accountKeeper.TryAddUnorderedNonce(
		ctx,
		account.GetAddress().Bytes(),
		unorderedTx.GetTimeoutTimeStamp(),
	)
	if err != nil {
		return errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "failed to add unordered nonce: %s", err)
	}

	return nil
}
