//go:build system_test

package systemtests

import (
	"sync"
	"testing"

	"cosmossdk.io/systemtests"
	"github.com/cosmos/evm/tests/systemtests/accountabstraction"
	"github.com/cosmos/evm/tests/systemtests/chainupgrade"
	"github.com/cosmos/evm/tests/systemtests/eip712"
	"github.com/cosmos/evm/tests/systemtests/mempool"
	suites "github.com/cosmos/evm/tests/systemtests/suite"
)

func TestMain(m *testing.M) {
	systemtests.RunTests(m)
}

var (
	sharedSuiteOnce sync.Once
	sharedSuite     *suites.SystemTestSuite
)

func getSharedSuite(t *testing.T) *suites.SystemTestSuite {
	t.Helper()

	sharedSuiteOnce.Do(func() {
		sharedSuite = suites.NewSystemTestSuite(t)
	})

	return sharedSuite
}

// Mempool Tests
func TestTxsOrdering(t *testing.T) {
	ms := mempool.NewSuite(getSharedSuite(t))
	mempool.RunTxsOrdering(t, ms)
}

func TestTxsReplacement(t *testing.T) {
	ms := mempool.NewSuite(getSharedSuite(t))
	mempool.RunTxsReplacement(t, ms)
	mempool.RunTxsReplacementWithCosmosTx(t, ms)
	mempool.RunMixedTxsReplacementLegacyAndDynamicFee(t, ms)
}

func TestExceptions(t *testing.T) {
	ms := mempool.NewSuite(getSharedSuite(t))
	mempool.RunTxRebroadcasting(t, ms)
	mempool.RunMinimumGasPricesZero(t, ms)
}

// Account Abstraction Tests
func TestEIP7702(t *testing.T) {
	accountabstraction.RunEIP7702(t, getSharedSuite(t))
}

// EIP-712 Tests
func TestEIP712BankSend(t *testing.T) {
	sut := eip712.NewSystemTestSuite(getSharedSuite(t))
	eip712.RunEIP712BankSend(t, sut)
}

func TestEIP712BankSendWithBalanceCheck(t *testing.T) {
	sut := eip712.NewSystemTestSuite(getSharedSuite(t))
	eip712.RunEIP712BankSendWithBalanceCheck(t, sut)
}

func TestEIP712MultipleBankSends(t *testing.T) {
	sut := eip712.NewSystemTestSuite(getSharedSuite(t))
	eip712.RunEIP712MultipleBankSends(t, sut)
}

func TestUpgrade(t *testing.T) {
	s := getSharedSuite(t)
	chainupgrade.RunChainUpgrade(t, s)
}
