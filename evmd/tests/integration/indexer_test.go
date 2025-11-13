package integration

import (
	"testing"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/indexer"
	testapp "github.com/cosmos/evm/testutil/app"
)

var indexerAppCreator = testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](CreateEvmd, "evm.IntegrationNetworkApp")

func TestKVIndexer(t *testing.T) {
	indexer.TestKVIndexer(t, indexerAppCreator)
}
