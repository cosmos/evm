package ics02

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
	// cmn "github.com/cosmos/evm/precompiles/common"
)

const (
	// GetClientStateMethod defines the get client state query method name.
	GetClientStateMethod = "getClientState"
)

// GetSigningInfo handles the `getSigningInfo` precompile call.
// It expects a single argument: the validator’s consensus address in hex format.
// That address comes from the validator’s CometBFT ed25519 public key,
// typically found in `$HOME/.evmd/config/priv_validator_key.json`.
func (p *Precompile) GetClientState(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	req, err := ParseGetClientStateArgs(args, p.clientPrecompile.ClientId)
	if err != nil {
		return nil, err
	}

	_, err = p.clientKeeper.ClientState(ctx.Context(), req)
	if err != nil {
		return nil, err
	}

	panic("not implemented")
}
