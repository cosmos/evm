package ics02

import (
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	commitmenttypesv2 "github.com/cosmos/ibc-go/v10/modules/core/23-commitment/types/v2"
	host "github.com/cosmos/ibc-go/v10/modules/core/24-host"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	UpdateClientMethod        = "updateClient"
	VerifyMembershipMethod    = "verifyMembership"
	VerifyNonMembershipMethod = "verifyNonMembership"
)

const (
	UpdateResultSuccess      uint8 = 0
	UpdateResultMisbehaviour uint8 = 1
)

// UpdateClient implements the ICS02 UpdateClient transactions.
func (p *Precompile) UpdateClient(
	ctx sdk.Context,
	_ *vm.Contract,
	_ vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	clientID, updateBz, err := ParseUpdateClientArgs(args)
	if err != nil {
		return nil, err
	}

	if host.ClientIdentifierValidator(clientID) != nil {
		return nil, errorsmod.Wrapf(
			clienttypes.ErrInvalidClient,
			"invalid client ID: %s",
			clientID,
		)
	}

	clientMsg, err := clienttypes.UnmarshalClientMessage(p.cdc, updateBz)
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

// VerifyMembership implements the ICS02 VerifyMembership transactions.
func (p *Precompile) VerifyMembership(
	ctx sdk.Context,
	_ *vm.Contract,
	_ vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	clientID, proof, proofHeight, pathBz, value, err := ParseVerifyMembershipArgs(method, args)
	if err != nil {
		return nil, err
	}

	if host.ClientIdentifierValidator(clientID) != nil {
		return nil, errorsmod.Wrapf(
			clienttypes.ErrInvalidClient,
			"invalid client ID: %s",
			clientID,
		)
	}

	path := commitmenttypesv2.NewMerklePath(pathBz...)

	if err := p.clientKeeper.VerifyMembership(ctx, clientID, proofHeight, 0, 0, proof, path, value); err != nil {
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

// VerifyNonMembership implements the ICS02 VerifyNonMembership transactions.
func (p *Precompile) VerifyNonMembership(
	ctx sdk.Context,
	_ *vm.Contract,
	_ vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	clientID, proof, proofHeight, pathBz, err := ParseVerifyNonMembershipArgs(method, args)
	if err != nil {
		return nil, err
	}

	if host.ClientIdentifierValidator(clientID) != nil {
		return nil, errorsmod.Wrapf(
			clienttypes.ErrInvalidClient,
			"invalid client ID: %s",
			clientID,
		)
	}

	path := commitmenttypesv2.NewMerklePath(pathBz...)

	if err := p.clientKeeper.VerifyNonMembership(ctx, clientID, proofHeight, 0, 0, proof, path); err != nil {
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
