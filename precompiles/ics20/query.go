package ics20

import (
	"strings"

	cmn "github.com/cosmos/evm/precompiles/common"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Denom returns the requested denomination information.
func (p Precompile) Denom(
	ctx sdk.Context,
	args DenomCall,
) (*DenomReturn, error) {
	req, err := NewDenomRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.transferKeeper.Denom(ctx, req)
	if err != nil {
		// if the trace does not exist, return empty array
		if strings.Contains(err.Error(), ErrDenomNotFound) {
			return &DenomReturn{Denom: Denom{}}, nil
		}
		return nil, err
	}

	denom := ConvertDenomToABI(*res.Denom)
	return &DenomReturn{Denom: denom}, nil
}

// Denoms returns the requested denomination information.
func (p Precompile) Denoms(
	ctx sdk.Context,
	args DenomsCall,
) (*DenomsReturn, error) {
	req, err := NewDenomsRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.transferKeeper.Denoms(ctx, req)
	if err != nil {
		return nil, err
	}

	denoms := make([]Denom, len(res.Denoms))
	for i, d := range res.Denoms {
		denoms[i] = ConvertDenomToABI(d)
	}

	return &DenomsReturn{
		Denoms:       denoms,
		PageResponse: cmn.FromPageResponse(res.Pagination),
	}, nil
}

// DenomHash returns the denom hash (in hex format) of the denomination information.
func (p Precompile) DenomHash(
	ctx sdk.Context,
	args DenomHashCall,
) (*DenomHashReturn, error) {
	req, err := NewDenomHashRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.transferKeeper.DenomHash(ctx, req)
	if err != nil {
		// if the denom hash does not exist, return empty string
		if strings.Contains(err.Error(), ErrDenomNotFound) {
			return &DenomHashReturn{Hash: ""}, nil
		}
		return nil, err
	}

	return &DenomHashReturn{Hash: res.Hash}, nil
}

// ConvertDenomToABI converts a transfertypes.Denom to the ABI Denom type
func ConvertDenomToABI(d transfertypes.Denom) Denom {
	hops := make([]Hop, len(d.GetTrace()))
	for i, h := range d.GetTrace() {
		hops[i] = Hop{
			PortId:    h.PortId,
			ChannelId: h.ChannelId,
		}
	}

	return Denom{
		Base:  d.GetBase(),
		Trace: hops,
	}
}
