package backend

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/txpool/legacypool"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/mempool/txpool"
)

func TestHandleBroadcastRawLog(t *testing.T) {
	txHash := common.HexToHash("0x123")
	tmpErrMsg := "transaction temporarily rejected or queued"
	cases := []struct {
		rawLog  string
		wantMsg string
	}{
		{legacypool.ErrOutOfOrderTxFromDelegated.Error(), tmpErrMsg},
		{txpool.ErrInflightTxLimitReached.Error(), tmpErrMsg},
		{legacypool.ErrAuthorityReserved.Error(), tmpErrMsg},
		{txpool.ErrUnderpriced.Error(), tmpErrMsg},
		{legacypool.ErrTxPoolOverflow.Error(), tmpErrMsg},
		{legacypool.ErrFutureReplacePending.Error(), tmpErrMsg},
		{mempool.ErrNonceGap.Error(), "transaction queued due to nonce gap"},
	}

	for _, tc := range cases {
		ok, msg := HandleBroadcastRawLog(tc.rawLog, txHash)
		require.True(t, ok, "expected true for: %s", tc.rawLog)
		require.Equal(t, tc.wantMsg, msg, "unexpected message for: %s", tc.rawLog)
	}

	ok, msg := HandleBroadcastRawLog("some other error", txHash)
	require.False(t, ok)
	require.Equal(t, "", msg)
}
