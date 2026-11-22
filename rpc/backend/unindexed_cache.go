package backend

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/indexer"
	servertypes "github.com/cosmos/evm/server/types"
)

// unindexedCacheHeights is how many heights unindexedCache keeps: the
// heights a lookup by hash scans (maxUnindexedBlocks), with room for the
// head to advance between lookups.
const unindexedCacheHeights = 2 * maxUnindexedBlocks

// unindexedCache keeps the tx results derived from the newest committed
// blocks (indexer.BlockTxResults), by height, for the tx index fallbacks.
// A committed block never changes, so an entry stays valid; the lowest height
// is evicted once more than unindexedCacheHeights are kept. A nil cache keeps
// nothing.
type unindexedCache struct {
	mu      sync.Mutex
	heights map[int64]map[common.Hash]servertypes.TxResult
}

func newUnindexedCache() *unindexedCache {
	return &unindexedCache{heights: make(map[int64]map[common.Hash]servertypes.TxResult)}
}

// get returns the results kept for height. The map must not be modified.
func (c *unindexedCache) get(height int64) (map[common.Hash]servertypes.TxResult, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	r, ok := c.heights[height]
	return r, ok
}

// put keeps results for height and returns them by hash.
func (c *unindexedCache) put(height int64, results []indexer.EthTxResult) map[common.Hash]servertypes.TxResult {
	byHash := make(map[common.Hash]servertypes.TxResult, len(results))
	for _, r := range results {
		byHash[r.Hash] = r.Result
	}
	if c == nil {
		return byHash
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.heights[height] = byHash
	for len(c.heights) > unindexedCacheHeights {
		lowest := height
		for h := range c.heights {
			lowest = min(lowest, h)
		}
		delete(c.heights, lowest)
	}
	return byHash
}
