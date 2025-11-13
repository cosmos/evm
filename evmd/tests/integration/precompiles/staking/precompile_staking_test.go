package staking

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/cosmos-sdk/baseapp"
	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/tests/integration/precompiles/staking"
)

func TestStakingPrecompileTestSuite(t *testing.T) {
	create := func(chainID string, evmChainID uint64, customBaseAppOptions ...func(*baseapp.BaseApp)) evm.StakingPrecompileApp {
		return integration.CreateEvmd(chainID, evmChainID, customBaseAppOptions...).(evm.StakingPrecompileApp)
	}
	s := staking.NewPrecompileTestSuite(create)
	suite.Run(t, s)
}

func TestStakingPrecompileIntegrationTestSuite(t *testing.T) {
	create := func(chainID string, evmChainID uint64, customBaseAppOptions ...func(*baseapp.BaseApp)) evm.StakingPrecompileApp {
		return integration.CreateEvmd(chainID, evmChainID, customBaseAppOptions...).(evm.StakingPrecompileApp)
	}
	staking.TestPrecompileIntegrationTestSuite(t, create)
}
