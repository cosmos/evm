package ante

import (
	"testing"

	"cosmosevm.io/evmd/tests/integration"

	"github.com/cosmos/evm/tests/integration/ante"
)

func TestAnte_Integration(t *testing.T) {
	ante.TestIntegrationAnteHandler(t, integration.CreateEvmd)
}
