package evm_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/cosmos/evm/ante/evm"

	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestEthSigVerificationDecorator_CacheHit(t *testing.T) {
	dec := evm.NewEthSigVerificationDecorator(nil)
	var tx sdk.Tx
	newCtx := func() sdk.Context {
		return sdk.NewContext(nil, tmproto.Header{}, false, log.NewNopLogger()).
			WithIncarnationCache(map[string]any{})
	}
	next := func(c sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) { return c, nil }
	cachedErr := errors.New("cached sig verification failure")

	t.Run("cached error short-circuits", func(t *testing.T) {
		ctx := newCtx()
		ctx.SetIncarnationCache(evm.EthSigVerificationResultCacheKey, cachedErr)
		_, err := dec.AnteHandle(ctx, tx, true, next)
		require.ErrorIs(t, err, cachedErr)
	})

	t.Run("non-error cached value returns explicit error", func(t *testing.T) {
		ctx := newCtx()
		ctx.SetIncarnationCache(evm.EthSigVerificationResultCacheKey, "not-an-error")
		_, err := dec.AnteHandle(ctx, tx, true, next)
		require.ErrorContains(t, err, "unexpected type string")
	})

	t.Run("cached nil success calls next without re-verifying", func(t *testing.T) {
		ctx := newCtx()
		ctx.SetIncarnationCache(evm.EthSigVerificationResultCacheKey, nil)

		nextCalled := false
		_, err := dec.AnteHandle(ctx, tx, true, func(c sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
			nextCalled = true
			return c, nil
		})
		require.NoError(t, err)
		require.True(t, nextCalled, "next handler must run when cache holds a nil success")
	})
}
