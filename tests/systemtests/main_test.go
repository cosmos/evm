//go:build system_test

package systemtests

import (
	"testing"

	"github.com/cosmos/evm/tests/systemtests/accountabstraction"
	"github.com/cosmos/evm/tests/systemtests/chainupgrade"
	"github.com/cosmos/evm/tests/systemtests/eip712"

	"github.com/cosmos/evm/tests/systemtests/mempool"
	"github.com/cosmos/evm/tests/systemtests/suite"

	"github.com/cosmos/cosmos-sdk/tools/systemtests"
)

func TestMain(m *testing.M) {
	systemtests.RunTests(m)
}

/*
 * Mempool Tests
 */
func TestMempoolTxsOrdering(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunTxsOrdering, suite.MempoolArgs()...)
}

func TestMempoolTxsReplacement(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunTxsReplacement, suite.MempoolArgs()...)
}

func TestMempoolTxsReplacementWithCosmosTx(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunTxsReplacementWithCosmosTx, suite.MempoolArgs()...)
}

func TestMempoolMixedTxsReplacementLegacyAndDynamicFee(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunMixedTxsReplacementLegacyAndDynamicFee, suite.MempoolMinGasPriceZeroArgs()...)
}

func TestMempoolTxBroadcasting(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunTxBroadcasting, suite.MempoolArgs()...)
}

func TestMempoolMinimumGasPricesZero(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunMinimumGasPricesZero, suite.MempoolArgs()...)
}

func TestMempoolCosmosTxsCompatibility(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunCosmosTxsCompatibility, suite.MempoolArgs()...)
}

func TestMempoolSetCode7702QueuedTxPromotion(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunSetCode7702QueuedTxPromotion, suite.MempoolArgs()...)
}

// /*
// * EIP-712 Tests
// */
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

func TestAccountAbstractionEIP7702SameBlock(t *testing.T) {
	suite.RunWithSharedSuite(t, accountabstraction.RunEIP7702SameBlock)
}

/*
* Chain Upgrade Tests
 */
func TestChainUpgrade(t *testing.T) {
	suite.RunWithSharedSuite(t, chainupgrade.RunChainUpgrade)
}
