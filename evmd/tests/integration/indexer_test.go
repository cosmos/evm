package integration

import (
	"testing"

	"github.com/cosmos/evm/tests/integration/indexer"
)

func TestKVIndexer(t *testing.T) {
	indexer.TestKVIndexer(t, CreateEvmd)
}
