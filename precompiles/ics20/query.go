package ics20

import (
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/cosmos/evm/x/vm/core/vm"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// DenomTraceMethod defines the ABI method name for the ICS20 DenomTrace
	// query.
	DenomTraceMethod = "denomTrace"
	// DenomTracesMethod defines the ABI method name for the ICS20 DenomTraces
	// query.
	DenomTracesMethod = "denomTraces"
	// DenomHashMethod defines the ABI method name for the ICS20 DenomHash
	// query.
	DenomHashMethod = "denomHash"
)

// DenomTrace returns the requested denomination trace information.
func (p Precompile) DenomTrace(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewDenomTraceRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.transferKeeper.DenomTrace(ctx, req)
	if err != nil {
		// if the trace does not exist, return empty array
		if strings.Contains(err.Error(), ErrTraceNotFound) {
			return method.Outputs.Pack(transfertypes.DenomTrace{})
		}
		return nil, err
	}

	return method.Outputs.Pack(*res.DenomTrace)
}

// DenomTraces returns the requested denomination traces information.
func (p Precompile) DenomTraces(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewDenomTracesRequest(method, args)
	if err != nil {
		return nil, err
	}

	res, err := p.transferKeeper.DenomTraces(ctx, req)
	if err != nil {
		return nil, err
	}

	return method.Outputs.Pack(res.DenomTraces, res.Pagination)
}

// DenomHash returns the denom hash (in hex format) of the denomination trace information.
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
		if strings.Contains(err.Error(), ErrTraceNotFound) {
			return method.Outputs.Pack("")
		}
		return nil, err
	}

	return method.Outputs.Pack(res.Hash)
}

