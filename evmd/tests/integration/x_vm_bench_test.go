package integration

import (
	"testing"

	"github.com/cosmos/evm/tests/integration/x/vm"
)

func BenchmarkApplyTransaction(b *testing.B) {
	vm.BenchmarkApplyTransaction(b, CreateEvmd)
}

func BenchmarkApplyTransactionWithLegacyTx(b *testing.B) {
	vm.BenchmarkApplyTransactionWithLegacyTx(b, CreateEvmd)
}

func BenchmarkApplyTransactionWithDynamicFeeTx(b *testing.B) {
	vm.BenchmarkApplyTransactionWithDynamicFeeTx(b, CreateEvmd)
}

func BenchmarkApplyMessage(b *testing.B) {
	vm.BenchmarkApplyMessage(b, CreateEvmd)
}

func BenchmarkApplyMessageWithLegacyTx(b *testing.B) {
	vm.BenchmarkApplyMessageWithLegacyTx(b, CreateEvmd)
}

func BenchmarkApplyMessageWithDynamicFeeTx(b *testing.B) {
	vm.BenchmarkApplyMessageWithDynamicFeeTx(b, CreateEvmd)
}

func BenchmarkTokenTransfer(b *testing.B) {
	vm.BenchmarkTokenTransfer(b, CreateEvmd)
}

func BenchmarkEmitLogs(b *testing.B) {
	vm.BenchmarkEmitLogs(b, CreateEvmd)
}

func BenchmarkTokenTransferFrom(b *testing.B) {
	vm.BenchmarkTokenTransferFrom(b, CreateEvmd)
}

func BenchmarkTokenMint(b *testing.B) {
	vm.BenchmarkTokenMint(b, CreateEvmd)
}

func BenchmarkMessageCall(b *testing.B) {
	vm.BenchmarkMessageCall(b, CreateEvmd)
}
