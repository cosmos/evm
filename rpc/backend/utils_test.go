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
	cases := []string{
		legacypool.ErrOutOfOrderTxFromDelegated.Error(),
		txpool.ErrInflightTxLimitReached.Error(),
		legacypool.ErrAuthorityReserved.Error(),
		txpool.ErrUnderpriced.Error(),
		legacypool.ErrTxPoolOverflow.Error(),
		legacypool.ErrFutureReplacePending.Error(),
		mempool.ErrNonceGap.Error(),
	}
	for _, rawLog := range cases {
		require.True(t, HandleBroadcastRawLog(rawLog, txHash), "expected true for: %s", rawLog)
	}
	require.False(t, HandleBroadcastRawLog("some other error", txHash))
}
