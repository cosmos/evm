package ics02

import (
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	// "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	commitmenttypesv2 "github.com/cosmos/ibc-go/v10/modules/core/23-commitment/types/v2"
)

const (
	UpdateClientMethod        = "updateClient"
	VerifyMembershipMethod    = "verifyMembership"
	VerifyNonMembershipMethod = "verifyNonMembership"
	MisbehaviourMethod        = "misbehaviour"
)

const (
	UpdateResult_Success      uint8 = 0
	UpdateResult_Misbehaviour uint8 = 1
	UpdateResult_Noop         uint8 = 2
)

// UpdateClient updates the IBC client by passing the update message to the IBC client keeper.
func (p *Precompile) UpdateClient(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
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

	p.clientKeeper.UpdateClient(ctx, p.clientPrecompile.ClientId, clientMsg)
	// TODO: check client state to make sure it is not frozen due to misbehaviour

	return method.Outputs.Pack(UpdateResult_Success)
}

// VerifyMembership verifies a membership proof by passing it to the IBC client keeper.
func (p *Precompile) VerifyMembership(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	verifyMsg, err := ParseVerifyMembershipArgs(args)
	if err != nil {
		return nil, err
	}

	clientId := p.clientPrecompile.ClientId
	proofHeight := clienttypes.NewHeight(verifyMsg.ProofHeight.RevisionNumber, verifyMsg.ProofHeight.RevisionHeight)
	path := commitmenttypesv2.NewMerklePath(verifyMsg.Path...)

	if err := p.clientKeeper.VerifyMembership(ctx, clientId, proofHeight, 0, 0, verifyMsg.Proof, path, verifyMsg.Value); err != nil {
		return nil, err
	}

	timestampNano, err := p.clientKeeper.GetClientTimestampAtHeight(ctx, clientId, proofHeight)
	if err != nil {
		return nil, err
	}
	timestampSeconds := uint64(time.Unix(0, int64(timestampNano)).Unix())

	return method.Outputs.Pack(timestampSeconds)
}

func (p *Precompile) VerifyNonMembership(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	verifyMsg, err := ParseVerifyNonMembershipArgs(args)
	if err != nil {
		return nil, err
	}

	clientId := p.clientPrecompile.ClientId
	proofHeight := clienttypes.NewHeight(verifyMsg.ProofHeight.RevisionNumber, verifyMsg.ProofHeight.RevisionHeight)
	path := commitmenttypesv2.NewMerklePath(verifyMsg.Path...)

	if err := p.clientKeeper.VerifyNonMembership(ctx, clientId, proofHeight, 0, 0, verifyMsg.Proof, path); err != nil {
		return nil, err
	}

	timestampNano, err := p.clientKeeper.GetClientTimestampAtHeight(ctx, clientId, proofHeight)
	if err != nil {
		return nil, err
	}
	timestampSeconds := uint64(time.Unix(0, int64(timestampNano)).Unix())

	return method.Outputs.Pack(timestampSeconds)
}
