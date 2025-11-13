package staking

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/cosmos-sdk/baseapp"
	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/staking"
	testapp "github.com/cosmos/evm/testutil/app"
)

func stakingAppFactory() func(string, uint64, ...func(*baseapp.BaseApp)) evm.EvmApp {
	return testapp.WrapToEvmApp[evm.StakingPrecompileApp](integration.CreateEvmd, "evm.StakingPrecompileApp")
}

func TestStakingPrecompileTestSuite(t *testing.T) {
	s := staking.NewPrecompileTestSuite(stakingAppFactory())
	suite.Run(t, s)
}

func TestStakingPrecompileIntegrationTestSuite(t *testing.T) {
	staking.TestPrecompileIntegrationTestSuite(t, stakingAppFactory())
}
