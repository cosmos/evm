package ibc

import (
	// "math/big"
	"testing"

	// "github.com/ethereum/go-ethereum/common"
	// "github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/precompiles/ics02"
	// chainutil "github.com/cosmos/evm/testutil"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	// evmante "github.com/cosmos/evm/x/vm/ante"
	// transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	// clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	// sdkmath "cosmossdk.io/math"
	//
	// sdk "github.com/cosmos/cosmos-sdk/types"
	// "github.com/cosmos/cosmos-sdk/types/query"
)

type ICS02ClientTestSuite struct {
	suite.Suite

	coordinator *evmibctesting.Coordinator

	// testing chains used for convenience and readability
	chainA           *evmibctesting.TestChain
	chainAPrecompile *ics02.Precompile
	chainB           *evmibctesting.TestChain
	chainBPrecompile *ics02.Precompile
}

func (suite *ICS02ClientTestSuite) SetupTest() {
	suite.coordinator = evmibctesting.NewCoordinator(suite.T(), 2, 0, integration.SetupEvmd)
	suite.chainA = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(1))
	suite.chainB = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(2))

	evmAppA := suite.chainA.App.(*evmd.EVMD)
	suite.chainAPrecompile = ics02.NewPrecompile(
		evmAppA.BankKeeper,
		evmAppA.IBCKeeper.ClientKeeper,
	)
	evmAppB := suite.chainB.App.(*evmd.EVMD)
	suite.chainBPrecompile = ics02.NewPrecompile(
		evmAppB.BankKeeper,
		evmAppB.IBCKeeper.ClientKeeper,
	)
}

// Constructs the following sends based on the established channels/connections
// 1 - from evmChainA to chainB
// func (suite *ICS02ClientTestSuite) TestGetClientStatus() {
// }

func TestICS02ClientTestSuite(t *testing.T) {
	suite.Run(t, new(ICS02ClientTestSuite))
}
