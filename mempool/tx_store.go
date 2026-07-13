package mempool

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

// CosmosTxStore is a set of cosmos transactions that can be added to or
// removed from.
type CosmosTxStore struct {
	txs         map[string]cosmosTxBucket
	nextUnkeyed uint64

	// consumed is a per-signer high-water mark of nonces consumed by committed
	// blocks: AddTx rejects and PruneCommitted drops txs at or below it. It
	// exists because the store is carried across heights (see Clone) — without
	// it, a recheck pass or an Insert racing FinalizeBlock could re-add a
	// just-committed tx and feed it back into a proposal. Holds one entry per
	// signer that has ever committed (grows with active accounts, not traffic).
	consumed map[string]uint64

	logger          log.Logger
	signerExtractor sdkmempool.SignerExtractionAdapter
	mu              sync.RWMutex
}

type cosmosTxBucket struct {
	txs     []cosmosTxWithMetadata
	signers map[string]struct{}
}

type cosmosTxWithMetadata struct {
	tx        sdk.Tx
	nonceMap  map[string]uint64
	nonceSum  uint64
	signerKey string
	txKey     string
}

// NewCosmosTxStore creates a new CosmosTxStore.
func NewCosmosTxStore(l log.Logger) *CosmosTxStore {
	return &CosmosTxStore{
		txs:             make(map[string]cosmosTxBucket),
		consumed:        make(map[string]uint64),
		logger:          l,
		signerExtractor: sdkmempool.NewDefaultSignerExtractionAdapter(),
	}
}

// Clone returns a deep-enough copy of store for carrying the validated set forward
// into next height. The tx values are shared (immutable), but the
// bucket/index/consumed maps are copied so mutations on clone do not affect source.
func (s *CosmosTxStore) Clone() *CosmosTxStore {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clone := &CosmosTxStore{
		txs:             make(map[string]cosmosTxBucket, len(s.txs)),
		consumed:        make(map[string]uint64, len(s.consumed)),
		nextUnkeyed:     s.nextUnkeyed,
		logger:          s.logger,
		signerExtractor: s.signerExtractor,
	}
	for signerKey, bucket := range s.txs {
		txs := make([]cosmosTxWithMetadata, len(bucket.txs))
		copy(txs, bucket.txs)
		signers := make(map[string]struct{}, len(bucket.signers))
		for signer := range bucket.signers {
			signers[signer] = struct{}{}
		}
		clone.txs[signerKey] = cosmosTxBucket{txs: txs, signers: signers}
	}
	for signer, nonce := range s.consumed {
		clone.consumed[signer] = nonce
	}
	return clone
}

// AddTx adds a single tx to the store while constructing a validated snapshot.
func (s *CosmosTxStore) AddTx(tx sdk.Tx) {
	s.mu.Lock()
	defer s.mu.Unlock()

	storedTx := s.newCosmosTxWithMetadata(tx)

	// Reject txs whose nonce a committed block already consumed. This guards the
	// carried-forward store from re-admitting an already-committed tx via a
	// recheck pass or an Insert that races FinalizeBlock.
	if s.isConsumedLocked(storedTx.nonceMap) {
		return
	}

	if storedTx.signerKey == "" {
		storedTx.signerKey = unkeyedSignerKey
	}
	if storedTx.txKey == "" {
		storedTx.txKey = s.newUnkeyedStoreKey()
	}

	bucket := s.txs[storedTx.signerKey]
	for i, existing := range bucket.txs {
		if existing.txKey == storedTx.txKey {
			// The slot is already occupied — expected with a carried-forward
			// store, where each recheck pass re-adds still-valid txs. Overwrite:
			// the pool admits one tx per signer/nonce, so the newest wins.
			bucket.txs[i] = storedTx
			s.txs[storedTx.signerKey] = bucket
			return
		}
	}

	if bucket.signers == nil {
		bucket.signers = signerSetFromNonceMap(storedTx.nonceMap)
	}
	bucket.txs = append(bucket.txs, storedTx)
	slices.SortFunc(bucket.txs, compareCosmosTxWithMetadata)
	s.txs[storedTx.signerKey] = bucket
}

// InvalidateFrom removes any stored tx that depends on the supplied tx's signer/nonces.
// It is used for live mempool replacements: once a tx at nonce N changes, any stored tx
// for the same signer(s) with nonce >= N can no longer be considered valid for proposal building.
func (s *CosmosTxStore) InvalidateFrom(tx sdk.Tx) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	storedTx := s.newCosmosTxWithMetadata(tx)

	// first check if this tx is already here. If it isn't; no need to do anything. It's a fresh insert.
	// If it is, we need to do the work of invaliding any txs from the same sender with a higher nonce.
	// nonce thresholds for each signer.
	if len(storedTx.nonceMap) == 0 || storedTx.signerKey == "" || storedTx.txKey == "" {
		return 0
	}

	bucket, exists := s.txs[storedTx.signerKey]
	if !exists {
		return 0
	}
	if !containsCosmosTx(bucket.txs, storedTx.txKey) {
		return 0
	}

	removed := 0
	for signerKey, existingBucket := range s.txs {
		if !bucketContainsAnySigner(existingBucket, storedTx.nonceMap) {
			continue
		}
		removed += s.filterBucketLocked(signerKey, existingBucket, func(t cosmosTxWithMetadata) bool {
			return invalidatesCosmosTx(t, storedTx.nonceMap)
		})
	}

	return removed
}

// RemoveTx removes a single tx from the store if present. It is the counterpart
// to AddTx used when a recheck pass drops a tx that became invalid: with a
// carried-forward store the tx would otherwise linger from the previous height.
// Returns true if a tx was removed.
func (s *CosmosTxStore) RemoveTx(tx sdk.Tx) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	storedTx := s.newCosmosTxWithMetadata(tx)
	if storedTx.signerKey == "" || storedTx.txKey == "" {
		// unkeyed txs are not addressable for targeted removal
		return false
	}

	bucket, ok := s.txs[storedTx.signerKey]
	if !ok {
		return false
	}
	return s.filterBucketLocked(storedTx.signerKey, bucket, func(t cosmosTxWithMetadata) bool {
		return t.txKey == storedTx.txKey
	}) > 0
}

// PruneCommitted records that a committed block consumed the given tx's
// signer/nonces and drops any stored tx at or below a consumed nonce. It is
// called synchronously as a block is finalized so the carried-forward store can
// never feed an already-committed tx into a later proposal, even before the
// next recheck pass runs. Returns the number of stored txs pruned.
func (s *CosmosTxStore) PruneCommitted(tx sdk.Tx) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	nonceMap, ok := s.cosmosTxNonceMap(tx)
	if !ok {
		return 0
	}

	// bump the per-signer high-water mark
	for signer, nonce := range nonceMap {
		if cur, exists := s.consumed[signer]; !exists || nonce > cur {
			s.consumed[signer] = nonce
		}
	}

	// drop any stored tx now under a watermark: a tx is invalid if ANY of its
	// signers has a consumed nonce, since every signer must be executable
	removed := 0
	for signerKey, bucket := range s.txs {
		removed += s.filterBucketLocked(signerKey, bucket, func(t cosmosTxWithMetadata) bool {
			return s.isConsumedLocked(t.nonceMap)
		})
	}

	return removed
}

// isConsumedLocked reports whether any signer of the given nonceMap sits at or
// below the committed high-water mark. Callers must hold s.mu.
func (s *CosmosTxStore) isConsumedLocked(nonceMap map[string]uint64) bool {
	for signer, nonce := range nonceMap {
		if mark, ok := s.consumed[signer]; ok && nonce <= mark {
			return true
		}
	}
	return false
}

// filterBucketLocked removes every tx in the bucket at signerKey for which
// match returns true, deleting the bucket if it empties. Callers must hold
// s.mu. Returns the number of txs removed.
func (s *CosmosTxStore) filterBucketLocked(signerKey string, bucket cosmosTxBucket, match func(cosmosTxWithMetadata) bool) int {
	next := bucket.txs[:0]
	removed := 0
	for _, existing := range bucket.txs {
		if match(existing) {
			removed++
			continue
		}
		next = append(next, existing)
	}
	if removed == 0 {
		return 0
	}

	clear(bucket.txs[len(next):])
	if len(next) == 0 {
		delete(s.txs, signerKey)
		return removed
	}
	bucket.txs = next
	s.txs[signerKey] = bucket
	return removed
}

func (s *CosmosTxStore) newCosmosTxWithMetadata(tx sdk.Tx) cosmosTxWithMetadata {
	storedTx := cosmosTxWithMetadata{tx: tx}

	nonceMap, ok := s.cosmosTxNonceMap(tx)
	if !ok {
		return storedTx
	}

	storedTx.nonceMap = nonceMap
	storedTx.nonceSum = cosmosTxNonceSum(nonceMap)
	storedTx.signerKey = cosmosTxSignerSetKey(nonceMap)
	storedTx.txKey = cosmosTxKey(nonceMap)
	return storedTx
}

const unkeyedSignerKey = "unkeyed"

func cosmosTxSignerSetKey(nonceMap map[string]uint64) string {
	var b strings.Builder
	for i, k := range sortedSignerKeys(nonceMap) {
		if i > 0 {
			b.WriteByte('|')
		}
		b.WriteString(k)
	}

	return b.String()
}

func cosmosTxKey(nonceMap map[string]uint64) string {
	var b strings.Builder
	for i, k := range sortedSignerKeys(nonceMap) {
		if i > 0 {
			b.WriteByte('|')
		}
		fmt.Fprintf(&b, "%s/%020d", k, nonceMap[k])
	}

	return b.String()
}

func cosmosTxNonceSum(nonceMap map[string]uint64) uint64 {
	var total uint64
	for _, nonce := range nonceMap {
		total += nonce
	}
	return total
}

// cosmosTxNonceMap extracts the signers from the transaction
// and returns a signer -> nonce map.
func (s *CosmosTxStore) cosmosTxNonceMap(tx sdk.Tx) (map[string]uint64, bool) {
	signers, err := s.signerExtractor.GetSigners(tx)
	if err != nil || len(signers) == 0 {
		return nil, false
	}

	nonceMap := make(map[string]uint64, len(signers))
	for _, sig := range signers {
		nonce, err := sdkmempool.ChooseNonce(sig.Sequence, tx)
		if err != nil {
			return nil, false
		}
		nonceMap[string(sig.Signer)] = nonce
	}

	return nonceMap, true
}

func sortedSignerKeys(nonceMap map[string]uint64) []string {
	keys := make([]string, 0, len(nonceMap))
	for k := range nonceMap {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func invalidatesCosmosTx(tx cosmosTxWithMetadata, thresholds map[string]uint64) bool {
	if len(tx.nonceMap) == 0 {
		return false
	}

	for account, threshold := range thresholds {
		nonce, exists := tx.nonceMap[account]
		if exists && nonce >= threshold {
			return true
		}
	}
	return false
}

func signerSetFromNonceMap(nonceMap map[string]uint64) map[string]struct{} {
	signers := make(map[string]struct{}, len(nonceMap))
	for signer := range nonceMap {
		signers[signer] = struct{}{}
	}
	return signers
}

func bucketContainsAnySigner(bucket cosmosTxBucket, thresholds map[string]uint64) bool {
	for signer := range thresholds {
		if _, ok := bucket.signers[signer]; ok {
			return true
		}
	}
	return false
}

func compareCosmosTxWithMetadata(a, b cosmosTxWithMetadata) int {
	if a.nonceSum < b.nonceSum {
		return -1
	}
	if a.nonceSum > b.nonceSum {
		return 1
	}
	return strings.Compare(a.txKey, b.txKey)
}

func containsCosmosTx(bucket []cosmosTxWithMetadata, txKey string) bool {
	for _, tx := range bucket {
		if tx.txKey == txKey {
			return true
		}
	}
	return false
}

func (s *CosmosTxStore) newUnkeyedStoreKey() string {
	storeKey := "unkeyed/" + strconv.FormatUint(s.nextUnkeyed, 10)
	s.nextUnkeyed++
	return storeKey
}

func (s *CosmosTxStore) snapshotTxs() []sdk.Tx {
	signerKeys := make([]string, 0, len(s.txs))
	for signerKey := range s.txs {
		signerKeys = append(signerKeys, signerKey)
	}
	slices.Sort(signerKeys)

	txs := make([]sdk.Tx, 0)
	for _, signerKey := range signerKeys {
		bucket := s.txs[signerKey]
		for _, tx := range bucket.txs {
			txs = append(txs, tx.tx)
		}
	}
	return txs
}

// Txs returns a copy of the current set of txs in the store.
func (s *CosmosTxStore) Txs() []sdk.Tx {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.snapshotTxs()
}

// Iterator returns an sdkmempool.Iterator over the txs in the store.
func (s *CosmosTxStore) Iterator() sdkmempool.Iterator {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.txs) == 0 {
		return nil
	}

	// copy the slice so the iterator is not affected by concurrent mutations
	return &cosmosTxIterator{txs: s.snapshotTxs()}
}

// Len returns the number of txs in the store.
func (s *CosmosTxStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var total int
	for _, bucket := range s.txs {
		total += len(bucket.txs)
	}
	return total
}

// cosmosTxIterator implements sdkmempool.Iterator over a slice of cosmos txs.
type cosmosTxIterator struct {
	txs []sdk.Tx
	pos int
}

func (it *cosmosTxIterator) Tx() sdk.Tx {
	return it.txs[it.pos]
}

func (it *cosmosTxIterator) Next() sdkmempool.Iterator {
	if it.pos+1 >= len(it.txs) {
		return nil
	}
	it.pos++
	return it
}
