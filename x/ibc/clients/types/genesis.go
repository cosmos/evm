package types

import (
	"github.com/ethereum/go-ethereum/common"

	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params: DefaultParams(),
	}
}

// Validate performs basic genesis state validation returning an error upon any
// failure.
func (gs GenesisState) Validate() error {
	for _, precompile := range gs.ClientPrecompiles {
		if !common.IsHexAddress(precompile.Address) {
			return errortypes.ErrInvalidAddress.Wrapf("address '%s' is not a valid ethereum hex address", precompile.Address)
		}

		if !clienttypes.IsValidClientID(precompile.ClientId) {
			return clienttypes.ErrInvalidClient.Wrapf("client ID '%s' is invalid", precompile.ClientId)
		}
	}

	return gs.Params.Validate()
}
