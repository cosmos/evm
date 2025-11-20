package eips_test

import (
	"testing"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/eips"
	testapp "github.com/cosmos/evm/testutil/app"
	//nolint:revive // dot imports are fine for Ginkgo
	//nolint:revive // dot imports are fine for Ginkgo
)

func TestEIPs(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](integration.CreateEvmd, "evm.IntegrationNetworkApp")
	eips.RunTests(t, create)
}
