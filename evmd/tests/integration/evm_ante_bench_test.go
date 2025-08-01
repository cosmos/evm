package integration

import (
	"testing"

	"github.com/cosmos/evm/tests/integration/ante"
)

func BenchmarkEvmAnteTestSuite(b *testing.B) {
	ante.RunBenchmarkEthGasConsumeDecorator(b, CreateEvmd)
}

func BenchmarkEvmAnteHnadler(b *testing.B) {
	ante.RunBenchmarkAnteHandler(b, CreateEvmd)
}
