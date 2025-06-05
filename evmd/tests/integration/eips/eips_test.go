package eips

import (
	"testing"

	"cosmosevm.io/evmd/tests/integration"

	"github.com/cosmos/evm/tests/integration/eips"
)

func Test_EIPs(t *testing.T) {
	eips.TestEIPs(t, integration.CreateEvmd)
}
