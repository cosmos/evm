package legacypool

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/cosmos/evm/mempool/txpool"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
)

// Helper to create a test transaction
func createTestTx(nonce uint64, gasTipCap *big.Int, gasFeeCap *big.Int) *types.Transaction {
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

	return tx
}

// TestBasicCollect tests basic functionality of adding and collecting txs
func TestBasicCollect(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	addr1 := common.HexToAddress("0x1")
	addr2 := common.HexToAddress("0x2")

	// Add transactions
	tx1 := createTestTx(0, big.NewInt(1e9), big.NewInt(2e9))
	tx2 := createTestTx(1, big.NewInt(1e9), big.NewInt(2e9))
	tx3 := createTestTx(0, big.NewInt(1e9), big.NewInt(2e9))

	collector.AddTx(addr1, tx1)
	collector.AddTx(addr1, tx2)
	collector.AddTx(addr2, tx3)

	// Mark as complete
	complete := collector.StartNewHeight(big.NewInt(1))
	collector.AddTx(addr1, tx1)
	collector.AddTx(addr1, tx2)
	collector.AddTx(addr2, tx3)
	complete()

	// Collect transactions
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result := collector.Collect(ctx, big.NewInt(1), txpool.PendingFilter{})

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result[addr1]) != 2 {
		t.Errorf("Expected 2 txs for addr1, got %d", len(result[addr1]))
	}

	if len(result[addr2]) != 1 {
		t.Errorf("Expected 1 tx for addr2, got %d", len(result[addr2]))
	}
}

// TestCollectTimeout tests that Collect returns nil when timing out before reaching target height
func TestCollectTimeoutBeforeHeight(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	// Request height 3 but don't advance to it
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := collector.Collect(ctx, big.NewInt(3), txpool.PendingFilter{})

	if result != nil {
		t.Error("Expected nil result when timing out before reaching target height")
	}
}

// TestCollectPartialResults tests that Collect returns partial results when timing out during processing
func TestCollectPartialResults(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	addr1 := common.HexToAddress("0x1")
	tx1 := createTestTx(0, big.NewInt(1e9), big.NewInt(2e9))

	// Start new height but don't mark as complete
	collector.StartNewHeight(big.NewInt(1))
	collector.AddTx(addr1, tx1)

	// Collect with timeout - should get partial results
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := collector.Collect(ctx, big.NewInt(1), txpool.PendingFilter{})

	if result == nil {
		t.Fatal("Expected partial results, got nil")
	}

	if len(result[addr1]) != 1 {
		t.Errorf("Expected 1 tx in partial results, got %d", len(result[addr1]))
	}
}

// TestCollectWithMinTip tests that transactions are filtered by minimum tip
func TestCollectWithMinTip(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	addr1 := common.HexToAddress("0x1")

	// Create txs with tips that meet minimum, then fall below
	// (sorted by nonce: high tip first, then low tip)
	txHighTip := createTestTx(0, big.NewInt(2e9), big.NewInt(3e9)) // 2 gwei tip (nonce 0)
	txLowTip := createTestTx(1, big.NewInt(1e8), big.NewInt(2e9))  // 0.1 gwei tip (nonce 1)

	complete := collector.StartNewHeight(big.NewInt(1))
	collector.AddTx(addr1, txHighTip)
	collector.AddTx(addr1, txLowTip)
	complete()

	// Collect with minTip of 1 gwei
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	filter := txpool.PendingFilter{
		MinTip:  uint256.MustFromBig(big.NewInt(1e9)),
		BaseFee: uint256.MustFromBig(big.NewInt(1e9)),
	}
	result := collector.Collect(ctx, big.NewInt(1), filter)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Should only get the high tip transaction (nonce 0)
	// The low tip tx at nonce 1 and any subsequent txs are filtered out
	if len(result[addr1]) != 1 {
		t.Fatalf("Expected 1 tx after filtering, got %d", len(result[addr1]))
	}

	if result[addr1][0].Nonce() != 0 {
		t.Errorf("Expected high tip tx (nonce 0), got nonce %d", result[addr1][0].Nonce())
	}
}

// TestCollectorBehindByOneHeight tests collecting when collector is one height behind
func TestCollectorBehindByOneHeight(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	addr1 := common.HexToAddress("0x1")
	tx1 := createTestTx(0, big.NewInt(1e9), big.NewInt(2e9))

	// Start at height 1
	complete1 := collector.StartNewHeight(big.NewInt(1))
	collector.AddTx(addr1, tx1)

	// Start collecting height 2 in background
	resultChan := make(chan map[common.Address]types.Transactions)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		result := collector.Collect(ctx, big.NewInt(2), txpool.PendingFilter{})
		resultChan <- result
	}()

	// Give collector goroutine time to start waiting
	time.Sleep(100 * time.Millisecond)

	// Complete height 1
	complete1()

	// Advance to height 2
	tx2 := createTestTx(1, big.NewInt(1e9), big.NewInt(2e9))
	complete2 := collector.StartNewHeight(big.NewInt(2))
	collector.AddTx(addr1, tx2)
	complete2()

	// Wait for result
	result := <-resultChan

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result[addr1]) != 1 {
		t.Errorf("Expected 1 tx, got %d", len(result[addr1]))
	}

	if result[addr1][0].Nonce() != 1 {
		t.Errorf("Expected tx with nonce 1, got %d", result[addr1][0].Nonce())
	}
}

// TestCollectorBehindByTwoHeights tests collecting when collector is two heights behind
func TestCollectorBehindByTwoHeights(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	addr1 := common.HexToAddress("0x1")

	// Start at height 1
	complete1 := collector.StartNewHeight(big.NewInt(1))
	collector.AddTx(addr1, createTestTx(0, big.NewInt(1e9), big.NewInt(2e9)))

	// Start collecting height 3 in background
	resultChan := make(chan map[common.Address]types.Transactions)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		result := collector.Collect(ctx, big.NewInt(3), txpool.PendingFilter{})
		resultChan <- result
	}()

	// Give collector goroutine time to start waiting
	time.Sleep(100 * time.Millisecond)

	// Complete height 1 and advance to height 2
	complete1()
	complete2 := collector.StartNewHeight(big.NewInt(2))
	collector.AddTx(addr1, createTestTx(1, big.NewInt(1e9), big.NewInt(2e9)))

	// Give some time for height 2 processing
	time.Sleep(100 * time.Millisecond)

	// Complete height 2 and advance to height 3
	complete2()
	tx3 := createTestTx(2, big.NewInt(1e9), big.NewInt(2e9))
	complete3 := collector.StartNewHeight(big.NewInt(3))
	collector.AddTx(addr1, tx3)
	complete3()

	// Wait for result
	result := <-resultChan

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result[addr1]) != 1 {
		t.Errorf("Expected 1 tx, got %d", len(result[addr1]))
	}

	if result[addr1][0].Nonce() != 2 {
		t.Errorf("Expected tx with nonce 2, got %d", result[addr1][0].Nonce())
	}
}

// TestRemoveTx tests that transactions can be removed
func TestRemoveTx(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	addr1 := common.HexToAddress("0x1")
	tx1 := createTestTx(0, big.NewInt(1e9), big.NewInt(2e9))
	tx2 := createTestTx(1, big.NewInt(1e9), big.NewInt(2e9))

	complete := collector.StartNewHeight(big.NewInt(1))
	collector.AddTx(addr1, tx1)
	collector.AddTx(addr1, tx2)

	// Remove one tx
	collector.RemoveTx(addr1, tx1)
	complete()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result := collector.Collect(ctx, big.NewInt(1), txpool.PendingFilter{})

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result[addr1]) != 1 {
		t.Fatalf("Expected 1 tx after removal, got %d", len(result[addr1]))
	}

	if result[addr1][0].Nonce() != 1 {
		t.Errorf("Expected tx with nonce 1, got %d", result[addr1][0].Nonce())
	}
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

	collector.Collect(ctx, big.NewInt(1), txpool.PendingFilter{})
}

// TestTxsAreSortedByNonce tests that collected transactions are sorted by nonce
func TestTxsAreSortedByNonce(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	addr1 := common.HexToAddress("0x1")

	// Add transactions in reverse nonce order
	tx2 := createTestTx(2, big.NewInt(1e9), big.NewInt(2e9))
	tx0 := createTestTx(0, big.NewInt(1e9), big.NewInt(2e9))
	tx1 := createTestTx(1, big.NewInt(1e9), big.NewInt(2e9))

	complete := collector.StartNewHeight(big.NewInt(1))
	collector.AddTx(addr1, tx2)
	collector.AddTx(addr1, tx0)
	collector.AddTx(addr1, tx1)
	complete()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result := collector.Collect(ctx, big.NewInt(1), txpool.PendingFilter{})

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result[addr1]) != 3 {
		t.Fatalf("Expected 3 txs, got %d", len(result[addr1]))
	}

	// Check that txs are sorted by nonce
	for i := 0; i < len(result[addr1]); i++ {
		if result[addr1][i].Nonce() != uint64(i) {
			t.Errorf("Expected nonce %d at position %d, got %d", i, i, result[addr1][i].Nonce())
		}
	}
}

// TestConcurrentRemove tests concurrent Remove operations
func TestConcurrentRemove(t *testing.T) {
	collector := newTxCollector(big.NewInt(1))

	addr1 := common.HexToAddress("0x1")
	complete := collector.StartNewHeight(big.NewInt(1))

	numOperations := 1000

	// First, add all transactions
	for i := 0; i < numOperations; i++ {
		tx := createTestTx(uint64(i), big.NewInt(1e9), big.NewInt(2e9))
		collector.AddTx(addr1, tx)
	}

	// Then concurrently remove some transactions
	var wg sync.WaitGroup
	for i := 0; i < numOperations; i += 2 {
		wg.Add(1)
		go func(nonce int) {
			defer wg.Done()
			tx := createTestTx(uint64(nonce), big.NewInt(1e9), big.NewInt(2e9))
			collector.RemoveTx(addr1, tx)
		}(i)
	}

	wg.Wait()
	complete()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result := collector.Collect(ctx, big.NewInt(1), txpool.PendingFilter{})

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Should have roughly half the transactions (the odd-nonce ones)
	count := len(result[addr1])
	if count != numOperations/2 {
		t.Errorf("Expected %d txs after concurrent removals, got %d", numOperations/2, count)
	}
}
