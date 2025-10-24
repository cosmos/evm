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

/*
 * Mempool Tests
 */
func TestMempoolTxsOrdering(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunTxsOrdering)
}

func TestMempoolTxsReplacement(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunTxsReplacement)
}

func TestMempoolTxsReplacementWithCosmosTx(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunTxsReplacementWithCosmosTx)
}

func TestMempoolMixedTxsReplacementEVMAndCosmos(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunMixedTxsReplacementEVMAndCosmos)
}

func TestMempoolMixedTxsReplacementLegacyAndDynamicFee(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunMixedTxsReplacementLegacyAndDynamicFee)
}

func TestMempoolTxRebroadcasting(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunTxRebroadcasting)
}

func TestMinimumGasPricesZero(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunMinimumGasPricesZero)
}

func TestMempoolCosmosTxsCompatibility(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunCosmosTxsCompatibility)
}

/*
 * EIP-712 Tests
 */
func TestEIP712BankSend(t *testing.T) {
	suite.RunWithSharedSuite(t, eip712.RunEIP712BankSend)
}

func TestEIP712BankSendWithBalanceCheck(t *testing.T) {
	suite.RunWithSharedSuite(t, eip712.RunEIP712BankSendWithBalanceCheck)
}

func TestEIP712MultipleBankSends(t *testing.T) {
	suite.RunWithSharedSuite(t, eip712.RunEIP712MultipleBankSends)
}

/*
 * Account Abstraction Tests
 */
func TestAccountAbstractionEIP7702(t *testing.T) {
	suite.RunWithSharedSuite(t, accountabstraction.RunEIP7702)
}

/*
 * Chain Upgrade Tests
 */
func TestChainUpgrade(t *testing.T) {
	suite.RunWithSharedSuite(t, chainupgrade.RunChainUpgrade)
}
