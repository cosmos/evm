package locals

import (
	"errors"

	"github.com/cosmos/evm/mempool/txpool"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
)

// IsTemporaryReject determines whether the given error indicates a temporary
// reason to reject a transaction from being included in the txpool. The result
// may change if the txpool's state changes later.
func IsTemporaryReject(err error) bool {
	switch {
	case errors.Is(err, legacypool.ErrOutOfOrderTxFromDelegated):
		return true
	case errors.Is(err, txpool.ErrInflightTxLimitReached):
		return true
	case errors.Is(err, legacypool.ErrAuthorityReserved):
		return true
	case errors.Is(err, txpool.ErrUnderpriced):
		return true
	case errors.Is(err, legacypool.ErrTxPoolOverflow):
		return true
	case errors.Is(err, legacypool.ErrFutureReplacePending):
		return true
	default:
		return false
	}
}
