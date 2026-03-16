package mempool

import (
	"container/heap"
	"strings"

	"github.com/holiman/uint256"

	cmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

type cosmosTxWithPriority struct {
	tx        cosmosTxWithMetadata
	signerKey string
	fee       *uint256.Int
}

type cosmosTxByPriceAndNonce []*cosmosTxWithPriority

func (h cosmosTxByPriceAndNonce) Len() int { return len(h) }

func (h cosmosTxByPriceAndNonce) Less(i, j int) bool {
	cmp := h[i].fee.Cmp(h[j].fee)
	if cmp == 0 {
		cmp = strings.Compare(h[i].signerKey, h[j].signerKey)
		if cmp == 0 {
			return compareCosmosTxWithMetadata(h[i].tx, h[j].tx) < 0
		}
		return cmp < 0
	}
	return cmp > 0
}

func (h cosmosTxByPriceAndNonce) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *cosmosTxByPriceAndNonce) Push(x any) {
	*h = append(*h, x.(*cosmosTxWithPriority))
}

func (h *cosmosTxByPriceAndNonce) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return item
}

// CosmosTransactionsByPriceAndNonce returns Cosmos transactions in fee-priority
// order while still honoring nonce ordering within each signer bucket.
type CosmosTransactionsByPriceAndNonce struct {
	txs       map[string][]cosmosTxWithMetadata
	heads     cosmosTxByPriceAndNonce
	bondDenom string
	baseFee   *uint256.Int
}

func NewCosmosTransactionsByPriceAndNonce(
	txs map[string][]cosmosTxWithMetadata,
	bondDenom string,
	baseFee *uint256.Int,
) sdkmempool.Iterator {
	heads := make(cosmosTxByPriceAndNonce, 0, len(txs))
	for signerKey, bucket := range txs {
		if len(bucket) == 0 {
			delete(txs, signerKey)
			continue
		}

		heads = append(heads, &cosmosTxWithPriority{
			tx:        bucket[0],
			signerKey: signerKey,
			fee:       extractCosmosEffectiveTip(bucket[0].tx, bondDenom, baseFee),
		})
		txs[signerKey] = bucket[1:]
	}

	if len(heads) == 0 {
		return nil
	}

	heap.Init(&heads)
	return &CosmosTransactionsByPriceAndNonce{
		txs:       txs,
		heads:     heads,
		bondDenom: bondDenom,
		baseFee:   baseFee,
	}
}

func (t *CosmosTransactionsByPriceAndNonce) Tx() sdk.Tx {
	if len(t.heads) == 0 {
		return nil
	}
	return t.heads[0].tx.tx
}

func (t *CosmosTransactionsByPriceAndNonce) Next() sdkmempool.Iterator {
	if len(t.heads) == 0 {
		return nil
	}

	signerKey := t.heads[0].signerKey
	if bucket := t.txs[signerKey]; len(bucket) > 0 {
		t.heads[0] = &cosmosTxWithPriority{
			tx:        bucket[0],
			signerKey: signerKey,
			fee:       extractCosmosEffectiveTip(bucket[0].tx, t.bondDenom, t.baseFee),
		}
		t.txs[signerKey] = bucket[1:]
		heap.Fix(&t.heads, 0)
	} else {
		heap.Pop(&t.heads)
	}

	if len(t.heads) == 0 {
		return nil
	}
	return t
}

func (s *CosmosTxStore) OrderedIterator(bondDenom string, baseFee *uint256.Int) sdkmempool.Iterator {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.txs) == 0 {
		return nil
	}

	snapshot := make(map[string][]cosmosTxWithMetadata, len(s.txs))
	for signerKey, bucket := range s.txs {
		snapshot[signerKey] = append([]cosmosTxWithMetadata(nil), bucket...)
	}

	return NewCosmosTransactionsByPriceAndNonce(snapshot, bondDenom, baseFee)
}

func extractCosmosEffectiveTip(tx sdk.Tx, bondDenom string, baseFee *uint256.Int) *uint256.Int {
	feeTx, ok := tx.(sdk.FeeTx)
	if !ok {
		return uint256.NewInt(0)
	}

	bondDenomFeeAmount := cmath.ZeroInt()
	fees := feeTx.GetFee()
	for _, coin := range fees {
		if coin.Denom == bondDenom {
			bondDenomFeeAmount = coin.Amount
		}
	}

	gas := feeTx.GetGas()
	if gas == 0 {
		return uint256.NewInt(0)
	}

	gasPrice, overflow := uint256.FromBig(bondDenomFeeAmount.Quo(cmath.NewIntFromUint64(gas)).BigInt())
	if overflow {
		return uint256.NewInt(0)
	}

	if baseFee == nil {
		return gasPrice
	}

	if gasPrice.Cmp(baseFee) < 0 {
		return uint256.NewInt(0)
	}

	return new(uint256.Int).Sub(gasPrice, baseFee)
}
