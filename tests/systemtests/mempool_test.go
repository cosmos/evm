package systemtests

import (
	"testing"

	"github.com/cosmos/evm/tests/systemtests/mempool"
)

func TestNonceGappedTxs(t *testing.T) {
	mempool.TestNonceGappedTxs(t)
}

func TestTxsOrdering(t *testing.T) {
	mempool.TestTxsOrdering(t)
}

func TestTxsReplacement(t *testing.T) {
	mempool.TestTxsReplacement(t)
	mempool.TestMixedTxsReplacement(t)
}
