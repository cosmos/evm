package locals

import (
	"errors"
	"testing"

	"github.com/cosmos/evm/mempool/txpool"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
)

func TestIsTemporaryReject_PositiveCases(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "delegated out-of-order nonce", err: legacypool.ErrOutOfOrderTxFromDelegated},
		{name: "inflight tx limit reached", err: txpool.ErrInflightTxLimitReached},
		{name: "authority reserved", err: legacypool.ErrAuthorityReserved},
		{name: "underpriced", err: txpool.ErrUnderpriced},
		{name: "txpool overflow", err: legacypool.ErrTxPoolOverflow},
		{name: "future replace pending", err: legacypool.ErrFutureReplacePending},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !IsTemporaryReject(tc.err) {
				t.Fatalf("expected temporary reject error to be detected, got false: %v", tc.err)
			}
		})
	}
}

func TestIsTemporaryReject_NegativeCases(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "nil", err: nil},
		{name: "unrelated", err: errors.New("some unrelated error")},
		{name: "substring lookalike", err: errors.New("under price threshold")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if IsTemporaryReject(tc.err) {
				t.Fatalf("did not expect temporary reject error for: %v", tc.err)
			}
		})
	}
}
