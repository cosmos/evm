//go:build system_test

package systemtests

import (
	"cosmossdk.io/systemtests"
	"github.com/cosmos/evm/tests/systemtests/mempool"
	"testing"
)

func TestMain(m *testing.M) {
	systemtests.RunTests(m)
}

//
///*
//* Mempool Tests
// */
//// Mempool Tests
//func TestTxsOrdering(t *testing.T) {
//	mempool.TestTxsOrdering(t)
//}
//
//func TestTxsReplacement(t *testing.T) {
//	mempool.TestTxsReplacement(t)
//	mempool.TestTxsReplacementWithCosmosTx(t)
//	mempool.TestMixedTxsReplacementLegacyAndDynamicFee(t)
//}
//
//func TestExceptions(t *testing.T) {
//	mempool.RunMinimumGasPricesZero(t)
//}
//
//// Account Abstraction Tests
//func TestEIP7702(t *testing.T) {
//	accountabstraction.TestEIP7702(t)
//}

func TestMempoolTxBroadcasting(t *testing.T) {
	mempool.RunTxBroadcasting(t)
}

//	func TestMempoolCosmosTxsCompatibility(t *testing.T) {
//		mempool.TestCosmosTxsCompatibility(t)
//	}
//
// // /*
// // * EIP-712 Tests
// // */
//func TestEIP712BankSend(t *testing.T) {
//	eip712.TestEIP712BankSend(t)
//}

//
//func TestEIP712BankSendWithBalanceCheck(t *testing.T) {
//	eip712.TestEIP712BankSendWithBalanceCheck(t)
//}
//
//func TestEIP712MultipleBankSends(t *testing.T) {
//	eip712.TestEIP712MultipleBankSends(t)
//}
//
///*
//* Account Abstraction Tests
// */
//func TestAccountAbstractionEIP7702(t *testing.T) {
//	accountabstraction.TestEIP7702(t)
//}
