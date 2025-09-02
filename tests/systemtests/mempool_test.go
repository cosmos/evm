package systemtests

import (
	"testing"

	"github.com/evmos/tests/systemtests/mempool"
)

func TestNonceGappedTransaction(t *testing.T) {
	mempool.TestNonceGappedTransaction(t)
}

func TestTransactionOrdering(t *testing.T) {
	mempool.TestTransactionOrdering(t)
}

func TestTransactionReplacement(t *testing.T) {
	mempool.TestTransactionReplacement(t)
}
