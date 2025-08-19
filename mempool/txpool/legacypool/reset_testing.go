//go:build test

package legacypool

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

// reset retrieves the current state of the blockchain and ensures the content
// of the transaction pool is valid with regard to the chain state.
// Testing version - skips reorg logic for Cosmos chains.
func (pool *LegacyPool) reset(oldHead, newHead *types.Header) {
	// If we're reorging an old state, reinject all dropped transactions
	var reinject types.Transactions

	if oldHead != nil && oldHead.Hash() != newHead.ParentHash {
		// Skip reorg logic on Cosmos chains due to instant finality
		// This condition indicates a reorg attempt which shouldn't happen in Cosmos
		log.Debug("Skipping reorg on Cosmos chain (testing mode)", "oldHead", oldHead.Hash(), "newHead", newHead.Hash(), "newParent", newHead.ParentHash)
		reinject = nil // No transactions to reinject
	}

	// Initialize the internal state to the current head
	if newHead == nil {
		newHead = pool.chain.CurrentBlock() // Special case during testing
	}

	// Ensure BaseFee is set for EIP-1559 compatibility in tests
	if newHead.BaseFee == nil && pool.chainconfig.IsLondon(newHead.Number) {
		// Set a default base fee for testing
		newHead.BaseFee = big.NewInt(1000000000) // 1 gwei default
	}

	pool.resetInternalState(newHead, reinject)
}
