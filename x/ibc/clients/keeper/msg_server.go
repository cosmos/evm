package keeper

import (
	"context"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/x/ibc/clients/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ types.MsgServer = (*Keeper)(nil)

// IncrementCounter defines the handler for the MsgIncrementCounter message.
func (k Keeper) RegisterClientPrecompile(goCtx context.Context, msg *types.MsgRegisterClientPrecompile) (*types.MsgRegisterClientPrecompileResponse, error) {
	if _, err := k.addressCodec.StringToBytes(msg.Sender); err != nil {
		return nil, errortypes.ErrInvalidAddress.Wrapf("invalid sender address: %v", err)
	}

	if !strings.EqualFold(msg.Sender, k.authority.String()) {
		return nil, errortypes.ErrUnauthorized.Wrapf("unauthorized, authority does not match the module's authority: got %s, want %s", msg.Sender, k.authority.String())
	}

	ctx := sdk.UnwrapSDKContext(goCtx)

	// Verify that the client ID is valid
	if _, found := k.clientKeeper.GetClientState(ctx, msg.ClientId); !found {
		return nil, types.ErrClientNotFound.Wrapf("client ID %s not found", msg.ClientId)
	}

	// address is validated in ValidateBasic
	address := common.HexToAddress(msg.Address)
	if _, err := k.createNewPrecompile(ctx, msg.ClientId, address); err != nil {
		return nil, err
	}

	return &types.MsgRegisterClientPrecompileResponse{}, nil
}

// UpdateParams params is defining the handler for the MsgUpdateParams message.
func (k Keeper) UpdateParams(ctx context.Context, msg *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if _, err := k.addressCodec.StringToBytes(msg.Authority); err != nil {
		return nil, errortypes.ErrInvalidAddress.Wrapf("invalid authority address: %v", err)
	}

	if !strings.EqualFold(msg.Authority, k.authority.String()) {
		return nil, errortypes.ErrUnauthorized.Wrapf("unauthorized, authority does not match the module's authority: got %s, want %s", msg.Authority, k.authority.String())
	}

	if err := k.ParamsItem.Set(ctx, msg.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}
