//go:build system_test

package systemtests

import (
	"testing"

	"github.com/cosmos/evm/tests/systemtests/accountabstraction"
	"github.com/cosmos/evm/tests/systemtests/chainupgrade"
	"github.com/cosmos/evm/tests/systemtests/eip712"
	"github.com/cosmos/evm/tests/systemtests/mempool"
	"github.com/cosmos/evm/tests/systemtests/suite"

	"cosmossdk.io/systemtests"
)

func TestMain(m *testing.M) {
	systemtests.RunTests(m)
}

func TestDefaultNodeArgs(t *testing.T) {
	s := suite.GetSharedSuite(t)

	/**
	 * Mempool tests
	 */
	t.Run("Mempool/TxsOrdering", func(t *testing.T) {
		mempool.RunTxsOrdering(t, s)
	})

	t.Run("Mempool/TxsReplacement", func(t *testing.T) {
		mempool.RunTxsReplacement(t, s)
	})

	t.Run("Mempool/TxsReplacementWithCosmosTx", func(t *testing.T) {
		mempool.RunTxsReplacementWithCosmosTx(t, s)
	})

	t.Run("Mempool/MixedTxsReplacementEVMAndCosmos", func(t *testing.T) {
		mempool.RunMixedTxsReplacementEVMAndCosmos(t, s)
	})

	t.Run("Mempool/MixedTxsReplacementLegacyAndDynamicFee", func(t *testing.T) {
		mempool.RunMixedTxsReplacementLegacyAndDynamicFee(t, s)
	})

	t.Run("Mempool/TxRebroadcasting", func(t *testing.T) {
		mempool.RunTxRebroadcasting(t, s)
	})

	t.Run("Mempool/TxRebroadcasting", func(t *testing.T) {
		mempool.RunCosmosTxsCompatibility(t, s)
	})

	/**
	 * EIP-712 tests
	 */
	t.Run("EIP712/BankSend", func(t *testing.T) {
		eip712.RunEIP712BankSend(t, s)
	})

	t.Run("EIP712/BankSendWithBalanceCheck", func(t *testing.T) {
		eip712.RunEIP712BankSendWithBalanceCheck(t, s)
	})

	t.Run("EIP712/MultipleBankSends", func(t *testing.T) {
		eip712.RunEIP712MultipleBankSends(t, s)
	})

	/**
	 * Account Abstraction tests
	 */
	t.Run("AccountAbstraction/EIP7702", func(t *testing.T) {
		accountabstraction.RunEIP7702(t, s)
	})

}

func TestChainUpgrade(t *testing.T) {
	s := suite.GetSharedSuite(t)
	chainupgrade.RunChainUpgrade(t, s)
}
