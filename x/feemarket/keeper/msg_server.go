package keeper

import (
	"context"

	"github.com/cosmos/evm/x/feemarket/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// UpdateParams implements the gRPC MsgServer interface. When an UpdateParams
// proposal passes, it updates the module parameters. The update can only be
// performed if the requested authority is the Cosmos SDK governance module
// account, unless an authority address is configured via the consensus
// AuthorityParams, in which case that takes precedence.
func (k *Keeper) UpdateParams(goCtx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := sdk.ValidateAuthority(ctx, k.authority.String(), req.Authority); err != nil {
		return nil, err
	}

	if err := k.SetParams(ctx, req.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}
