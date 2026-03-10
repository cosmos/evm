//go:build system_test

package systemtests

import (
	"testing"

	"github.com/cosmos/evm/tests/systemtests/accountabstraction"
	"github.com/cosmos/evm/tests/systemtests/chainupgrade"
	"github.com/cosmos/evm/tests/systemtests/eip712"

	"github.com/cosmos/evm/tests/systemtests/mempool"
	"github.com/cosmos/evm/tests/systemtests/suite"

	"github.com/cosmos/cosmos-sdk/testutil/systemtests"
)

var (
	exclusiveMempoolNodeArgs = append(
		suite.DefaultNodeArgs(),
	)
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

func TestMempoolMixedTxsReplacementLegacyAndDynamicFee(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunMixedTxsReplacementLegacyAndDynamicFee)
}

func TestMempoolTxBroadcasting(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunTxBroadcasting)
}

func TestMinimumGasPricesZero(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunMinimumGasPricesZero, suite.MinimumGasPriceZeroArgs()...)
}

func TestMempoolCosmosTxsCompatibility(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunCosmosTxsCompatibility)
}

/*
* Exclusive Mempool Tests
 */
func TestExclusiveMempoolTxsOrdering(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunTxsOrdering, suite.ExlcusiveMempoolArgs()...)
}

func TestExclusiveMempoolTxsReplacement(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunTxsReplacement, suite.ExlcusiveMempoolArgs()...)
}

func TestExclusiveMempoolTxsReplacementWithCosmosTx(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunTxsReplacementWithCosmosTx, suite.ExlcusiveMempoolArgs()...)
}

func TestExclusiveMempoolMixedTxsReplacementLegacyAndDynamicFee(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunMixedTxsReplacementLegacyAndDynamicFee, suite.ExlcusiveMempoolMinGasPriceZeroArgs()...)
}

func TestExclusiveMempoolTxBroadcasting(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunTxBroadcasting, suite.ExlcusiveMempoolArgs()...)
}

func TestExclusiveMempoolMinimumGasPricesZero(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunMinimumGasPricesZero, suite.ExlcusiveMempoolArgs()...)
}

func TestExclusiveMempoolCosmosTxsCompatibility(t *testing.T) {
	suite.RunWithSharedSuite(t, mempool.RunCosmosTxsCompatibility, suite.ExlcusiveMempoolArgs()...)
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

/*
* Chain Upgrade Tests
 */
func TestChainUpgrade(t *testing.T) {
	suite.RunWithSharedSuite(t, chainupgrade.RunChainUpgrade)
}
