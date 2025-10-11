//go:build system_test

package systemtests

import (
	"testing"

	"cosmossdk.io/systemtests"
	"github.com/cosmos/evm/tests/systemtests/accountabstraction"
	"github.com/cosmos/evm/tests/systemtests/mempool"
)

func TestMain(m *testing.M) {
	systemtests.RunTests(m)
}

// Mempool Tests
func TestTxsOrdering(t *testing.T) {
	mempool.TestTxsOrdering(t)
}

func TestTxsReplacement(t *testing.T) {
	mempool.TestTxsReplacement(t)
	mempool.TestTxsReplacementWithCosmosTx(t)
	mempool.TestMixedTxsReplacementEVMAndCosmos(t)
	mempool.TestMixedTxsReplacementLegacyAndDynamicFee(t)
}

func TestExceptions(t *testing.T) {
	mempool.TestTxRebroadcasting(t)
	mempool.TestMinimumGasPricesZero(t)
}

// Account Abstraction Tests
func TestEIP7702(t *testing.T) {
	accountabstraction.TestEIP7702(t)
}
