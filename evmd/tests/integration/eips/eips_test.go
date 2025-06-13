package eips

import (
	"testing"

	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/eips"
)

func Test_EIPs(t *testing.T) {
	eips.TestEIPs(t, integration.CreateEvmd)
}
