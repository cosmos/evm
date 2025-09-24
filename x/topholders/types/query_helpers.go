package types

import (
	"github.com/cosmos/cosmos-sdk/types/query"
)

// NewQueryTopHoldersResponse creates a new QueryTopHoldersResponse
func NewQueryTopHoldersResponse(holders []HolderInfo, pagination *query.PageResponse, lastUpdated, blockHeight int64, totalCount uint32) *QueryTopHoldersResponse {
	return &QueryTopHoldersResponse{
		Holders:     holders,
		Pagination:  pagination,
		LastUpdated: lastUpdated,
		BlockHeight: blockHeight,
		TotalCount:  totalCount,
	}
}

// NewQueryCacheStatusResponse creates a new QueryCacheStatusResponse
func NewQueryCacheStatusResponse(lastUpdated, blockHeight int64, totalHolders uint32, isUpdating bool) *QueryCacheStatusResponse {
	return &QueryCacheStatusResponse{
		LastUpdated:  lastUpdated,
		BlockHeight:  blockHeight,
		TotalHolders: totalHolders,
		IsUpdating:   isUpdating,
	}
}

// Validate validates the QueryTopHoldersRequest
func (q *QueryTopHoldersRequest) Validate() error {
	if q.Pagination != nil && q.Pagination.Limit > 1000 {
		return ErrTooManyHolders
	}
	return nil
}
