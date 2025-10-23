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
