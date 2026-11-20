package ics02

import (
	cmn "github.com/cosmos/evm/precompiles/common"
	clienttypes "github.com/cosmos/ibc-go/v11/modules/core/02-client/types"
)

const (
	// SolidityErrInvalidClientID is defined in ICS02I.sol.
	SolidityErrInvalidClientID = "InvalidClientID"
	// SolidityErrInvalidProof is defined in ICS02I.sol.
	SolidityErrInvalidProof = "InvalidProof"
	// SolidityErrInvalidPath is defined in ICS02I.sol.
	SolidityErrInvalidPath = "InvalidPath"
	// SolidityErrInvalidValue is defined in ICS02I.sol.
	SolidityErrInvalidValue = "InvalidValue"

	// Registered IBC client custom errors defined in ICS02I.sol.
	SolidityErrIBCClientInvalidClientType = "IBCClientInvalidClientType"
	SolidityErrIBCClientRouteNotFound     = "IBCClientRouteNotFound"
	SolidityErrIBCClientNotActive         = "IBCClientNotActive"
)

// ErrorMappings contains the published IBC client errors reachable from
// the ICS02 keeper boundary. Keys are derived from the registered sentinels.
var ics02ErrorMappings = cmn.CosmosErrorMappings{
	cmn.NewCosmosErrorMapping(clienttypes.ErrInvalidClientType, SolidityErrIBCClientInvalidClientType),
	cmn.NewCosmosErrorMapping(clienttypes.ErrRouteNotFound, SolidityErrIBCClientRouteNotFound),
	cmn.NewCosmosErrorMapping(clienttypes.ErrClientNotActive, SolidityErrIBCClientNotActive),
}

func ErrorMappings() cmn.CosmosErrorMappings {
	return ics02ErrorMappings.Clone()
}
