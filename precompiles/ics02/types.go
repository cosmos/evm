package ics02

import (
	"fmt"

	cmn "github.com/cosmos/evm/precompiles/common"

	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
)

// ParseGetClientStateArgs parses the arguments for the GetClientState method.
func ParseGetClientStateArgs(args []interface{}, clientId string) (*clienttypes.QueryClientStateRequest, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 0, len(args))
	}

	return &clienttypes.QueryClientStateRequest{
		ClientId: clientId,
	}, nil
}

// ParseUpdateClientArgs parses the arguments for the UpdateClient method.
func ParseUpdateClientArgs(args []interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	updateBytes, ok := args[0].([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid update client bytes: %v", args[0])
	}
	return updateBytes, nil
}
