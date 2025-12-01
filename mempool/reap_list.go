package mempool

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// txNode represents a node in the doubly linked list for transactions ready to
// be reaped.
type txNode struct {
	tx   *ethtypes.Transaction
	hash common.Hash
	prev *txNode
	next *txNode
}

type reapList struct {
	// reapListHead is the head of the doubly linked list (oldest tx)
	reapListHead *txNode
	// reapListTail is the tail of the doubly linked list (newest tx)
	reapListTail *txNode
	// txNodePool is a pool for txNode reuse to minimize allocations
	txNodePool sync.Pool
}

func newReapList() *reapList {
	return &reapList{
		txNodePool: sync.Pool{
			New: func() interface{} {
				return &txNode{}
			},
		},
	}
}

// Reap returns a list of the oldest to newest transactions bytes from the reap
// list until either maxBytes or maxGas is reached for the list of transactions
// being returned. If maxBytes and maxGas are both 0 all txs will be returned.
//
// If encoding fails for a tx, it is removed from the reap list and is not
// returned.
func (rl *reapList) Reap(maxBytes uint64, maxGas uint64, encode func(tx *ethtypes.Transaction) ([]byte, error)) [][]byte {
	var (
		totalBytes uint64
		totalGas   uint64
		result     [][]byte
	)

	// Iterate from head (oldest) to tail (newest)
	current := rl.reapListHead
	for current != nil {
		txBytes, err := encode(current.tx)
		if err != nil {
			// Skip this tx and continue
			next := current.next
			rl.removeNode(current)
			current = next
			continue
		}

		txSize := uint64(len(txBytes))
		txGas := current.tx.Gas()

		// Check if adding this tx would exceed limits
		if (maxBytes > 0 && totalBytes+txSize > maxBytes) || (maxGas > 0 && totalGas+txGas > maxGas) {
			break
		}

		// Add to result
		result = append(result, txBytes)
		totalBytes += txSize
		totalGas += txGas

		// Remove from list and return to pool
		next := current.next
		rl.removeNode(current)
		current = next
	}

	return result
}

func (rl *reapList) removeNode(node *txNode) {
	// Remove from linked list
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		// This was the head
		rl.reapListHead = node.next
	}

	if node.next != nil {
		node.next.prev = node.prev
	} else {
		// This was the tail
		rl.reapListTail = node.prev
	}

	// Clear node and return to pool
	node.tx = nil
	node.hash = common.Hash{}
	node.prev = nil
	node.next = nil
	rl.txNodePool.Put(node)
}

// Insert inserts a tx to the back of the reap list as the "newest" transaction
// (last to be returned if Reap was called now).
func (rl *reapList) Insert(tx *ethtypes.Transaction) {
	hash := tx.Hash()

	// Get a node from the pool
	node := rl.txNodePool.Get().(*txNode)
	node.tx = tx
	node.hash = hash
	node.prev = nil
	node.next = nil

	// Add to tail of list
	if rl.reapListTail == nil {
		// List is empty
		rl.reapListHead = node
		rl.reapListTail = node
	} else {
		// Append to tail
		node.prev = rl.reapListTail
		rl.reapListTail.next = node
		rl.reapListTail = node
	}
}
