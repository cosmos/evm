package types

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewHolderInfo creates a new HolderInfo instance
func NewHolderInfo(address string, liquidBalance, bondedBalance, unbondingBalance math.Int, rank uint32) HolderInfo {
	totalBalance := liquidBalance.Add(bondedBalance).Add(unbondingBalance)
	return HolderInfo{
		Address:          address,
		LiquidBalance:    liquidBalance,
		BondedBalance:    bondedBalance,
		UnbondingBalance: unbondingBalance,
		TotalBalance:     totalBalance,
		Rank:             rank,
	}
}

// NewHolderInfoWithTag creates a new HolderInfo instance with a module tag
func NewHolderInfoWithTag(address string, liquidBalance, bondedBalance, unbondingBalance math.Int, rank uint32, moduleTag string) HolderInfo {
	holder := NewHolderInfo(address, liquidBalance, bondedBalance, unbondingBalance, rank)
	holder.ModuleTag = moduleTag
	return holder
}

// NewTopHoldersCache creates a new TopHoldersCache instance
func NewTopHoldersCache(holders []HolderInfo, lastUpdated, blockHeight int64) TopHoldersCache {
	return TopHoldersCache{
		Holders:     holders,
		LastUpdated: lastUpdated,
		BlockHeight: blockHeight,
	}
}

// Validate performs basic validation of HolderInfo
func (h HolderInfo) Validate() error {
	if h.Address == "" {
		return ErrInvalidAddress
	}

	if _, err := sdk.AccAddressFromBech32(h.Address); err != nil {
		return ErrInvalidAddress
	}

	if h.LiquidBalance.IsNegative() {
		return ErrInvalidBalance
	}

	if h.BondedBalance.IsNegative() {
		return ErrInvalidBalance
	}

	if h.UnbondingBalance.IsNegative() {
		return ErrInvalidBalance
	}

	expectedTotal := h.LiquidBalance.Add(h.BondedBalance).Add(h.UnbondingBalance)
	if !h.TotalBalance.Equal(expectedTotal) {
		return ErrInvalidBalance
	}

	return nil
}

// Validate performs basic validation of TopHoldersCache
func (t TopHoldersCache) Validate() error {
	if len(t.Holders) > 1000 {
		return ErrTooManyHolders
	}

	for i, holder := range t.Holders {
		if err := holder.Validate(); err != nil {
			return err
		}

		if holder.Rank != uint32(i+1) {
			return ErrInvalidRank
		}

		// Check that holders are sorted by total balance (descending)
		if i > 0 && holder.TotalBalance.GT(t.Holders[i-1].TotalBalance) {
			return ErrInvalidSorting
		}
	}

	return nil
}
