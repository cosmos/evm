package ics02

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	// "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
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
