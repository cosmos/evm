package suite

import (
	"fmt"
	"math/big"
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
