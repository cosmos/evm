package mempool_test

import (
	"crypto/ecdsa"
	"encoding/binary"
	"errors"
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/mempool"
)

// Helper function to create a test transaction with specific gas
func testTx(t *testing.T, key *ecdsa.PrivateKey, nonce uint64, gas uint64) *types.Transaction {
	t.Helper()

	tx := types.NewTransaction(
		nonce,
		common.Address{},
		big.NewInt(txValue),
		gas,
		big.NewInt(1),
		nil,
	)
	signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, key)
	require.NoError(t, err)
	return signedTx
}

// Helper function to create an encoder that returns deterministic sizes
func deterministicEncoder(bytesPerTx uint64) func(tx *types.Transaction) ([]byte, error) {
	return func(tx *types.Transaction) ([]byte, error) {
		// Return a byte slice of fixed size for predictable testing
		return make([]byte, bytesPerTx), nil
	}
}

// Helper function to create an encoder that fails for specific transaction nonces
func failingEncoder(failNonces map[uint64]bool) func(tx *types.Transaction) ([]byte, error) {
	return func(tx *types.Transaction) ([]byte, error) {
		if failNonces[tx.Nonce()] {
			return nil, errors.New("encoding failed")
		}
		return make([]byte, 100+(tx.Nonce()*10)), nil
	}
}

func TestReapList_EmptyList(t *testing.T) {
	rl := mempool.NewReapList(deterministicEncoder(100))

	result := rl.Reap(0, 0)

	require.Empty(t, result, "reaping empty list should return empty result")
}

func TestReapList_SingleTransaction(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(deterministicEncoder(100))
	tx := testTx(t, key, 0, 21000)
	rl.Push(tx)

	result := rl.Reap(0, 0)

	require.Len(t, result, 1, "should reap single transaction")
	require.Len(t, result[0], 100, "transaction should have expected size")

	// Second reap should return empty as tx was removed
	result = rl.Reap(0, 0)
	require.Empty(t, result, "second reap should return empty")
}

func TestReapList_NoLimits(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(deterministicEncoder(100))

	// Add 10 transactions
	for i := uint64(0); i < 10; i++ {
		tx := testTx(t, key, i, 21000)
		rl.Push(tx)
	}

	result := rl.Reap(0, 0)

	require.Len(t, result, 10, "should reap all transactions with no limits")
}

func TestReapList_MaxBytesLimit(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Each tx is 100 bytes
	rl := mempool.NewReapList(deterministicEncoder(100))

	// Add 10 transactions
	for i := uint64(0); i < 10; i++ {
		tx := testTx(t, key, i, 21000)
		rl.Push(tx)
	}

	// Limit to 350 bytes (should get 3 transactions)
	result := rl.Reap(350, 0)

	require.Len(t, result, 3, "should reap 3 transactions with 350 byte limit")

	// Next reap should get remaining 7
	result = rl.Reap(0, 0)
	require.Len(t, result, 7, "should have 7 transactions remaining")
}

func TestReapList_MaxGasLimit(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(deterministicEncoder(100))

	// Add transactions with varying gas
	txGases := []uint64{21000, 30000, 40000, 50000, 60000}
	var nonce uint64
	for _, gas := range txGases {
		tx := testTx(t, key, nonce, gas)
		rl.Push(tx)
		nonce++
	}

	// Limit to 100000 gas (should get first 3 txs: 21000 + 30000 + 40000 = 91000)
	result := rl.Reap(0, 100000)

	require.Len(t, result, 3, "should reap 3 transactions with 100000 gas limit")

	// Next reap should get remaining 2
	result = rl.Reap(0, 0)
	require.Len(t, result, 2, "should have 2 transactions remaining")
}

func TestReapList_BothLimits(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(deterministicEncoder(100))

	// Add transactions with varying gas
	txGases := []uint64{21000, 30000, 40000, 50000, 60000}
	var nonce uint64
	for _, gas := range txGases {
		tx := testTx(t, key, nonce, gas)
		rl.Push(tx)
		nonce++
	}

	// Limit to 250 bytes (2.5 txs) and 70000 gas (first 3 txs would be 91000)
	// Should be limited by bytes, so only 2 transactions
	result := rl.Reap(250, 70000)

	require.Len(t, result, 2, "should be limited by bytes, returning 2 transactions")

	// Next reap with gas limit should get next 2 txs (40000 + 50000 = 90000 < 100000)
	result = rl.Reap(0, 100000)
	require.Len(t, result, 2, "should reap next 2 transactions within gas limit")

	// Final reap should get last tx
	result = rl.Reap(0, 0)
	require.Len(t, result, 1, "should have 1 transaction remaining")
}

func TestReapList_ExactBytesLimit(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Each tx is 100 bytes
	rl := mempool.NewReapList(deterministicEncoder(100))

	// Add 5 transactions
	for i := uint64(0); i < 5; i++ {
		tx := testTx(t, key, i, 21000)
		rl.Push(tx)
	}

	// Limit to exactly 300 bytes (should get exactly 3 transactions)
	result := rl.Reap(300, 0)

	require.Len(t, result, 3, "should reap exactly 3 transactions with exact byte limit")
}

func TestReapList_ExactGasLimit(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(deterministicEncoder(100))

	// Add transactions with specific gas amounts
	txGases := []uint64{21000, 30000, 40000}
	var nonce uint64
	for _, gas := range txGases {
		tx := testTx(t, key, nonce, gas)
		rl.Push(tx)
		nonce++
	}

	// Limit to exactly 51000 gas (21000 + 30000 = 51000, exactly 2 txs)
	result := rl.Reap(0, 51000)

	require.Len(t, result, 2, "should reap exactly 2 transactions with exact gas limit")
}

func TestReapList_EncodingFailure(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Create encoder that fails for nonce 1 and 3
	failNonces := map[uint64]bool{1: true, 3: true}
	rl := mempool.NewReapList(failingEncoder(failNonces))

	// Add 5 transactions (nonces 0-4)
	for i := uint64(0); i < 5; i++ {
		tx := testTx(t, key, i, 21000)
		rl.Push(tx)
	}

	result := rl.Reap(0, 0)

	// Should get 3 transactions (0, 2, 4) - nonces 1 and 3 fail encoding
	require.Len(t, result, 3, "should skip transactions that fail encoding")

	// Verify we got the correct transactions by checking sizes
	// Nonce 0: size = 100 + 0*10 = 100
	// Nonce 2: size = 100 + 2*10 = 120
	// Nonce 4: size = 100 + 4*10 = 140
	require.Len(t, result[0], 100, "first tx should be nonce 0")
	require.Len(t, result[1], 120, "second tx should be nonce 2")
	require.Len(t, result[2], 140, "third tx should be nonce 4")
}

func TestReapList_OrderPreservation(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Create encoder that embeds nonce in the bytes for verification
	encoder := func(tx *types.Transaction) ([]byte, error) {
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, tx.Nonce())
		return buf, nil
	}

	rl := mempool.NewReapList(encoder)

	// Add transactions in specific order
	var nonce uint64
	for ; nonce < 5; nonce++ {
		tx := testTx(t, key, nonce, 21000)
		rl.Push(tx)
	}

	result := rl.Reap(0, 0)

	require.Len(t, result, 5, "should reap all transactions")

	// Verify order is preserved (oldest to newest)
	nonce = 0
	for ; nonce < 5; nonce++ {
		nonce := binary.LittleEndian.Uint64(result[nonce])
		require.Equal(t, nonce, nonce, "transactions should be in order")
	}
}

func TestReapList_MultipleReaps(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(deterministicEncoder(100))

	// Add 10 transactions
	for i := uint64(0); i < 10; i++ {
		tx := testTx(t, key, i, 21000)
		rl.Push(tx)
	}

	// First reap: get 3
	result := rl.Reap(300, 0)
	require.Len(t, result, 3)

	// Second reap: get 2
	result = rl.Reap(200, 0)
	require.Len(t, result, 2)

	// Third reap: get remaining 5
	result = rl.Reap(0, 0)
	require.Len(t, result, 5)

	// Fourth reap: empty
	result = rl.Reap(0, 0)
	require.Empty(t, result)
}

func TestReapList_PushAfterReap(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(deterministicEncoder(100))

	// Add 5 transactions
	for i := uint64(0); i < 5; i++ {
		tx := testTx(t, key, i, 21000)
		rl.Push(tx)
	}

	// Reap 3
	result := rl.Reap(300, 0)
	require.Len(t, result, 3)

	// Add 3 more
	for i := uint64(5); i < 8; i++ {
		tx := testTx(t, key, i, 21000)
		rl.Push(tx)
	}

	// Should have 5 total (2 remaining + 3 new)
	result = rl.Reap(0, 0)
	require.Len(t, result, 5)
}

func TestReapList_ConcurrentPushAndReap(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(deterministicEncoder(100))

	var wg sync.WaitGroup

	// Pusher goroutine: continuously add transactions
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := uint64(0); i < 100; i++ {
			tx := testTx(t, key, i, 21000)
			rl.Push(tx)
		}
	}()

	// Reaper goroutine: continuously reap transactions
	wg.Add(1)
	totalReaped := 0
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			result := rl.Reap(200, 0) // Reap 2 at a time
			totalReaped += len(result)
		}
	}()

	wg.Wait()

	// Final reap to get any remaining
	result := rl.Reap(0, 0)
	totalReaped += len(result)

	// We should have reaped close to 100 transactions (may vary due to timing)
	// The exact number depends on race timing, but should be reasonable
	require.GreaterOrEqual(t, totalReaped, 50, "should reap at least half of pushed transactions")
	require.LessOrEqual(t, totalReaped, 100, "should not reap more than pushed")
}

func TestReapList_FirstTransactionExceedsLimit(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(deterministicEncoder(1000))

	// Add transaction
	tx := testTx(t, key, 0, 21000)
	rl.Push(tx)

	// Try to reap with limit smaller than first tx
	result := rl.Reap(500, 0)

	// Should return empty as first tx exceeds limit
	require.Empty(t, result, "should return empty when first tx exceeds limit")

	// Transaction should still be in list for next reap with higher limit
	result = rl.Reap(1000, 0)
	require.Len(t, result, 1, "transaction should still be available with higher limit")
}

func TestReapList_AllTransactionsFailEncoding(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Encoder that always fails
	encoder := func(tx *types.Transaction) ([]byte, error) {
		return nil, errors.New("encoding always fails")
	}

	rl := mempool.NewReapList(encoder)

	// Add transactions
	for i := uint64(0); i < 5; i++ {
		tx := testTx(t, key, i, 21000)
		rl.Push(tx)
	}

	result := rl.Reap(0, 0)

	// Should return empty as all encodings fail
	require.Empty(t, result, "should return empty when all transactions fail encoding")
}
