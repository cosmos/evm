package ibc

import (
	// "math/big"
	"math/big"
	"testing"

	// "github.com/ethereum/go-ethereum/common"
	// "github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/precompiles/ics02"
	"github.com/cosmos/gogoproto/proto"

	// chainutil "github.com/cosmos/evm/testutil"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	// evmante "github.com/cosmos/evm/x/vm/ante"
	// transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	// clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	// sdkmath "cosmossdk.io/math"

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

func (s *ICS02ClientTestSuite) SetupTest() {
	s.coordinator = evmibctesting.NewCoordinator(s.T(), 2, 0, integration.SetupEvmd)
	s.chainA = s.coordinator.GetChain(evmibctesting.GetEvmChainID(1))
	s.chainB = s.coordinator.GetChain(evmibctesting.GetEvmChainID(2))

	evmAppA := s.chainA.App.(*evmd.EVMD)
	s.chainAPrecompile = ics02.NewPrecompile(
		evmAppA.BankKeeper,
		evmAppA.IBCKeeper.ClientKeeper,
	)
	evmAppB := s.chainB.App.(*evmd.EVMD)
	s.chainBPrecompile = ics02.NewPrecompile(
		evmAppB.BankKeeper,
		evmAppB.IBCKeeper.ClientKeeper,
	)
}

func (s *ICS02ClientTestSuite) TestGetClientState() {
	var (
		clientID string
		expClientState []byte
		expErr error
	)

	testCases := []struct {
		name string
		malleate func()
	}{
		{
			name: "success",
			malleate: func() {
				clientID = ibctesting.FirstClientID
				clientState, found := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientState(
					s.chainA.GetContext(),
					clientID,
				)
				s.Require().True(found)
				
				expClientState, expErr = proto.Marshal(clientState)
				s.Require().NoError(expErr)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			pathBToA := evmibctesting.NewTransferPath(s.chainB, s.chainA)
			pathBToA.Setup()

			senderIdx := 1
			senderAccount := s.chainA.SenderAccounts[senderIdx]

			// setup
			tc.malleate()

			calldata, err := s.chainAPrecompile.Pack(ics02.GetClientStateMethod, clientID)
			s.Require().NoError(err)

			_, _, resp, err := s.chainA.SendEvmTx(senderAccount, senderIdx, s.chainAPrecompile.Address(), big.NewInt(0), calldata, 100_000)
			if expErr != nil {
				s.Require().ErrorContains(err, expErr.Error())
				return
			}
			s.Require().NoError(err)

			// decode result
			out, err := s.chainAPrecompile.Unpack(ics02.GetClientStateMethod, resp.Ret)
			s.Require().NoError(err)

			clientStateBz, ok := out[0].([]byte)
			s.Require().True(ok)
			s.Require().Equal(expClientState, clientStateBz)
		})
	}
}

func TestICS02ClientTestSuite(t *testing.T) {
	suite.Run(t, new(ICS02ClientTestSuite))
}
