//go:build !test

package legacypool

import (
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

// reset retrieves the current state of the blockchain and ensures the content
// of the transaction pool is valid with regard to the chain state.
func (pool *LegacyPool) reset(oldHead, newHead *types.Header) {
	// If we're reorging an old state, reinject all dropped transactions
	var reinject types.Transactions

	if oldHead != nil && oldHead.Hash() != newHead.ParentHash {
		// this is a strange reorg check from geth, it is possible for cosmos
		// chains to call this function with newHead=oldHead+2, so
		// newHead.ParentHash != oldHead.Hash. This would incorrectly be seen
		// as a reorg on a cosmos chain and would therefore panic. Since this
		// logic would only panic for cosmos chains in a valid state, we have
		// removed it and replaced with a debug log.
		//
		// see https://github.com/cosmos/evm/pull/668 for more context.
		log.Debug("leacypool saw skipped block (reorg) on cosmos chain, doing nothing...", "oldHead", oldHead.Hash(), "newHead", newHead.Hash(), "newParent", newHead.ParentHash)
	}
	pool.resetInternalState(newHead, reinject)
}
