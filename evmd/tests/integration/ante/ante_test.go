package ante

import (
	"testing"

	"github.com/cosmos/evm/tests/integration/ante"
)

func TestAnte_Integration(t *testing.T) {
	ante.TestIntegrationAnteHandler(t, evmAppCreator)
}

func BenchmarkAnteHandler(b *testing.B) {
	// Run the benchmark with a mock EVM app
	ante.RunBenchmarkAnteHandler(b, evmAppCreator)
}

func TestValidateHandlerOptions(t *testing.T) {
	ante.RunValidateHandlerOptionsTest(t, evmAppCreator)
}
