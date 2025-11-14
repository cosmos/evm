package ics02

import (
	"fmt"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// GetClientStateMethod defines the get client state query method name.
	GetClientStateMethod = "getClientState"
)

// GetClientState returns the client state for the precompile's client ID.
func (p *Precompile) GetClientState(
	ctx sdk.Context,
	args GetClientStateCall,
) (*GetClientStateReturn, error) {
	clientState, found := p.clientKeeper.GetClientState(ctx, args.ClientId)
	if !found {
		return nil, fmt.Errorf("client state not found for client ID %s", args.ClientId)
	}

	clientStateAny, err := codectypes.NewAnyWithValue(clientState)
	if err != nil {
		return nil, err
	}
	if len(clientStateAny.Value) == 0 {
		return nil, fmt.Errorf("client state not found for client ID %s", args.ClientId)
	}

	return &GetClientStateReturn{
		Field1: clientStateAny.Value,
	}, nil
}
