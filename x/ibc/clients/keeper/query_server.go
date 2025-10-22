package keeper

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/common"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/types/query"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cosmos/evm/x/ibc/clients/types"
)

var _ types.QueryServer = (*Keeper)(nil)

// ClientPrecompile defines the handler for the Query/ClientPrecompile RPC method.
func (k Keeper) ClientPrecompile(ctx context.Context, req *types.QueryClientPrecompileRequest) (*types.QueryClientPrecompileResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}

	if common.IsHexAddress(req.Client) {
		addressBz := common.HexToAddress(req.Client).Bytes()
		precompile, err := k.AddressPrecompilesMap.Get(ctx, addressBz)
		if err != nil {
			return nil, err
		}
		return &types.QueryClientPrecompileResponse{Precompile: &precompile}, nil
	} else {
		precompile, err := k.ClientPrecompilesMap.Get(ctx, req.Client)
		if err != nil {
			return nil, errorsmod.Wrapf(err, "precompile for client ID %s not found", req.Client)
		}
		return &types.QueryClientPrecompileResponse{Precompile: &precompile}, nil
	}

}

// ClientPrecompiles defines the handler for the Query/ClientPrecompiles RPC method.
func (k Keeper) ClientPrecompiles(ctx context.Context, req *types.QueryClientPrecompilesRequest) (*types.QueryClientPrecompilesResponse, error) {
	precompiles, pageRes, err := query.CollectionPaginate(
		ctx,
		k.ClientPrecompilesMap,
		req.Pagination,
		func(key string, value types.ClientPrecompile) (*types.ClientPrecompile, error) {
			return &value, nil
		})
	if err != nil {
		return nil, err
	}

	return &types.QueryClientPrecompilesResponse{Precompiles: precompiles, Pagination: pageRes}, nil
}

// Params defines the handler for the Query/Params RPC method.
func (k Keeper) Params(ctx context.Context, req *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	params, err := k.ParamsItem.Get(ctx)
	if err != nil {
		if errors.Is(err, collections.ErrNotFound) {
			return &types.QueryParamsResponse{Params: types.Params{}}, nil
		}

		return nil, status.Error(codes.Internal, err.Error())
	}

	return &types.QueryParamsResponse{Params: params}, nil
}
