package ics02

import (
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/gogoproto/proto"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	commitmenttypesv2 "github.com/cosmos/ibc-go/v10/modules/core/23-commitment/types/v2"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	UpdateClientMethod        = "updateClient"
	VerifyMembershipMethod    = "verifyMembership"
	VerifyNonMembershipMethod = "verifyNonMembership"
	MisbehaviourMethod        = "misbehaviour"
)

const (
	UpdateResultSuccess      uint8 = 0
	UpdateResultMisbehaviour uint8 = 1
	UpdateResultNoop         uint8 = 2
)

// UpdateClient updates the IBC client by passing the update message to the IBC client keeper.
func (p *Precompile) UpdateClient(
	ctx sdk.Context,
	_ *vm.Contract,
	_ vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	clientID := p.clientPrecompile.ClientId
	updateMsg, err := ParseUpdateClientArgs(args)
	if err != nil {
		return nil, err
	}

	var anyUpdateMsg codectypes.Any
	if err := proto.Unmarshal(updateMsg, &anyUpdateMsg); err != nil {
		return nil, err
	}

	clientMsg, err := clienttypes.UnpackClientMessage(&anyUpdateMsg)
	if err != nil {
		return nil, err
	}

	if err := p.clientKeeper.UpdateClient(ctx, clientID, clientMsg); err != nil {
		return nil, err
	}

	if p.clientKeeper.GetClientStatus(ctx, clientID) == ibcexported.Frozen {
		return method.Outputs.Pack(UpdateResultMisbehaviour)
	}

	return method.Outputs.Pack(UpdateResultSuccess)
}

// VerifyMembership verifies a membership proof by passing it to the IBC client keeper.
func (p *Precompile) VerifyMembership(
	ctx sdk.Context,
	_ *vm.Contract,
	_ vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	verifyMsg, err := ParseVerifyMembershipArgs(args)
	if err != nil {
		return nil, err
	}

	clientID := p.clientPrecompile.ClientId
	proofHeight := clienttypes.NewHeight(verifyMsg.ProofHeight.RevisionNumber, verifyMsg.ProofHeight.RevisionHeight)
	path := commitmenttypesv2.NewMerklePath(verifyMsg.Path...)

	if err := p.clientKeeper.VerifyMembership(ctx, clientID, proofHeight, 0, 0, verifyMsg.Proof, path, verifyMsg.Value); err != nil {
		return nil, err
	}

	timestampNano, err := p.clientKeeper.GetClientTimestampAtHeight(ctx, clientID, proofHeight)
	if err != nil {
		return nil, err
	}
	// Convert nanoseconds to seconds without overflow.
	if timestampNano > math.MaxInt64 {
		return nil, fmt.Errorf("timestamp in nanoseconds exceeds int64 max value")
	}
	timestampSeconds := time.Unix(0, int64(timestampNano)).Unix()

	return method.Outputs.Pack(big.NewInt(timestampSeconds))
}

func (p *Precompile) VerifyNonMembership(
	ctx sdk.Context,
	_ *vm.Contract,
	_ vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	verifyMsg, err := ParseVerifyNonMembershipArgs(args)
	if err != nil {
		return nil, err
	}

	clientID := p.clientPrecompile.ClientId
	proofHeight := clienttypes.NewHeight(verifyMsg.ProofHeight.RevisionNumber, verifyMsg.ProofHeight.RevisionHeight)
	path := commitmenttypesv2.NewMerklePath(verifyMsg.Path...)

	if err := p.clientKeeper.VerifyNonMembership(ctx, clientID, proofHeight, 0, 0, verifyMsg.Proof, path); err != nil {
		return nil, err
	}

	timestampNano, err := p.clientKeeper.GetClientTimestampAtHeight(ctx, clientID, proofHeight)
	if err != nil {
		return nil, err
	}
	// Convert nanoseconds to seconds without overflow.
	if timestampNano > math.MaxInt64 {
		return nil, fmt.Errorf("timestamp in nanoseconds exceeds int64 max value")
	}
	timestampSeconds := time.Unix(0, int64(timestampNano)).Unix()

	return method.Outputs.Pack(big.NewInt(timestampSeconds))
}

// Misbehaviour submits a misbehaviour update to the IBC client keeper.
func (p *Precompile) Misbehaviour(
	ctx sdk.Context,
	_ *vm.Contract,
	_ vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	clientID := p.clientPrecompile.ClientId
	updateMsg, err := ParseMisbehaviourArgs(args)
	if err != nil {
		return nil, err
	}

	var anyUpdateMsg codectypes.Any
	if err := proto.Unmarshal(updateMsg, &anyUpdateMsg); err != nil {
		return nil, err
	}

	clientMsg, err := clienttypes.UnpackClientMessage(&anyUpdateMsg)
	if err != nil {
		return nil, err
	}

	if err := p.clientKeeper.UpdateClient(ctx, clientID, clientMsg); err != nil {
		return nil, err
	}

	if p.clientKeeper.GetClientStatus(ctx, clientID) != ibcexported.Frozen {
		return nil, fmt.Errorf("client %s not frozen after misbehaviour update", clientID)
	}

	return method.Outputs.Pack(true)
}
