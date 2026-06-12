package slashing

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/slashing/types"
)

const (
	// GetSigningInfoMethod defines the ABI method name for the slashing SigningInfo query
	GetSigningInfoMethod = "getSigningInfo"
	// GetSigningInfosMethod defines the ABI method name for the slashing SigningInfos query
	GetSigningInfosMethod = "getSigningInfos"
	// GetParamsMethod defines the ABI method name for the slashing Params query
	GetParamsMethod = "getParams"
)

// GetSigningInfo handles the `getSigningInfo` precompile call.
// It expects a single argument: the validator’s consensus address in hex format.
// That address comes from the validator’s CometBFT ed25519 public key,
// typically found in `$HOME/.evmd/config/priv_validator_key.json`.
func (p *Precompile) GetSigningInfo(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	req, err := ParseSigningInfoArgs(args, p.consCodec)
	if err != nil {
		return nil, err
	}

	res, err := p.slashingKeeper.SigningInfo(ctx, req)
	if err != nil {
		return nil, err
	}

	out, err := new(SigningInfoOutput).FromResponse(res)
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack(out.SigningInfo)
}

// GetSigningInfos implements the query to get signing info for all validators.
func (p *Precompile) GetSigningInfos(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	req, err := ParseSigningInfosArgs(method, args)
	if err != nil {
		return nil, err
	}

	res, err := p.slashingKeeper.SigningInfos(ctx, req)
	if err != nil {
		return nil, err
	}

	out, err := new(SigningInfosOutput).FromResponse(res)
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack(out.SigningInfos, out.PageResponse)
}

// GetParams implements the query to get the slashing parameters.
func (p *Precompile) GetParams(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	_ []interface{},
) ([]byte, error) {
	res, err := p.slashingKeeper.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}

	out := new(ParamsOutput).FromResponse(res)
	return method.Outputs.Pack(out.Params)
}
