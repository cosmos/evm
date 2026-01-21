package legacypool

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/cosmos/evm/mempool/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
)

// Helper to create a test transaction
func createTestTx(nonce uint64, gasTipCap *big.Int, gasFeeCap *big.Int) txpool.TxWithFees {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)

	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       21000,
		To:        &addr,
		Value:     big.NewInt(100),
		Data:      nil,
	})

	return txpool.TxWithFees{Tx: tx, Fees: uint256.NewInt(1)}
}

// TestBasicCollect tests basic functionality of adding and collecting txs
func TestBasicCollect(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	// Add transactions
	tx1 := createTestTx(0, big.NewInt(1e9), big.NewInt(2e9))
	tx2 := createTestTx(1, big.NewInt(1e9), big.NewInt(2e9))
	tx3 := createTestTx(0, big.NewInt(1e9), big.NewInt(2e9))

	collector.AppendTx(tx1)
	collector.AppendTx(tx2)
	collector.AppendTx(tx3)

	// Mark as complete
	complete := collector.StartNewHeight(big.NewInt(1))
	collector.AppendTx(tx1)
	collector.AppendTx(tx2)
	collector.AppendTx(tx3)
	complete()

	// Collect transactions
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result := collector.Collect(ctx, big.NewInt(1))
	assert.NotNil(t, result)

	assert.Equal(t, tx1, result[0])
	assert.Equal(t, tx2, result[1])
	assert.Equal(t, tx3, result[2])
}

// TestCollectTimeout tests that Collect returns nil when timing out before reaching target height
func TestCollectTimeoutBeforeHeight(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	// Request height 3 but don't advance to it
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := collector.Collect(ctx, big.NewInt(3))
	assert.Nil(t, result)
}

// TestCollectPartialResults tests that Collect returns partial results when timing out during processing
func TestCollectPartialResults(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	tx1 := createTestTx(0, big.NewInt(1e9), big.NewInt(2e9))

	// Start new height but don't mark as complete
	collector.StartNewHeight(big.NewInt(1))
	collector.AppendTx(tx1)

	// Collect with timeout - should get partial results
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := collector.Collect(ctx, big.NewInt(1))
	assert.NotNil(t, result)
	assert.Equal(t, tx1, result[0])
}

// TestCollectorBehindByOneHeight tests collecting when collector is one height behind
func TestCollectorBehindByOneHeight(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	tx1 := createTestTx(0, big.NewInt(1e9), big.NewInt(2e9))

	// Start at height 1
	complete1 := collector.StartNewHeight(big.NewInt(1))
	collector.AppendTx(tx1)

	// Start collecting height 2 in background
	resultChan := make(chan []txpool.TxWithFees)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		result := collector.Collect(ctx, big.NewInt(2))
		resultChan <- result
	}()

	// Give collector goroutine time to start waiting
	time.Sleep(100 * time.Millisecond)

	// Complete height 1
	complete1()

	// Advance to height 2
	tx2 := createTestTx(1, big.NewInt(1e9), big.NewInt(2e9))
	complete2 := collector.StartNewHeight(big.NewInt(2))
	collector.AppendTx(tx2)
	complete2()

	// Wait for result
	result := <-resultChan
	assert.NotNil(t, result)
	assert.Len(t, result, 1)
	assert.Equal(t, tx2, result[0])
}

// TestCollectorBehindByTwoHeights tests collecting when collector is two heights behind
func TestCollectorBehindByTwoHeights(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	// Start at height 1
	complete1 := collector.StartNewHeight(big.NewInt(1))
	collector.AppendTx(createTestTx(0, big.NewInt(1e9), big.NewInt(2e9)))

	// Start collecting height 3 in background
	resultChan := make(chan []txpool.TxWithFees)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		result := collector.Collect(ctx, big.NewInt(3))
		resultChan <- result
	}()

	// Give collector goroutine time to start waiting
	time.Sleep(100 * time.Millisecond)

	// Complete height 1 and advance to height 2
	complete1()
	complete2 := collector.StartNewHeight(big.NewInt(2))
	collector.AppendTx(createTestTx(1, big.NewInt(1e9), big.NewInt(2e9)))

	// Give some time for height 2 processing
	time.Sleep(100 * time.Millisecond)

	// Complete height 2 and advance to height 3
	complete2()
	tx3 := createTestTx(2, big.NewInt(1e9), big.NewInt(2e9))
	complete3 := collector.StartNewHeight(big.NewInt(3))
	collector.AppendTx(tx3)
	complete3()

	// Wait for result
	result := <-resultChan
	assert.NotNil(t, result)
	assert.Len(t, result, 1)
	assert.Equal(t, tx3, result[0])
}

// TestRemoveTx tests that transactions can be removed
func TestRemoveTx(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	tx1 := createTestTx(0, big.NewInt(1e9), big.NewInt(2e9))
	tx2 := createTestTx(1, big.NewInt(1e9), big.NewInt(2e9))

	complete := collector.StartNewHeight(big.NewInt(1))
	collector.AppendTx(tx1)
	collector.AppendTx(tx2)

	// Remove one tx
	collector.RemoveTx(tx1.Tx)
	complete()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result := collector.Collect(ctx, big.NewInt(1))
	assert.NotNil(t, result)
	assert.Len(t, result, 1)
	assert.Equal(t, tx2, result[0])
}

// TestPanicOnOldHeight tests that requesting an old height panics
func TestPanicOnOldHeight(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	// Advance to height 2
	complete1 := collector.StartNewHeight(big.NewInt(1))
	complete1()

	collector.StartNewHeight(big.NewInt(2))

	// Try to collect height 1 (old height) - should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when requesting old height")
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	collector.Collect(ctx, big.NewInt(1))
}

// TestConcurrentRemove tests concurrent Remove operations
func TestConcurrentRemove(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	complete := collector.StartNewHeight(big.NewInt(1))

	numOperations := 1000

	// First, add all transactions
	for i := 0; i < numOperations; i++ {
		tx := createTestTx(uint64(i), big.NewInt(1e9), big.NewInt(2e9))
		collector.AppendTx(tx)
	}

	// Then concurrently remove some transactions
	var wg sync.WaitGroup
	for i := 0; i < numOperations; i += 2 {
		wg.Add(1)
		go func(nonce int) {
			defer wg.Done()
			tx := createTestTx(uint64(nonce), big.NewInt(1e9), big.NewInt(2e9))
			collector.RemoveTx(tx.Tx)
		}(i)
	}

	wg.Wait()
	complete()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result := collector.Collect(ctx, big.NewInt(1))
	assert.NotNil(t, result)

	// Should have roughly half the transactions (the odd-nonce ones)
	count := len(result)
	if count != numOperations/2 {
		t.Errorf("Expected %d txs after concurrent removals, got %d", numOperations/2, count)
	}
}
