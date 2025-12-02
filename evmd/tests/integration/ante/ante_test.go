package ante

import (
	"testing"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/ante"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestAnte_Integration(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.AnteIntegrationApp](CreateEvmd, "evm.AnteIntegrationApp")
	ante.TestIntegrationAnteHandler(t, create)
}

func TestAnte_IntegrationWithBlockSTM(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.AnteIntegrationApp](CreateEvmdWithBlockSTM, "evm.AnteIntegrationApp")
	ante.TestIntegrationAnteHandler(t, create)
}

func BenchmarkAnteHandler(b *testing.B) {
	create := testapp.ToEvmAppCreator[evm.AnteIntegrationApp](CreateEvmd, "evm.AnteIntegrationApp")
	// Run the benchmark with a mock EVM app
	ante.RunBenchmarkAnteHandler(b, create)
}

func BenchmarkAnteHandlerWithBlockSTM(b *testing.B) {
	create := testapp.ToEvmAppCreator[evm.AnteIntegrationApp](CreateEvmdWithBlockSTM, "evm.AnteIntegrationApp")
	// Run the benchmark with a mock EVM app
	ante.RunBenchmarkAnteHandler(b, create)
}

func TestValidateHandlerOptions(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.AnteIntegrationApp](CreateEvmd, "evm.AnteIntegrationApp")
	ante.RunValidateHandlerOptionsTest(t, create)
}

func TestValidateHandlerOptionsWithBlockSTM(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.AnteIntegrationApp](CreateEvmdWithBlockSTM, "evm.AnteIntegrationApp")
	ante.RunValidateHandlerOptionsTest(t, create)
}
