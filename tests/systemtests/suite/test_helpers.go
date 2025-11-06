package suite

import (
	"context"
	"fmt"
	"math/big"
	"slices"
	"sync"
	"time"

	"github.com/cosmos/evm/tests/systemtests/clients"
)

// BaseFee returns the most recently retrieved and stored baseFee.
func (s *BaseTestSuite) BaseFee() *big.Int {
	return s.baseFee
}

// BaseFeeMultiplier returns the cached base fee scaled by the provided multiplier.
func (s *BaseTestSuite) BaseFeeMultiplier(multiplier int64) *big.Int {
	if multiplier <= 0 {
		panic("base fee multiplier must be positive")
	}

	if s.baseFee == nil {
		panic("base fee is not initialized")
	}

	return new(big.Int).Mul(s.baseFee, big.NewInt(multiplier))
}

// SetBaseFee overrides the cached base fee.
func (s *BaseTestSuite) SetBaseFee(fee *big.Int) {
	if fee == nil {
		s.baseFee = nil
		return
	}
	s.baseFee = new(big.Int).Set(fee)
}

const DefaultGasPriceMultiplier int64 = 10

// GasPrice returns the gas price computed from the cached base fee using the default multiplier.
func (s *BaseTestSuite) GasPrice() *big.Int {
	return s.GasPriceMultiplier(DefaultGasPriceMultiplier)
}

// GasPriceMultiplier returns the gas price computed from the cached base fee scaled by the provided multiplier.
func (s *BaseTestSuite) GasPriceMultiplier(multiplier int64) *big.Int {
	if multiplier <= 0 {
		panic("gas price multiplier must be positive")
	}

	if s.baseFee == nil {
		panic("base fee is not initialized")
	}

	return new(big.Int).Mul(s.baseFee, big.NewInt(multiplier))
}

// Account returns the shared test account matching the identifier.
func (s *BaseTestSuite) Account(id string) *TestAccount {
	acc, ok := s.accountsByID[id]
	if !ok {
		panic(fmt.Sprintf("account %s not found", id))
	}
	return acc
}

// EthAccount returns the Ethereum account associated with the given identifier.
func (s *BaseTestSuite) EthAccount(id string) *clients.EthAccount {
	return s.Account(id).Eth
}

// CosmosAccount returns the Cosmos account associated with the given identifier.
func (s *BaseTestSuite) CosmosAccount(id string) *clients.CosmosAccount {
	return s.Account(id).Cosmos
}

// Nodes returns the node IDs in the system under test
func (s *BaseTestSuite) Nodes() []string {
	nodes := make([]string, 4)
	for i := 0; i < 4; i++ {
		nodes[i] = fmt.Sprintf("node%d", i)
	}
	return nodes
}

// Node returns the node ID for the given index
func (s *BaseTestSuite) Node(idx int) string {
	return fmt.Sprintf("node%d", idx)
}

// Acc returns the test account for the given index
func (s *BaseTestSuite) Acc(idx int) *TestAccount {
	if idx < 0 || idx >= len(s.accounts) {
		panic(fmt.Sprintf("account index out of range: %d", idx))
	}
	return s.accounts[idx]
}

// AccID returns the identifier of the test account for the given index.
func (s *BaseTestSuite) AccID(idx int) string {
	return s.Acc(idx).ID
}

// GetOptions returns the current test options
func (s *BaseTestSuite) GetOptions() *TestOptions {
	return s.options
}

// SetOptions sets the current test options
func (s *BaseTestSuite) SetOptions(options *TestOptions) {
	s.options = options
}

// CheckTxsPendingAsync verifies that the expected pending transactions are still pending in the mempool.
// The check runs asynchronously because, if done synchronously, the pending transactions
// might be committed before the verification takes place.
func (s *BaseTestSuite) CheckTxsPendingAsync(expPendingTxs []*TxInfo) error {
	if len(expPendingTxs) == 0 {
		return nil
	}

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		errors []error
	)

	for _, txInfo := range expPendingTxs {
		wg.Add(1)
		go func(tx *TxInfo) { //nolint:gosec // Concurrency is intentional for parallel tx checking
			defer wg.Done()
			err := s.CheckTxPending(tx.DstNodeID, tx.TxHash, tx.TxType, defaultTxPoolContentTimeout)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("tx %s is not pending: %v", tx.TxHash, err))
				mu.Unlock()
			}
		}(txInfo)
	}

	wg.Wait()

	// Return the first error if any occurred
	if len(errors) > 0 {
		return fmt.Errorf("failed to check transactions are pending status: %w", errors[0])
	}

	return nil
}

// CheckTxsQueuedAsync verifies asynchronously that the expected queued transactions are actually queued
// (and not pending) in the mempool. It mirrors CheckTxsPendingAsync in style to better surface API
// failures when querying txpool content.
func (s *BaseTestSuite) CheckTxsQueuedAsync(expQueuedTxs []*TxInfo) error {
	if len(expQueuedTxs) == 0 {
		return nil
	}

	type nodeContent struct {
		nodeID        string
		pendingHashes []string
		queuedHashes  []string
	}

	nodes := s.Nodes()
	contents := make([]nodeContent, len(nodes))

	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		errors []error
	)

	for idx, nodeID := range nodes {
		wg.Add(1)
		go func(i int, nID string) { //nolint:gosec // Concurrency is intentional for parallel tx checking
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), defaultTxPoolContentTimeout)
			defer cancel()

			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			var lastErr error
			for {
				pending, queued, err := s.TxPoolContent(nID, TxTypeEVM, defaultTxPoolContentTimeout)
				if err == nil {
					contents[i] = nodeContent{
						nodeID:        nID,
						pendingHashes: pending,
						queuedHashes:  queued,
					}
					return
				}
				lastErr = err

				select {
				case <-ctx.Done():
					mu.Lock()
					errors = append(errors, fmt.Errorf("failed to call txpool_content api on %s: %w", nID, lastErr))
					mu.Unlock()
					return
				case <-ticker.C:
				}
			}
		}(idx, nodeID)
	}

	wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("failed to check queued transactions: %w", errors[0])
	}

	for _, txInfo := range expQueuedTxs {
		if txInfo.TxType != TxTypeEVM {
			panic("queued txs should be only EVM txs")
		}

		for _, content := range contents {
			pendingTxHashes := content.pendingHashes
			queuedTxHashes := content.queuedHashes

			if content.nodeID == txInfo.DstNodeID {
				if ok := slices.Contains(pendingTxHashes, txInfo.TxHash); ok {
					return fmt.Errorf("tx %s is pending but actually it should be queued.", txInfo.TxHash)
				}
				if ok := slices.Contains(queuedTxHashes, txInfo.TxHash); !ok {
					return fmt.Errorf("tx %s is not contained in queued txs in mempool", txInfo.TxHash)
				}
			} else {
				if ok := slices.Contains(pendingTxHashes, txInfo.TxHash); ok {
					return fmt.Errorf("Locally queued transaction %s is also found in the pending transactions of another node's mempool", txInfo.TxHash)
				}
				if ok := slices.Contains(queuedTxHashes, txInfo.TxHash); ok {
					return fmt.Errorf("Locally queued transaction %s is also found in the queued transactions of another node's mempool", txInfo.TxHash)
				}
			}
		}
	}

	return nil
}
