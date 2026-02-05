package ibc

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/precompiles/ics02"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	"github.com/cosmos/gogoproto/proto"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	commitmenttypesv2 "github.com/cosmos/ibc-go/v10/modules/core/23-commitment/types/v2"
	ibchost "github.com/cosmos/ibc-go/v10/modules/core/24-host"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	ibctm "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"
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
		evmAppA.IBCKeeper.ClientKeeper,
	)
	evmAppB := s.chainB.App.(*evmd.EVMD)
	s.chainBPrecompile = ics02.NewPrecompile(
		evmAppA.AppCodec(),
		evmAppB.IBCKeeper.ClientKeeper,
	)
}

func (s *ICS02ClientTestSuite) TestGetClientState() {
	var (
		calldata       []byte
		expClientState []byte
		expErr         bool
	)

	testCases := []struct {
		name     string
		malleate func()
	}{
		{
			name: "success",
			malleate: func() {
				clientID := ibctesting.FirstClientID
				clientState, found := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientState(
					s.chainA.GetContext(),
					clientID,
				)
				s.Require().True(found)

				var err error
				calldata, err = s.chainAPrecompile.Pack(ics02.GetClientStateMethod, clientID)
				s.Require().NoError(err)

				expClientState, err = proto.Marshal(clientState)
				s.Require().NoError(err)
			},
		},
		{
			name: "failure: invalid client ID",
			malleate: func() {
				var err error
				calldata, err = s.chainAPrecompile.Pack(ics02.GetClientStateMethod, ibctesting.InvalidID)
				s.Require().NoError(err)
				expErr = true
			},
		},
		{
			name: "failure: client not found",
			malleate: func() {
				var err error
				calldata, err = s.chainAPrecompile.Pack(ics02.GetClientStateMethod, ibctesting.SecondClientID)
				s.Require().NoError(err)
				expErr = true
			},
		},
		{
			name: "failure: invalid calldata",
			malleate: func() {
				calldata = []byte(ibctesting.InvalidID)
				expErr = true
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// == reset test state ==
			s.SetupTest()

			expClientState = nil
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
		expResult uint8
		calldata  []byte
		expErr    bool
	)

	testCases := []struct {
		name     string
		malleate func()
	}{
		{
			name: "success: update client",
			malleate: func() {
				clientID := ibctesting.FirstClientID
				// == construct update header ==
				// 1. Update chain B to have new header
				s.chainB.Coordinator.CommitBlock(s.chainB, s.chainA)
				// 2. Construct update header
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					clientID,
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
				clientID := ibctesting.FirstClientID
				// == construct update header ==
				// 1. Update chain B to have new header
				s.chainB.Coordinator.CommitBlock(s.chainB, s.chainA)
				// 2. Construct update header
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					clientID,
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

				expResult = ics02.UpdateResultSuccess
			},
		},
		{
			name: "success: valid fork misbehaviour",
			malleate: func() {
				clientID := ibctesting.FirstClientID
				// == construct update header ==
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					clientID,
				)
				trustedVals, ok := s.chainB.TrustedValidators[trustedHeight.RevisionHeight]
				s.Require().True(ok)

				err := s.pathBToA.EndpointB.UpdateClient()
				s.Require().NoError(err)

				height := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					clientID,
				)

				misbehaviour := &ibctm.Misbehaviour{
					ClientId: clientID,
					Header1:  s.chainB.CreateTMClientHeader(s.chainB.ChainID, int64(height.RevisionHeight), trustedHeight, s.chainB.ProposedHeader.Time.Add(time.Minute), s.chainB.Vals, s.chainB.NextVals, trustedVals, s.chainB.Signers),
					Header2:  s.chainB.CreateTMClientHeader(s.chainB.ChainID, int64(height.RevisionHeight), trustedHeight, s.chainB.ProposedHeader.Time, s.chainB.Vals, s.chainB.NextVals, trustedVals, s.chainB.Signers),
				}

				anyMisbehavior, err := clienttypes.PackClientMessage(misbehaviour)
				s.Require().NoError(err)

				updateBz, err := anyMisbehavior.Marshal()
				s.Require().NoError(err)

				calldata, err = s.chainAPrecompile.Pack(ics02.UpdateClientMethod, clientID, updateBz)
				s.Require().NoError(err)
				// ====

				expResult = ics02.UpdateResultMisbehaviour
			},
		},
		{
			name: "failure: invalid client ID",
			malleate: func() {
				clientID := ibctesting.FirstClientID
				// == construct update header ==
				// 1. Update chain B to have new header
				s.chainB.Coordinator.CommitBlock(s.chainB, s.chainA)
				// 2. Construct update header
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					clientID,
				)
				header, err := s.pathBToA.EndpointA.Chain.IBCClientHeader(s.pathBToA.EndpointA.Chain.LatestCommittedHeader, trustedHeight)
				s.Require().NoError(err)

				anyHeader, err := clienttypes.PackClientMessage(header)
				s.Require().NoError(err)

				updateBz, err := anyHeader.Marshal()
				s.Require().NoError(err)

				// use invalid client ID
				calldata, err = s.chainAPrecompile.Pack(ics02.UpdateClientMethod, ibctesting.InvalidID, updateBz)
				s.Require().NoError(err)
				// ====
				expErr = true
			},
		},
		{
			name: "failure: invalid client message",
			malleate: func() {
				clientID := ibctesting.FirstClientID

				var err error
				calldata, err = s.chainAPrecompile.Pack(ics02.UpdateClientMethod, clientID, []byte(ibctesting.InvalidID))
				s.Require().NoError(err)
				// ====
				expErr = true
			},
		},
		{
			name: "failure: invalid calldata",
			malleate: func() {
				calldata = []byte(ibctesting.InvalidID)
				expErr = true
			},
		},
		{
			name: "failure: invalid header update",
			malleate: func() {
				clientID := ibctesting.FirstClientID
				// == construct update header ==
				// 1. Update chain B to have new header
				s.chainB.Coordinator.CommitBlock(s.chainB, s.chainA)
				// 2. Construct update header
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					clientID,
				)
				header, err := s.pathBToA.EndpointA.Chain.IBCClientHeader(s.pathBToA.EndpointA.Chain.LatestCommittedHeader, trustedHeight)
				s.Require().NoError(err)

				// modify header to be invalid
				header.Header.Time = header.Header.Time.Add(10 * time.Second)

				anyHeader, err := clienttypes.PackClientMessage(header)
				s.Require().NoError(err)

				updateBz, err := anyHeader.Marshal()
				s.Require().NoError(err)

				calldata, err = s.chainAPrecompile.Pack(ics02.UpdateClientMethod, clientID, updateBz)
				s.Require().NoError(err)
				// ====

				expErr = true
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// == reset test state ==
			s.SetupTest()

			expResult = 0
			expErr = false
			calldata = nil
			// ====

			senderIdx := 1
			senderAccount := s.chainA.SenderAccounts[senderIdx]

			// setup
			tc.malleate()

			_, _, resp, err := s.chainA.SendEvmTx(senderAccount, senderIdx, s.chainAPrecompile.Address(), big.NewInt(0), calldata, 200_000)
			if expErr {
				s.Require().Error(err)
				return
			}
			if err != nil {
				s.FailNow("failed to send tx", "error: %v", err, "vmerror: %v", resp.VmError)
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

func (s *ICS02ClientTestSuite) TestVerifyMembership() {
	var (
		calldata  []byte
		expErr    bool
		expResult *big.Int
	)

	testCases := []struct {
		name     string
		malleate func()
	}{
		{
			name: "success: prove membership of clientState",
			malleate: func() {
				clientID := ibctesting.FirstClientID
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					clientID,
				)

				clientKey := ibchost.FullClientStateKey(clientID)
				clientProof, proofHeight := s.pathBToA.EndpointA.QueryProofAtHeight(clientKey, trustedHeight.RevisionHeight)

				// get pure value from chain B
				res, err := s.chainB.App.Query(
					s.chainB.GetContext().Context(),
					&abci.RequestQuery{
						Path:   fmt.Sprintf("store/%s/key", ibcexported.StoreKey),
						Height: int64(trustedHeight.RevisionHeight - 1), //nolint:gosec
						Data:   clientKey,
					})
				s.Require().NoError(err)
				value := res.Value

				pathBz := [][]byte{[]byte(ibcexported.StoreKey), clientKey}
				calldata, err = s.chainAPrecompile.Pack(ics02.VerifyMembershipMethod,
					clientID,
					clientProof,
					trustedHeight,
					pathBz,
					value,
				)
				s.Require().NoError(err)

				timestampNano, err := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientTimestampAtHeight(s.chainA.GetContext(), clientID, proofHeight)
				s.Require().NoError(err)

				expResult = big.NewInt(int64(timestampNano / 1_000_000_000)) //nolint:gosec

				// verify membership on-chain to ensure proof is valid
				path := commitmenttypesv2.NewMerklePath(pathBz...)
				err = s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.VerifyMembership(
					s.chainA.GetContext(),
					clientID,
					proofHeight,
					0,
					0,
					clientProof,
					path,
					value,
				)
				s.Require().NoError(err)
			},
		},
		{
			name: "failure: pass non-membership proof as membership proof",
			malleate: func() {
				existingClientID := ibctesting.FirstClientID
				missingClientID := ibctesting.SecondClientID // NOTE: use a non-existent client ID
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					existingClientID,
				)

				clientKey := ibchost.FullClientStateKey(missingClientID)
				clientProof, _ := s.pathBToA.EndpointA.QueryProofAtHeight(clientKey, trustedHeight.RevisionHeight)

				// get pure value from chain B (for the existing client)
				res, err := s.chainB.App.Query(
					s.chainB.GetContext().Context(),
					&abci.RequestQuery{
						Path:   fmt.Sprintf("store/%s/key", ibcexported.StoreKey),
						Height: int64(trustedHeight.RevisionHeight - 1), //nolint:gosec
						Data:   ibchost.FullClientStateKey(existingClientID),
					})
				s.Require().NoError(err)
				value := res.Value

				pathBz := [][]byte{[]byte(ibcexported.StoreKey), clientKey}
				calldata, err = s.chainAPrecompile.Pack(ics02.VerifyMembershipMethod,
					existingClientID,
					clientProof,
					trustedHeight,
					pathBz,
					value,
				)
				s.Require().NoError(err)

				expErr = true
			},
		},
		{
			name: "failure: invalid client ID",
			malleate: func() {
				clientID := ibctesting.FirstClientID
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					clientID,
				)

				clientKey := ibchost.FullClientStateKey(clientID)
				clientProof, _ := s.pathBToA.EndpointA.QueryProofAtHeight(clientKey, trustedHeight.RevisionHeight)

				// get pure value from chain B
				res, err := s.chainB.App.Query(
					s.chainB.GetContext().Context(),
					&abci.RequestQuery{
						Path:   fmt.Sprintf("store/%s/key", ibcexported.StoreKey),
						Height: int64(trustedHeight.RevisionHeight - 1), //nolint:gosec
						Data:   clientKey,
					})
				s.Require().NoError(err)
				value := res.Value

				pathBz := [][]byte{[]byte(ibcexported.StoreKey), clientKey}
				calldata, err = s.chainAPrecompile.Pack(ics02.VerifyMembershipMethod,
					ibctesting.InvalidID, // use invalid client ID
					clientProof,
					trustedHeight,
					pathBz,
					value,
				)
				s.Require().NoError(err)

				expErr = true
			},
		},
		{
			name: "failure: invalid proof",
			malleate: func() {
				clientID := ibctesting.FirstClientID
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					clientID,
				)

				clientKey := ibchost.FullClientStateKey(clientID)
				// get pure value from chain B
				res, err := s.chainB.App.Query(
					s.chainB.GetContext().Context(),
					&abci.RequestQuery{
						Path:   fmt.Sprintf("store/%s/key", ibcexported.StoreKey),
						Height: int64(trustedHeight.RevisionHeight - 1), //nolint:gosec
						Data:   clientKey,
					})
				s.Require().NoError(err)
				value := res.Value

				pathBz := [][]byte{[]byte(ibcexported.StoreKey), clientKey}
				calldata, err = s.chainAPrecompile.Pack(ics02.VerifyMembershipMethod,
					clientID,
					[]byte(ibctesting.InvalidID), // use invalid client proof
					trustedHeight,
					pathBz,
					value,
				)
				s.Require().NoError(err)

				expErr = true
			},
		},
		{
			name: "failure: invalid height",
			malleate: func() {
				clientID := ibctesting.FirstClientID
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					clientID,
				)

				clientKey := ibchost.FullClientStateKey(clientID)
				clientProof, _ := s.pathBToA.EndpointA.QueryProofAtHeight(clientKey, trustedHeight.RevisionHeight)

				// get pure value from chain B
				res, err := s.chainB.App.Query(
					s.chainB.GetContext().Context(),
					&abci.RequestQuery{
						Path:   fmt.Sprintf("store/%s/key", ibcexported.StoreKey),
						Height: int64(trustedHeight.RevisionHeight - 1), //nolint:gosec
						Data:   clientKey,
					})
				s.Require().NoError(err)
				value := res.Value

				pathBz := [][]byte{[]byte(ibcexported.StoreKey), clientKey}
				calldata, err = s.chainAPrecompile.Pack(ics02.VerifyMembershipMethod,
					clientID,
					clientProof,
					clienttypes.NewHeight(69, 420), // use invalid height
					pathBz,
					value,
				)
				s.Require().NoError(err)

				expErr = true
			},
		},
		{
			name: "failure: invalid path",
			malleate: func() {
				clientID := ibctesting.FirstClientID
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					clientID,
				)

				clientKey := ibchost.FullClientStateKey(clientID)
				clientProof, _ := s.pathBToA.EndpointA.QueryProofAtHeight(clientKey, trustedHeight.RevisionHeight)

				// get pure value from chain B
				res, err := s.chainB.App.Query(
					s.chainB.GetContext().Context(),
					&abci.RequestQuery{
						Path:   fmt.Sprintf("store/%s/key", ibcexported.StoreKey),
						Height: int64(trustedHeight.RevisionHeight - 1), //nolint:gosec
						Data:   clientKey,
					})
				s.Require().NoError(err)
				value := res.Value

				pathBz := [][]byte{[]byte(ibctesting.InvalidID), clientKey} // use invalid path
				calldata, err = s.chainAPrecompile.Pack(ics02.VerifyMembershipMethod,
					clientID,
					clientProof,
					trustedHeight,
					pathBz,
					value,
				)
				s.Require().NoError(err)

				expErr = true
			},
		},
		{
			name: "failure: invalid value",
			malleate: func() {
				clientID := ibctesting.FirstClientID
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					clientID,
				)

				clientKey := ibchost.FullClientStateKey(clientID)
				clientProof, _ := s.pathBToA.EndpointA.QueryProofAtHeight(clientKey, trustedHeight.RevisionHeight)

				pathBz := [][]byte{[]byte(ibcexported.StoreKey), clientKey}
				var err error
				calldata, err = s.chainAPrecompile.Pack(ics02.VerifyMembershipMethod,
					clientID,
					clientProof,
					trustedHeight,
					pathBz,
					[]byte(ibctesting.InvalidID), // use invalid value
				)
				s.Require().NoError(err)

				expErr = true
			},
		},
		{
			name: "failure: invalid calldata",
			malleate: func() {
				calldata = []byte(ibctesting.InvalidID)
				expErr = true
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// == reset test state ==
			s.SetupTest()

			expErr = false
			calldata = nil
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
			s.Require().NoError(err)

			// decode result
			out, err := s.chainAPrecompile.Unpack(ics02.VerifyMembershipMethod, resp.Ret)
			s.Require().NoError(err)

			res, ok := out[0].(*big.Int)
			s.Require().True(ok)
			s.Require().Equal(expResult, res)
		})
	}
}

func (s *ICS02ClientTestSuite) TestVerifyNonMembership() {
	var (
		calldata  []byte
		expErr    bool
		expResult *big.Int
	)

	testCases := []struct {
		name     string
		malleate func()
	}{
		{
			name: "success: prove non-membership of clientState",
			malleate: func() {
				existingClientID := ibctesting.FirstClientID
				missingClientID := ibctesting.SecondClientID // NOTE: use a non-existent client ID
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					existingClientID,
				)

				clientKey := ibchost.FullClientStateKey(missingClientID)
				clientProof, proofHeight := s.pathBToA.EndpointA.QueryProofAtHeight(clientKey, trustedHeight.RevisionHeight)

				pathBz := [][]byte{[]byte(ibcexported.StoreKey), clientKey}
				var err error
				calldata, err = s.chainAPrecompile.Pack(ics02.VerifyNonMembershipMethod,
					existingClientID,
					clientProof,
					trustedHeight,
					pathBz,
				)
				s.Require().NoError(err)

				timestampNano, err := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientTimestampAtHeight(s.chainA.GetContext(), existingClientID, proofHeight)
				s.Require().NoError(err)

				expResult = big.NewInt(int64(timestampNano / 1_000_000_000)) //nolint:gosec

				// verify non-membership on-chain to ensure proof is valid
				path := commitmenttypesv2.NewMerklePath(pathBz...)
				err = s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.VerifyNonMembership(
					s.chainA.GetContext(),
					existingClientID,
					proofHeight,
					0,
					0,
					clientProof,
					path,
				)
				s.Require().NoError(err)
			},
		},
		{
			name: "failure: pass membership proof as non-membership proof",
			malleate: func() {
				clientID := ibctesting.FirstClientID
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					clientID,
				)

				clientKey := ibchost.FullClientStateKey(clientID)
				clientProof, _ := s.pathBToA.EndpointA.QueryProofAtHeight(clientKey, trustedHeight.RevisionHeight)

				pathBz := [][]byte{[]byte(ibcexported.StoreKey), clientKey}
				var err error
				calldata, err = s.chainAPrecompile.Pack(ics02.VerifyNonMembershipMethod,
					clientID,
					clientProof,
					trustedHeight,
					pathBz,
				)
				s.Require().NoError(err)

				expErr = true
			},
		},
		{
			name: "failure: invalid client ID",
			malleate: func() {
				existingClientID := ibctesting.FirstClientID
				missingClientID := ibctesting.SecondClientID // NOTE: use a non-existent client ID
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					existingClientID,
				)

				clientKey := ibchost.FullClientStateKey(missingClientID)
				clientProof, _ := s.pathBToA.EndpointA.QueryProofAtHeight(clientKey, trustedHeight.RevisionHeight)

				pathBz := [][]byte{[]byte(ibcexported.StoreKey), clientKey}
				var err error
				calldata, err = s.chainAPrecompile.Pack(ics02.VerifyNonMembershipMethod,
					ibctesting.InvalidID, // use invalid client ID
					clientProof,
					trustedHeight,
					pathBz,
				)
				s.Require().NoError(err)

				expErr = true
			},
		},
		{
			name: "failure: invalid proof",
			malleate: func() {
				existingClientID := ibctesting.FirstClientID
				missingClientID := ibctesting.SecondClientID // NOTE: use a non-existent client ID
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					existingClientID,
				)

				clientKey := ibchost.FullClientStateKey(missingClientID)
				pathBz := [][]byte{[]byte(ibcexported.StoreKey), clientKey}
				var err error
				calldata, err = s.chainAPrecompile.Pack(ics02.VerifyNonMembershipMethod,
					existingClientID,
					[]byte(ibctesting.InvalidID), // use invalid client proof
					trustedHeight,
					pathBz,
				)
				s.Require().NoError(err)

				expErr = true
			},
		},
		{
			name: "failure: invalid height",
			malleate: func() {
				existingClientID := ibctesting.FirstClientID
				missingClientID := ibctesting.SecondClientID // NOTE: use a non-existent client ID
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					existingClientID,
				)

				clientKey := ibchost.FullClientStateKey(missingClientID)
				clientProof, _ := s.pathBToA.EndpointA.QueryProofAtHeight(clientKey, trustedHeight.RevisionHeight)

				pathBz := [][]byte{[]byte(ibcexported.StoreKey), clientKey}
				var err error
				calldata, err = s.chainAPrecompile.Pack(ics02.VerifyNonMembershipMethod,
					existingClientID,
					clientProof,
					clienttypes.NewHeight(69, 420), // use invalid height
					pathBz,
				)
				s.Require().NoError(err)

				expErr = true
			},
		},
		{
			name: "failure: invalid path",
			malleate: func() {
				existingClientID := ibctesting.FirstClientID
				missingClientID := ibctesting.SecondClientID // NOTE: use a non-existent client ID
				trustedHeight := s.chainA.App.(*evmd.EVMD).IBCKeeper.ClientKeeper.GetClientLatestHeight(
					s.chainA.GetContext(),
					existingClientID,
				)

				clientKey := ibchost.FullClientStateKey(missingClientID)
				clientProof, _ := s.pathBToA.EndpointA.QueryProofAtHeight(clientKey, trustedHeight.RevisionHeight)

				pathBz := [][]byte{[]byte(ibctesting.InvalidID), clientKey} // use invalid path
				var err error
				calldata, err = s.chainAPrecompile.Pack(ics02.VerifyNonMembershipMethod,
					existingClientID,
					clientProof,
					trustedHeight,
					pathBz,
				)
				s.Require().NoError(err)

				expErr = true
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// == reset test state ==
			s.SetupTest()

			expErr = false
			calldata = nil
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
			s.Require().NoError(err)

			// decode result
			out, err := s.chainAPrecompile.Unpack(ics02.VerifyNonMembershipMethod, resp.Ret)
			s.Require().NoError(err)

			res, ok := out[0].(*big.Int)
			s.Require().True(ok)
			s.Require().Equal(expResult, res)
		})
	}
}

func TestICS02ClientTestSuite(t *testing.T) {
	suite.Run(t, new(ICS02ClientTestSuite))
}
