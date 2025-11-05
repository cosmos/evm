package locals

import (
	"errors"

	"github.com/cosmos/evm/mempool/txpool"
	"github.com/ethereum/go-ethereum/core/txpool/legacypool"
)

var (
	// ErrNonceGap is returned if the tx nonce is higher than the account nonce.
	// This is a duplicate of mempool.ErrNonceGap to avoid import cycle.
	ErrNonceGap = errors.New("tx nonce is higher than account nonce")
)

// IsTemporaryReject determines whether the given error indicates a temporary reason to reject a
// transaction from being included in the txpool. The result may change if the txpool's state changes later.
// We use strings.Contains instead of errors.Is because we are passing in rawLog errors.
func IsTemporaryReject(err error) bool {
	if err == nil {
		return false
	}

	switch err.Error() {
	case legacypool.ErrOutOfOrderTxFromDelegated.Error(),
		txpool.ErrInflightTxLimitReached.Error(),
		legacypool.ErrAuthorityReserved.Error(),
		txpool.ErrUnderpriced.Error(),
		legacypool.ErrTxPoolOverflow.Error(),
		legacypool.ErrFutureReplacePending.Error(),
		ErrNonceGap.Error():
		return true
	}

	return false
}
