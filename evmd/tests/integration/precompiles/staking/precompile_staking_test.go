package staking

import (
	"testing"

	"github.com/stretchr/testify/suite"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/staking"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestStakingPrecompileTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.StakingPrecompileApp](integration.CreateEvmd, "evm.StakingPrecompileApp")
	s := staking.NewPrecompileTestSuite(create)
	suite.Run(t, s)
}

func TestStakingPrecompileIntegrationTestSuite(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.StakingPrecompileApp](integration.CreateEvmd, "evm.StakingPrecompileApp")
	staking.TestPrecompileIntegrationTestSuite(t, create)
}
