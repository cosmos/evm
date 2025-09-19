package v2

import (
	"github.com/cosmos/evm/x/ibc/transfer/keeper"
	v2 "github.com/cosmos/ibc-go/v10/modules/apps/transfer/v2"
	ibcapi "github.com/cosmos/ibc-go/v10/modules/core/api"
)

var _ ibcapi.IBCModule = IBCModule{}

// IBCModule implements the ICS26 interface for transfer given the transfer keeper.
type IBCModule struct {
	*v2.IBCModule
}

// NewIBCModule creates a new IBCModule given the keeper
func NewIBCModule(k keeper.Keeper) IBCModule {
	transferModule := v2.NewIBCModule(k.Keeper)
	return IBCModule{
		IBCModule: &transferModule,
	}
}
