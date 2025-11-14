package slashing

import (
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
// It expects a single argument: the validator's consensus address in hex format.
// That address comes from the validator's CometBFT ed25519 public key,
// typically found in `$HOME/.evmd/config/priv_validator_key.json`.
func (p *Precompile) GetSigningInfo(
	ctx sdk.Context,
	args GetSigningInfoCall,
) (*GetSigningInfoReturn, error) {
	req, err := ParseSigningInfoArgs(args, p.consCodec)
	if err != nil {
		return nil, err
	}

	res, err := p.slashingKeeper.SigningInfo(ctx, req)
	if err != nil {
		return nil, err
	}

	ret := new(GetSigningInfoReturn)
	if err := ret.FromResponse(res); err != nil {
		return nil, err
	}

	return ret, nil
}

// GetSigningInfos implements the query to get signing info for all validators.
func (p *Precompile) GetSigningInfos(
	ctx sdk.Context,
	args GetSigningInfosCall,
) (*GetSigningInfosReturn, error) {
	req, err := ParseSigningInfosArgs(args)
	if err != nil {
		return nil, err
	}

	res, err := p.slashingKeeper.SigningInfos(ctx, req)
	if err != nil {
		return nil, err
	}

	ret := new(GetSigningInfosReturn)
	if err := ret.FromResponse(res); err != nil {
		return nil, err
	}

	return ret, nil
}

// GetParams implements the query to get the slashing parameters.
func (p *Precompile) GetParams(
	ctx sdk.Context,
	_ GetParamsCall,
) (*GetParamsReturn, error) {
	res, err := p.slashingKeeper.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		return nil, err
	}

	ret := new(GetParamsReturn)
	if err := ret.FromResponse(res); err != nil {
		return nil, err
	}

	return ret, nil
}
