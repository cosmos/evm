//go:build system_test

package systemtests

import (
	"testing"

	"github.com/cosmos/evm/tests/systemtests/mempool"
)

func TestTxsOrdering(t *testing.T) {
	mempool.TestTxsOrdering(t)
}

func TestTxsReplacement(t *testing.T) {
	mempool.TestTxsReplacement(t)
	mempool.TestMixedTxsReplacementEVMAndCosmos(t)
	mempool.TestMixedTxsReplacementLegacyAndDynamicFee(t)
}
