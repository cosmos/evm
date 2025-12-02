package keeper

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/cosmos/evm/x/topholders/types"
)

var _ types.QueryServer = (*Keeper)(nil)

// TopHolders implements the Query/TopHolders gRPC method
func (k *Keeper) TopHolders(ctx context.Context, req *types.QueryTopHoldersRequest) (*types.QueryTopHoldersResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if err := req.Validate(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	cache, found := k.GetTopHoldersCache(ctx)
	if !found {
		return nil, status.Error(codes.NotFound, "top holders cache not found")
	}

	// Calculate pagination bounds
	offset := uint64(0)
	limit := uint64(100) // default limit

	if req.Pagination != nil {
		if req.Pagination.Offset > 0 {
			offset = req.Pagination.Offset
		}
		if req.Pagination.Limit > 0 && req.Pagination.Limit <= 1000 {
			limit = req.Pagination.Limit
		}
	}

	// Apply pagination
	start := offset
	end := offset + limit

	if start >= uint64(len(cache.Holders)) {
		return types.NewQueryTopHoldersResponse(
			[]types.HolderInfo{},
			&query.PageResponse{
				NextKey: nil,
				Total:   uint64(len(cache.Holders)),
			},
			cache.LastUpdated,
			cache.BlockHeight,
			uint32(len(cache.Holders)),
		), nil
	}

	if end > uint64(len(cache.Holders)) {
		end = uint64(len(cache.Holders))
	}

	holders := cache.Holders[start:end]

	// Create pagination response
	var nextKey []byte
	if end < uint64(len(cache.Holders)) {
		nextKey = []byte{1}
	}

	pageResponse := &query.PageResponse{
		NextKey: nextKey,
		Total:   uint64(len(cache.Holders)),
	}

	return types.NewQueryTopHoldersResponse(
		holders,
		pageResponse,
		cache.LastUpdated,
		cache.BlockHeight,
		uint32(len(cache.Holders)),
	), nil
}

// CacheStatus implements the Query/CacheStatus gRPC method
func (k *Keeper) CacheStatus(ctx context.Context, req *types.QueryCacheStatusRequest) (*types.QueryCacheStatusResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	cache, found := k.GetTopHoldersCache(ctx)
	if !found {
		return types.NewQueryCacheStatusResponse(0, 0, 0, k.IsUpdating()), nil
	}

	return types.NewQueryCacheStatusResponse(
		cache.LastUpdated,
		cache.BlockHeight,
		uint32(len(cache.Holders)),
		k.IsUpdating(),
	), nil
}
