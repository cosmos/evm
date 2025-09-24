package keeper

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/evm/x/topholders/types"
)

// Global cache shared across all keeper instances for memory-only mode
var (
	globalCache      *types.TopHoldersCache
	globalCacheMutex sync.RWMutex
	globalIsUpdating bool
	globalLastUpdate time.Time
)

// Keeper of the topholders store
type Keeper struct {
	cdc           codec.BinaryCodec
	storeKey      storetypes.StoreKey
	memKey        storetypes.StoreKey
	authority     string

	bankKeeper    types.BankKeeper
	stakingKeeper types.StakingKeeper
	accountKeeper types.AccountKeeper

	// Cache management
	cacheMutex    sync.RWMutex
	isUpdating    bool
	lastUpdate    time.Time
	inMemoryCache *types.TopHoldersCache

	logger log.Logger
}

// NewKeeper creates a new topholders Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey,
	memKey storetypes.StoreKey,
	authority string,
	bankKeeper types.BankKeeper,
	stakingKeeper types.StakingKeeper,
	accountKeeper types.AccountKeeper,
	logger log.Logger,
) Keeper {
	return Keeper{
		cdc:           cdc,
		storeKey:      storeKey,
		memKey:        memKey,
		authority:     authority,
		bankKeeper:    bankKeeper,
		stakingKeeper: stakingKeeper,
		accountKeeper: accountKeeper,
		logger:        logger.With("module", "x/"+types.ModuleName),
		lastUpdate:    time.Now().Add(-11 * time.Minute),
	}
}

// GetAuthority returns the module's authority.
func (k *Keeper) GetAuthority() string {
	return k.authority
}

// Logger returns a module-specific logger.
func (k *Keeper) Logger() log.Logger {
	return k.logger
}

// GetTopHoldersCache retrieves the cached top holders data
func (k *Keeper) GetTopHoldersCache(ctx context.Context) (types.TopHoldersCache, bool) {
	// If in memory-only mode, use global cache
	if k.storeKey == nil {
		globalCacheMutex.RLock()
		defer globalCacheMutex.RUnlock()

		if globalCache != nil {
			return *globalCache, true
		}

		return types.TopHoldersCache{}, false
	}

	// Otherwise use instance cache and try to load from store
	k.cacheMutex.RLock()
	defer k.cacheMutex.RUnlock()

	// First try in-memory cache
	if k.inMemoryCache != nil {
		return *k.inMemoryCache, true
	}

	// Try persistent store as fallback
	defer func() {
		if r := recover(); r != nil {
			k.logger.Error("topholders store not available", "error", r)
		}
	}()

	store := k.getStore(ctx)
	bz := store.Get(types.TopHoldersKey)
	if bz == nil {
		return types.TopHoldersCache{}, false
	}

	var cache types.TopHoldersCache
	if err := json.Unmarshal(bz, &cache); err != nil {
		k.logger.Error("failed to unmarshal top holders cache", "error", err)
		return types.TopHoldersCache{}, false
	}

	return cache, true
}

// SetTopHoldersCache stores the top holders cache
func (k *Keeper) SetTopHoldersCache(ctx context.Context, cache types.TopHoldersCache) error {
	// If in memory-only mode, use global cache
	if k.storeKey == nil {
		globalCacheMutex.Lock()
		defer globalCacheMutex.Unlock()
		globalCache = &cache
		return nil
	}

	// Otherwise use instance cache and persist to disk
	k.cacheMutex.Lock()
	defer k.cacheMutex.Unlock()

	// Store in memory
	k.inMemoryCache = &cache

	// Persist to disk
	defer func() {
		if r := recover(); r != nil {
			k.logger.Warn("failed to persist cache to store", "error", r)
		}
	}()

	store := k.getStore(ctx)
	bz, err := json.Marshal(cache)
	if err != nil {
		k.logger.Error("failed to marshal cache", "error", err)
		return nil
	}

	store.Set(types.TopHoldersKey, bz)
	return nil
}

// IsUpdating returns whether a cache update is currently in progress
func (k *Keeper) IsUpdating() bool {
	// If in memory-only mode, use global state
	if k.storeKey == nil {
		globalCacheMutex.RLock()
		defer globalCacheMutex.RUnlock()
		return globalIsUpdating
	}

	// Otherwise use instance state
	k.cacheMutex.RLock()
	defer k.cacheMutex.RUnlock()
	return k.isUpdating
}

// SetUpdating sets the updating status
func (k *Keeper) SetUpdating(updating bool) {
	// If in memory-only mode, use global state
	if k.storeKey == nil {
		globalCacheMutex.Lock()
		defer globalCacheMutex.Unlock()
		globalIsUpdating = updating
		if !updating {
			globalLastUpdate = time.Now()
		}
		return
	}

	// Otherwise use instance state
	k.cacheMutex.Lock()
	defer k.cacheMutex.Unlock()
	k.isUpdating = updating
	if !updating {
		k.lastUpdate = time.Now()
	}
}

// GetLastUpdateTime returns the last update time
func (k *Keeper) GetLastUpdateTime() time.Time {
	// If in memory-only mode, use global state
	if k.storeKey == nil {
		globalCacheMutex.RLock()
		defer globalCacheMutex.RUnlock()
		return globalLastUpdate
	}

	// Otherwise use instance state
	k.cacheMutex.RLock()
	defer k.cacheMutex.RUnlock()
	return k.lastUpdate
}

// getStore returns the KVStore for the module
func (k *Keeper) getStore(ctx context.Context) storetypes.KVStore {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return sdkCtx.KVStore(k.storeKey)
}

// ShouldUpdateCache determines if the cache should be updated based on time elapsed
func (k *Keeper) ShouldUpdateCache() bool {
	if k.IsUpdating() {
		return false
	}

	lastUpdate := k.GetLastUpdateTime()
	return time.Since(lastUpdate) > 10*time.Minute
}

// BeginBlocker is called at the beginning of every block
func (k *Keeper) BeginBlocker(ctx context.Context) error {
	if err := k.UpdateCache(ctx); err != nil {
		k.logger.Error("failed to update top holders cache", "error", err)
	}
	return nil
}
