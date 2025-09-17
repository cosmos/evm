package suite

import (
	"fmt"
	"math/big"
	"slices"
	"sync"
	"time"
)

// BaseFee returns the most recently retrieved and stored baseFee.
func (s *SystemTestSuite) BaseFee() *big.Int {
	return s.baseFee
}

// BaseFeeX2 returns the double of the most recently retrieved and stored baseFee.
func (s *SystemTestSuite) BaseFeeX2() *big.Int {
	return new(big.Int).Mul(s.baseFee, big.NewInt(2))
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

// Acc returns the account ID for the given index
func (s *SystemTestSuite) Acc(idx int) string {
	return fmt.Sprintf("acc%d", idx)
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
	if len(expPendingTxs) > 0 {
		var wg sync.WaitGroup
		errCh := make(chan error, len(expPendingTxs))

		for _, txInfo := range expPendingTxs {
			wg.Add(1)
			go func(tx *TxInfo) {
				defer wg.Done()
				err := s.CheckTxPending(tx.DstNodeID, tx.TxHash, tx.TxType, time.Second*30)
				if err != nil {
					errCh <- fmt.Errorf("tx %s is not pending or committed: %v", tx.TxHash, err)
				}
			}(txInfo)
		}

		wg.Wait()
		close(errCh)

		// Check for any errors
		for err := range errCh {
			return fmt.Errorf("failed to check transactions are pending status: %w", err)
		}
	}

	return nil
}

// CheckTxsQueuedSync verifies that the expected queued transactions are actually queued and not pending in the mempool.
func (s *SystemTestSuite) CheckTxsQueuedSync(expQueuedTxs []*TxInfo) error {
	pendingHashes := make([][]string, len(s.Nodes()))
	queuedHashes := make([][]string, len(s.Nodes()))
	for i := range s.Nodes() {
		pending, queued, err := s.TxPoolContent(s.Node(i), TxTypeEVM)
		if err != nil {
			return fmt.Errorf("failed to call txpool_content api")
		}
		queuedHashes[i] = queued
		pendingHashes[i] = pending
	}

	for _, txInfo := range s.GetExpQueuedTxs() {
		if txInfo.TxType != TxTypeEVM {
			panic("queued txs should be only EVM txs")
		}

		for i := range s.Nodes() {
			pendingTxHashes := pendingHashes[i]
			queuedTxHashes := queuedHashes[i]

			if s.Node(i) == txInfo.DstNodeID {
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
