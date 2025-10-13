package ics20

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// DenomMethod defines the ABI method name for the ICS20 Denom
	// query.
	DenomMethod = "denom"
	// DenomsMethod defines the ABI method name for the ICS20 Denoms
	// query.
	DenomsMethod = "denoms"
	// DenomHashMethod defines the ABI method name for the ICS20 DenomHash
	// query.
	DenomHashMethod = "denomHash"
)

// Denom returns the requested denomination information.
func (p Precompile) Denom(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewDenomRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.transferKeeper.Denom(ctx, req)
	if err != nil {
		// if the trace does not exist, return empty array
		if strings.Contains(err.Error(), ErrDenomNotFound) {
			return method.Outputs.Pack(transfertypes.Denom{})
		}
		return nil, err
	}

	return method.Outputs.Pack(*res.Denom)
}

// Denoms returns the requested denomination information.
func (p Precompile) Denoms(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewDenomsRequest(method, args)
	if err != nil {
		return nil, err
	}

	res, err := p.transferKeeper.Denoms(ctx, req)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(res.Denoms, res.Pagination)
}

// DenomHash returns the denom hash (in hex format) of the denomination information.
func (p Precompile) DenomHash(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewDenomHashRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.transferKeeper.DenomHash(ctx, req)
	if err != nil {
		// if the denom hash does not exist, return empty string
		if strings.Contains(err.Error(), ErrDenomNotFound) {
			return method.Outputs.Pack("")
		}
		return nil, err
	}

	return method.Outputs.Pack(res.Hash)
}
