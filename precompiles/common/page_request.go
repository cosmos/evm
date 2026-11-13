package common

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/cosmos/cosmos-sdk/types/query"
)

// PageRequestFromArg converts an ABI pagination tuple into a Cosmos PageRequest.
// Conversion failures are returned as the shared InvalidPageRequest Solidity error.
func PageRequestFromArg(moduleABI abi.ABI, method string, index int, arg interface{}) (pageRequest query.PageRequest, err error) {
	if typed, ok := arg.(query.PageRequest); ok {
		return typed, nil
	}
	if typed, ok := arg.(*query.PageRequest); ok {
		if typed == nil {
			return query.PageRequest{}, invalidPageRequestError(moduleABI, method, index, arg)
		}
		return *typed, nil
	}

	defer func() {
		if recover() != nil {
			pageRequest = query.PageRequest{}
			err = invalidPageRequestError(moduleABI, method, index, arg)
		}
	}()

	converted := abi.ConvertType(arg, new(query.PageRequest))
	typed, ok := converted.(*query.PageRequest)
	if !ok || typed == nil {
		return query.PageRequest{}, invalidPageRequestError(moduleABI, method, index, arg)
	}
	return *typed, nil
}

func invalidPageRequestError(moduleABI abi.ABI, method string, index int, arg interface{}) error {
	return NewRevertWithSolidityError(
		moduleABI,
		SolidityErrInvalidPageRequest,
		method,
		big.NewInt(int64(index)),
		bestEffortString(arg),
	)
}

func bestEffortString(value interface{}) (formatted string) {
	formatted = fmt.Sprintf("%T", value)
	defer func() {
		_ = recover()
	}()
	formatted = fmt.Sprintf("%v", value)
	return formatted
}
