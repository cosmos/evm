//go:build system_test

package systemtests

import (
	"testing"

	"github.com/cosmos/evm/tests/systemtests/accountabstraction"
	"github.com/cosmos/evm/tests/systemtests/mempool"

	"github.com/cosmos/evm/tests/systemtests/eip712"

	"cosmossdk.io/systemtests"
)

func TestMain(m *testing.M) {
	systemtests.RunTests(m)
}

func TestCosmosTxCompat(t *testing.T) {
	mempool.TestCosmosTxsCompatibility(t)
}

// Mempool Tests
func TestTxsOrdering(t *testing.T) {
	mempool.TestTxsOrdering(t)
}

func TestTxsReplacement(t *testing.T) {
	mempool.TestTxsReplacement(t)
	mempool.TestTxsReplacementWithCosmosTx(t)
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

// EIP-712 Tests
func TestEIP712BankSend(t *testing.T) {
	eip712.TestEIP712BankSend(t)
}

func TestEIP712BankSendWithBalanceCheck(t *testing.T) {
	eip712.TestEIP712BankSendWithBalanceCheck(t)
}

func TestEIP712MultipleBankSends(t *testing.T) {
	eip712.TestEIP712MultipleBankSends(t)
}
