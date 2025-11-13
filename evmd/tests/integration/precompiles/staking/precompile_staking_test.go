package staking

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/staking"
	testapp "github.com/cosmos/evm/testutil/app"
)

var evmAppCreator = testapp.ToEvmAppCreator[evm.StakingPrecompileApp](integration.CreateEvmd, "evm.StakingPrecompileApp")

func TestStakingPrecompileTestSuite(t *testing.T) {
	s := staking.NewPrecompileTestSuite(evmAppCreator)
	suite.Run(t, s)
}

func TestStakingPrecompileIntegrationTestSuite(t *testing.T) {
	staking.TestPrecompileIntegrationTestSuite(t, evmAppCreator)
}
