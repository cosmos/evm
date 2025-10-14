//go:build system_test

package systemtests

import (
	"sync"
	"testing"

	"cosmossdk.io/systemtests"
	"github.com/cosmos/evm/tests/systemtests/accountabstraction"
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

func TestDefaultNodeArgs(t *testing.T) {
	s := getSharedSuite(t)

	t.Run("Mempool/TxsOrdering", func(t *testing.T) {
		mempool.RunTxsOrdering(t, mempool.NewSuite(s))
	})

	t.Run("Mempool/TxsReplacement", func(t *testing.T) {
		mempool.RunTxsReplacement(t, mempool.NewSuite(s))
	})

	t.Run("Mempool/TxsReplacementWithCosmosTx", func(t *testing.T) {
		mempool.RunTxsReplacementWithCosmosTx(t, mempool.NewSuite(s))
	})

	t.Run("Mempool/MixedTxsReplacement", func(t *testing.T) {
		mempool.RunMixedTxsReplacementLegacyAndDynamicFee(t, mempool.NewSuite(s))
	})

	t.Run("Mempool/TxRebroadcasting", func(t *testing.T) {
		mempool.RunTxRebroadcasting(t, mempool.NewSuite(s))
	})

	t.Run("EIP712/BankSend", func(t *testing.T) {
		eip712.RunEIP712BankSend(t, eip712.NewSystemTestSuite(s))
	})

	t.Run("EIP712/BankSendWithBalanceCheck", func(t *testing.T) {
		eip712.RunEIP712BankSendWithBalanceCheck(t, eip712.NewSystemTestSuite(s))
	})

	t.Run("EIP712/MultipleBankSends", func(t *testing.T) {
		eip712.RunEIP712MultipleBankSends(t, eip712.NewSystemTestSuite(s))
	})

	t.Run("AccountAbstraction/EIP7702", func(t *testing.T) {
		accountabstraction.RunEIP7702(t, s)
	})
}

func TestMinimumGasPricesZero(t *testing.T) {
	s := getSharedSuite(t)
	mempool.RunMinimumGasPricesZero(t, mempool.NewSuite(s))
}

// func TestUpgrade(t *testing.T) {
// 	s := getSharedSuite(t)
// 	s.LockChain()
// 	defer s.UnlockChain()

// 	chainupgrade.RunChainUpgrade(t, s)
// }
