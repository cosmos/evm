package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
)

// ClientKeeper expected account IBC client keeper
type ClientKeeper interface {
	Route(ctx sdk.Context, clientID string) (ibcexported.LightClientModule, error)
}
