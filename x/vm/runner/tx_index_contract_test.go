package runner_test

import (
	"context"
	"sync"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/baseapp/txnrunner"
	"github.com/cosmos/cosmos-sdk/store/v2"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Guards the SDK TxRunner contract x/vm relies on: deliverTx's txIndex must
// match the tx's slice position (BaseApp forwards it to ctx.TxIndex()).

func newTestMultiStore(t *testing.T, key storetypes.StoreKey) storetypes.MultiStore {
	t.Helper()
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db, log.NewNopLogger())
	cms.MountStoreWithDB(key, storetypes.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	return cms
}

func noopTxDecoder(_ []byte) (sdk.Tx, error) { return nil, nil }

// DefaultRunner: txIndex == slice position, exactly once per tx.
func TestDefaultRunner_PreservesBlockOrderIndex(t *testing.T) {
	r := txnrunner.NewDefaultRunner(noopTxDecoder)

	const n = 16
	txs := make([][]byte, n)
	for i := range txs {
		txs[i] = []byte{byte(i)}
	}

	seen := make([]int, n)
	deliverTx := func(tx []byte, _ sdk.Tx, _ storetypes.MultiStore, txIndex int, _ map[string]any) *abci.ExecTxResult {
		require.Equalf(t, int(tx[0]), txIndex,
			"DefaultRunner must pass original block-order index; tx payload %d got txIndex %d", tx[0], txIndex)
		seen[txIndex]++
		return &abci.ExecTxResult{Code: 0}
	}

	results, err := r.Run(context.Background(), nil, txs, deliverTx)
	require.NoError(t, err)
	require.Len(t, results, n)
	for i, c := range seen {
		require.Equalf(t, 1, c, "tx %d executed %d times, want exactly 1", i, c)
	}
}

// STMRunner: txIndex == slice position under parallel execution and
// incarnation retries.
func TestSTMRunner_PreservesBlockOrderIndex(t *testing.T) {
	key := storetypes.NewKVStoreKey("test")
	ms := newTestMultiStore(t, key)

	coinDenom := func(_ storetypes.MultiStore) string { return "stake" }
	r := txnrunner.NewSTMRunner(
		noopTxDecoder,
		[]storetypes.StoreKey{key},
		4,
		false,
		coinDenom,
	)

	const n = 32
	txs := make([][]byte, n)
	for i := range txs {
		txs[i] = []byte{byte(i)}
	}

	var (
		mu   sync.Mutex
		runs = make(map[int]int, n)
	)
	deliverTx := func(tx []byte, _ sdk.Tx, _ storetypes.MultiStore, txIndex int, _ map[string]any) *abci.ExecTxResult {
		require.Equalf(t, int(tx[0]), txIndex,
			"STMRunner must pass original block-order index; tx payload %d got txIndex %d", tx[0], txIndex)
		mu.Lock()
		runs[txIndex]++
		mu.Unlock()
		return &abci.ExecTxResult{Code: 0}
	}

	results, err := r.Run(context.Background(), ms, txs, deliverTx)
	require.NoError(t, err)
	require.Len(t, results, n)

	for i := 0; i < n; i++ {
		require.GreaterOrEqualf(t, runs[i], 1, "tx %d never executed under STM", i)
	}
}
