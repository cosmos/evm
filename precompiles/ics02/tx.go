package ics02

import (
	"fmt"
	"math"
	"math/big"
	"time"

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
	args *UpdateClientCall,
) (*UpdateClientReturn, error) {
	clientID := args.ClientId
	updateBz := args.UpdateMsg

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
		return &UpdateClientReturn{UpdateResultMisbehaviour}, nil
	}

	return &UpdateClientReturn{UpdateResultSuccess}, nil
}

// VerifyMembership implements the ICS02 VerifyMembership transactions.
func (p *Precompile) VerifyMembership(
	ctx sdk.Context,
	args *VerifyMembershipCall,
) (*VerifyMembershipReturn, error) {
	clientID := args.ClientId
	if host.ClientIdentifierValidator(clientID) != nil {
		return nil, errorsmod.Wrapf(
			clienttypes.ErrInvalidClient,
			"invalid client ID: %s",
			clientID,
		)
	}

	path := commitmenttypesv2.NewMerklePath(args.Path...)

	if err := p.clientKeeper.VerifyMembership(ctx, clientID, args.ProofHeight.ToProofHeight(), 0, 0, args.Proof, path, args.Value); err != nil {
		return nil, err
	}

	timestampNano, err := p.clientKeeper.GetClientTimestampAtHeight(ctx, clientID, args.ProofHeight.ToProofHeight())
	if err != nil {
		return nil, err
	}
	// Convert nanoseconds to seconds without overflow.
	if timestampNano > math.MaxInt64 {
		return nil, fmt.Errorf("timestamp in nanoseconds exceeds int64 max value")
	}
	timestampSeconds := time.Unix(0, int64(timestampNano)).Unix()

	return &VerifyMembershipReturn{big.NewInt(timestampSeconds)}, nil
}

// VerifyNonMembership implements the ICS02 VerifyNonMembership transactions.
func (p *Precompile) VerifyNonMembership(
	ctx sdk.Context,
	args *VerifyNonMembershipCall,
) (*VerifyMembershipReturn, error) {
	clientID := args.ClientId
	if host.ClientIdentifierValidator(clientID) != nil {
		return nil, errorsmod.Wrapf(
			clienttypes.ErrInvalidClient,
			"invalid client ID: %s",
			clientID,
		)
	}
	proofHeight := args.ProofHeight.ToProofHeight()

	path := commitmenttypesv2.NewMerklePath(args.Path...)

	if err := p.clientKeeper.VerifyNonMembership(ctx, clientID, proofHeight, 0, 0, args.Proof, path); err != nil {
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

	return &VerifyMembershipReturn{big.NewInt(timestampSeconds)}, nil
}
