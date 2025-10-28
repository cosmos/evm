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
	ibctesting "github.com/cosmos/ibc-go/v10/testing"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	// sdkmath "cosmossdk.io/math"
	// sdk "github.com/cosmos/cosmos-sdk/types"
	// "github.com/cosmos/cosmos-sdk/types/query"
	// codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

type ICS02ClientTestSuite struct {
	suite.Suite

	coordinator *evmibctesting.Coordinator

	// testing chains used for convenience and readability
	chainA           *evmibctesting.TestChain
	chainAPrecompile *ics02.Precompile
	chainB           *evmibctesting.TestChain
	chainBPrecompile *ics02.Precompile

	pathBToA *evmibctesting.Path
}

func (s *ICS02ClientTestSuite) SetupTest() {
	s.coordinator = evmibctesting.NewCoordinator(s.T(), 2, 0, integration.SetupEvmd)
	s.chainA = s.coordinator.GetChain(evmibctesting.GetEvmChainID(1))
	s.chainB = s.coordinator.GetChain(evmibctesting.GetEvmChainID(2))

	s.pathBToA = evmibctesting.NewTransferPath(s.chainB, s.chainA)
	s.pathBToA.Setup()


	evmAppA := s.chainA.App.(*evmd.EVMD)
	s.chainAPrecompile = ics02.NewPrecompile(
		evmAppA.AppCodec(),
		evmAppA.BankKeeper,
		evmAppA.IBCKeeper.ClientKeeper,
	)
	evmAppB := s.chainB.App.(*evmd.EVMD)
	s.chainBPrecompile = ics02.NewPrecompile(
		evmAppA.AppCodec(),
		evmAppB.BankKeeper,
		evmAppB.IBCKeeper.ClientKeeper,
	)
}

func (s *ICS02ClientTestSuite) TestGetClientState() {
	var (
		clientID string
		expClientState []byte
		expErr bool
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
				
				var err error
				expClientState, err = proto.Marshal(clientState)
				s.Require().NoError(err)
			},
		},
		{
			name: "failure: client not found",
			malleate: func() {
				clientID = ibctesting.InvalidID
				expErr = true
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// == reset test state ==
			s.SetupTest()

			clientID = ""
			expClientState = nil
			expErr = false
			// ====

			senderIdx := 1
			senderAccount := s.chainA.SenderAccounts[senderIdx]

			// setup
			tc.malleate()

			calldata, err := s.chainAPrecompile.Pack(ics02.GetClientStateMethod, clientID)
			s.Require().NoError(err)

			_, _, resp, err := s.chainA.SendEvmTx(senderAccount, senderIdx, s.chainAPrecompile.Address(), big.NewInt(0), calldata, 100_000)
			if expErr {
				s.Require().Error(err)
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

func (s *ICS02ClientTestSuite) TestUpdateClient() {
	var (
		clientID string
		expResult uint8
		calldata []byte
		expErr bool
	)

	testCases := []struct {
		name string
		malleate func()
	}{
		{
			name: "success: update client",
			malleate: func() {
				clientID = ibctesting.FirstClientID
				expResult = ics02.UpdateResultSuccess
				// == construct update header ==
				// 1. Update chain B to have new header
				s.chainB.Coordinator.CommitBlock(s.chainB, s.chainA)
				// 2. Construct update header
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					clientID,
				)
				var (
				)
				header, err := s.pathBToA.EndpointA.Chain.IBCClientHeader(s.pathBToA.EndpointA.Chain.LatestCommittedHeader, trustedHeight)
				s.Require().NoError(err)
	
				anyHeader, err := clienttypes.PackClientMessage(header)
				s.Require().NoError(err)

				updateBz, err := anyHeader.Marshal()
				s.Require().NoError(err)

				calldata, err = s.chainAPrecompile.Pack(ics02.UpdateClientMethod, clientID, updateBz)
				s.Require().NoError(err)
				// ====

				expResult = ics02.UpdateResultSuccess
			},
		},
		{
			name: "success: noop",
			malleate: func() {
				clientID = ibctesting.FirstClientID
				expResult = ics02.UpdateResultSuccess
				// == construct update header ==
				// 1. Update chain B to have new header
				s.chainB.Coordinator.CommitBlock(s.chainB, s.chainA)
				// 2. Construct update header
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					clientID,
				)
				var (
				)
				header, err := s.pathBToA.EndpointA.Chain.IBCClientHeader(s.pathBToA.EndpointA.Chain.LatestCommittedHeader, trustedHeight)
				s.Require().NoError(err)
	
				anyHeader, err := clienttypes.PackClientMessage(header)
				s.Require().NoError(err)

				updateBz, err := anyHeader.Marshal()
				s.Require().NoError(err)

				calldata, err = s.chainAPrecompile.Pack(ics02.UpdateClientMethod, clientID, updateBz)
				s.Require().NoError(err)
				// ====

				err = s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.UpdateClient(s.chainA.GetContext(), clientID, header)
				s.Require().NoError(err)

				// TODO: right now, precompile always returns UpdateResultSuccess even on noop
				// This can be improved in future to actually detect noop and return UpdateResultNoop
				expResult = ics02.UpdateResultSuccess
			},
		},
		{
			name: "failure: client not found",
			malleate: func() {
				clientID = ibctesting.InvalidID
				expErr = true
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// == reset test state ==
			s.SetupTest()

			clientID = ""
			expResult = 0
			expErr = false
			// ====

			senderIdx := 1
			senderAccount := s.chainA.SenderAccounts[senderIdx]

			// setup
			tc.malleate()

			_, _, resp, err := s.chainA.SendEvmTx(senderAccount, senderIdx, s.chainAPrecompile.Address(), big.NewInt(0), calldata, 100_000)
			if expErr {
				s.Require().Error(err)
				return
			}
			if err != nil {
				s.FailNow(resp.VmError)
			}

			// decode result
			out, err := s.chainAPrecompile.Unpack(ics02.UpdateClientMethod, resp.Ret)
			s.Require().NoError(err)

			res, ok := out[0].(uint8)
			s.Require().True(ok)
			s.Require().Equal(expResult, res)
		})
	}
}

func TestICS02ClientTestSuite(t *testing.T) {
	suite.Run(t, new(ICS02ClientTestSuite))
}
