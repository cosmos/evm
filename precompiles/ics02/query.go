package ics02

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// GetClientStateMethod defines the get client state query method name.
	GetClientStateMethod = "getClientState"
)

// GetClientState returns the client state for the precompile's client ID.
func (p *Precompile) GetClientState(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	clientID := p.clientPrecompile.ClientId
	req, err := ParseGetClientStateArgs(args, clientID)
	if err != nil {
		return nil, err
	}

	res, err := p.clientKeeper.ClientState(ctx.Context(), req)
	if err != nil {
		return nil, err
	}
	if res.ClientState == nil || len(res.ClientState.Value) == 0 {
		return nil, fmt.Errorf("client state not found for client ID %s", clientID)
	}

	return method.Outputs.Pack(res.ClientState.Value)
}
