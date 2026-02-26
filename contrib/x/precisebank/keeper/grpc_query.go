package keeper

import (
	"context"

	"github.com/cosmos/evm/x/precisebank/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type queryServer struct {
	keeper Keeper
}

// NewQueryServerImpl creates a new server for handling gRPC queries.
func NewQueryServerImpl(k Keeper) types.QueryServer {
	return &queryServer{keeper: k}
}

var _ types.QueryServer = queryServer{}

// Remainder returns the remainder amount in x/precisebank.
func (s queryServer) Remainder(
	goCtx context.Context,
	req *types.QueryRemainderRequest,
) (*types.QueryRemainderResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	remainder := s.keeper.GetRemainderAmount(ctx)
	remainderCoin := sdk.NewCoin(types.ExtendedCoinDenom(), remainder)

	return &types.QueryRemainderResponse{
		Remainder: remainderCoin,
	}, nil
}

// FractionalBalance returns the fractional balance of an account.
func (s queryServer) FractionalBalance(
	goCtx context.Context,
	req *types.QueryFractionalBalanceRequest,
) (*types.QueryFractionalBalanceResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	address, err := sdk.AccAddressFromBech32(req.Address)
	if err != nil {
		return nil, err
	}

	amt := s.keeper.GetFractionalBalance(ctx, address)
	fractionalBalance := sdk.NewCoin(types.ExtendedCoinDenom(), amt)

	return &types.QueryFractionalBalanceResponse{
		FractionalBalance: fractionalBalance,
	}, nil
}
