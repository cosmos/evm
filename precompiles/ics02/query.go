package ics02

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

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
	_ *vm.Contract,
	_ vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	clientID, err := ParseGetClientStateArgs(args)
	if err != nil {
		return nil, err
	}

	clientState, found := p.clientKeeper.GetClientState(ctx, clientID)
	if !found {
		return nil, fmt.Errorf("client state not found for client ID %s", clientID)
	}

	clientStateAny, err := codectypes.NewAnyWithValue(clientState)
	if err != nil {
		return nil, err
	}
	if len(clientStateAny.Value) == 0 {
		return nil, fmt.Errorf("client state not found for client ID %s", clientID)
	}

	return method.Outputs.Pack(clientStateAny.Value)
}
