package suite

import (
	"fmt"
	"math/big"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/cosmos/evm/tests/systemtests/clients"
)

// BaseFee returns the most recently retrieved and stored baseFee.
func (s *SystemTestSuite) BaseFee() *big.Int {
	return s.baseFee
}

// BaseFeeX2 returns the double of the most recently retrieved and stored baseFee.
func (s *SystemTestSuite) BaseFeeX2() *big.Int {
	return new(big.Int).Mul(s.baseFee, big.NewInt(2))
}

// SetBaseFee overrides the cached base fee.
func (s *SystemTestSuite) SetBaseFee(fee *big.Int) {
	if fee == nil {
		s.baseFee = nil
		return
	}
	s.baseFee = new(big.Int).Set(fee)
}

const defaultGasPriceMultiplier = 10

func (s *SystemTestSuite) GetTxGasPrice(baseFee *big.Int) *big.Int {
	return new(big.Int).Mul(baseFee, big.NewInt(defaultGasPriceMultiplier))
}

// Account returns the shared test account matching the identifier.
func (s *SystemTestSuite) Account(id string) *TestAccount {
	acc, ok := s.accountsByID[id]
	if !ok {
		panic(fmt.Sprintf("account %s not found", id))
	}
	return acc
}

// EthAccount returns the Ethereum account associated with the given identifier.
func (s *SystemTestSuite) EthAccount(id string) *clients.EthAccount {
	return s.Account(id).Eth
}

// CosmosAccount returns the Cosmos account associated with the given identifier.
func (s *SystemTestSuite) CosmosAccount(id string) *clients.CosmosAccount {
	return s.Account(id).Cosmos
}

// AcquireAcc blocks until an idle account is available and returns it.
func (s *SystemTestSuite) AcquireAcc() *TestAccount {
	s.accountsMu.Lock()
	defer s.accountsMu.Unlock()

	for {
		for _, acc := range s.accounts {
			if !acc.inUse {
				acc.inUse = true
				return acc
			}
		}
		s.accountCond.Wait()
	}
}

// ReleaseAcc releases a previously acquired account back into the idle pool.
func (s *SystemTestSuite) ReleaseAcc(acc *TestAccount) {
	if acc == nil {
		return
	}

	s.accountsMu.Lock()
	defer s.accountsMu.Unlock()

	if !acc.inUse {
		panic(fmt.Sprintf("account %s released without acquisition", acc.ID))
	}

	acc.inUse = false
	s.accountCond.Signal()
}

// AcquireAccForTest acquires an idle account and registers automatic release via t.Cleanup.
func (s *SystemTestSuite) AcquireAccForTest(t *testing.T) *TestAccount {
	acc := s.AcquireAcc()
	t.Cleanup(func() {
		s.ReleaseAcc(acc)
	})
	return acc
}

// GetExpPendingTxs returns the expected pending transactions
func (s *SystemTestSuite) GetExpPendingTxs() []*TxInfo {
	return s.expPendingTxs
}

// SetExpPendingTxs sets the expected pending transactions
func (s *SystemTestSuite) SetExpPendingTxs(txs ...*TxInfo) {
	s.expPendingTxs = txs
}

// GetExpQueuedTxs returns the expected queued transactions
func (s *SystemTestSuite) GetExpQueuedTxs() []*TxInfo {
	return s.expQueuedTxs
}

// SetExpQueuedTxs sets the expected queued transactions, filtering out any Cosmos transactions
func (s *SystemTestSuite) SetExpQueuedTxs(txs ...*TxInfo) {
	queuedTxs := make([]*TxInfo, 0)
	for _, txInfo := range txs {
		if txInfo.TxType == TxTypeCosmos {
			continue
		}
		queuedTxs = append(queuedTxs, txInfo)
	}
	s.expQueuedTxs = queuedTxs
}

// PromoteExpTxs promotes the given number of expected queued transactions to expected pending transactions
func (s *SystemTestSuite) PromoteExpTxs(count int) {
	if count <= 0 || len(s.expQueuedTxs) == 0 {
		return
	}

	// Ensure we don't try to promote more than available
	actualCount := count
	if actualCount > len(s.expQueuedTxs) {
		actualCount = len(s.expQueuedTxs)
	}

	// Pop from expQueuedTxs and push to expPendingTxs
	txs := s.expQueuedTxs[:actualCount]
	s.expPendingTxs = append(s.expPendingTxs, txs...)
	s.expQueuedTxs = s.expQueuedTxs[actualCount:]
}

// Nodes returns the node IDs in the system under test
func (s *SystemTestSuite) Nodes() []string {
	nodes := make([]string, 4)
	for i := 0; i < 4; i++ {
		nodes[i] = fmt.Sprintf("node%d", i)
	}
	return nodes
}

// Node returns the node ID for the given index
func (s *SystemTestSuite) Node(idx int) string {
	return fmt.Sprintf("node%d", idx)
}

// Acc returns the test account for the given index
func (s *SystemTestSuite) Acc(idx int) *TestAccount {
	if idx < 0 || idx >= len(s.accounts) {
		panic(fmt.Sprintf("account index out of range: %d", idx))
	}
	return s.accounts[idx]
}

// AccID returns the identifier of the test account for the given index.
func (s *SystemTestSuite) AccID(idx int) string {
	return s.Acc(idx).ID
}

// GetOptions returns the current test options
func (s *SystemTestSuite) GetOptions() *TestOptions {
	return s.options
}

// SetOptions sets the current test options
func (s *SystemTestSuite) SetOptions(options *TestOptions) {
	s.options = options
}

// CheckTxsPendingAsync verifies that the expected pending transactions are still pending in the mempool.
// The check runs asynchronously because, if done synchronously, the pending transactions
// might be committed before the verification takes place.
func (s *SystemTestSuite) CheckTxsPendingAsync(expPendingTxs []*TxInfo) error {
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
			err := s.CheckTxPending(tx.DstNodeID, tx.TxHash, tx.TxType, time.Second*120)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("tx %s is not pending or committed: %v", tx.TxHash, err))
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
func (s *SystemTestSuite) CheckTxsQueuedAsync(expQueuedTxs []*TxInfo) error {
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
		go func(i int, nID string) { //nolint:gosec // intentional concurrency for parallel checks
			defer wg.Done()

			pending, queued, err := s.TxPoolContent(nID, TxTypeEVM, defaultTxPoolContentTimeout)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to call txpool_content api on %s: %w", nID, err))
				mu.Unlock()
				return
			}

			contents[i] = nodeContent{
				nodeID:        nID,
				pendingHashes: pending,
				queuedHashes:  queued,
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
