package ics20

import (
	"strings"

	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Denom returns the requested denomination information.
func (p Precompile) Denom(
	ctx sdk.Context,
	_ *vm.Contract,
	input []byte,
) ([]byte, error) {
	var args DenomCall
	if _, err := args.Decode(input); err != nil {
		return nil, err
	}

	req, err := NewDenomRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.transferKeeper.Denom(ctx, req)
	if err != nil {
		// if the trace does not exist, return empty array
		if strings.Contains(err.Error(), ErrDenomNotFound) {
			return DenomReturn{Denom: Denom{}}.Encode()
		}
		return nil, err
	}

	denom := ConvertDenomToABI(*res.Denom)
	return DenomReturn{Denom: denom}.Encode()
}

// Denoms returns the requested denomination information.
func (p Precompile) Denoms(
	ctx sdk.Context,
	_ *vm.Contract,
	input []byte,
) ([]byte, error) {
	var args DenomsCall
	if _, err := args.Decode(input); err != nil {
		return nil, err
	}

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

	pageResponse := cmn.PageResponse{
		NextKey: res.Pagination.NextKey,
		Total:   res.Pagination.Total,
	}

	return DenomsReturn{
		Denoms:       denoms,
		PageResponse: pageResponse,
	}.Encode()
}

// DenomHash returns the denom hash (in hex format) of the denomination information.
func (p Precompile) DenomHash(
	ctx sdk.Context,
	_ *vm.Contract,
	input []byte,
) ([]byte, error) {
	var args DenomHashCall
	if _, err := args.Decode(input); err != nil {
		return nil, err
	}

	req, err := NewDenomHashRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.transferKeeper.DenomHash(ctx, req)
	if err != nil {
		// if the denom hash does not exist, return empty string
		if strings.Contains(err.Error(), ErrDenomNotFound) {
			return DenomHashReturn{Hash: ""}.Encode()
		}
		return nil, err
	}

	return DenomHashReturn{Hash: res.Hash}.Encode()
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
