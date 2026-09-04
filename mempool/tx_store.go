package mempool

import (
	"maps"
	"reflect"
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

	// signerBuckets indexes signer -> keys of the buckets containing it, so
	// shared-signer scans (InvalidateFrom, InvalidateReplaced) touch only
	// matching buckets. Membership changes when a bucket is created or emptied.
	signerBuckets map[string]map[string]struct{}

	// height is the chain height the snapshot is being (re)built for (see
	// SetHeight). AddTx stamps entries with it; carried entries keep the stamp
	// of the pass that validated them (see ValidatedAt).
	height uint64

	// byTx indexes stored pointer-typed txs to their validatedAt stamp, so
	// ValidatedAt and AddTx re-adds skip signer extraction and key building.
	// A same-nonce replacement is a different object, so it never vouches for
	// the tx it replaced. Non-pointer txs are not indexed (interface map keys
	// must be comparable) and take the slow paths.
	byTx map[sdk.Tx]uint64

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
		signerBuckets:   make(map[string]map[string]struct{}),
		byTx:            make(map[sdk.Tx]uint64),
		logger:          l,
		signerExtractor: sdkmempool.NewDefaultSignerExtractionAdapter(),
	}
}

// Clone returns a deep-enough copy of store for carrying the validated set forward
// into next height. The tx values are shared (immutable), but the
// bucket/index maps are copied so mutations on clone do not affect source.
func (s *CosmosTxStore) Clone() *CosmosTxStore {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clone := &CosmosTxStore{
		txs:             make(map[string]cosmosTxBucket, len(s.txs)),
		signerBuckets:   make(map[string]map[string]struct{}, len(s.signerBuckets)),
		byTx:            maps.Clone(s.byTx),
		nextUnkeyed:     s.nextUnkeyed,
		height:          s.height,
		logger:          s.logger,
		signerExtractor: s.signerExtractor,
	}
	for signer, bucketKeys := range s.signerBuckets {
		clone.signerBuckets[signer] = maps.Clone(bucketKeys)
	}
	for signerKey, bucket := range s.txs {
		// Unkeyed txs are unremovable and get a fresh key on every AddTx, so a
		// carried copy would duplicate once per pass; let each pass re-add them.
		if signerKey == unkeyedSignerKey {
			continue
		}
		clone.txs[signerKey] = cosmosTxBucket{
			txs:     slices.Clone(bucket.txs),
			signers: maps.Clone(bucket.signers),
		}
	}
	return clone
}

// SetHeight records the chain height the snapshot is being (re)built for.
// Each pass calls it once, after the rechecker context moves to that height's
// state, so AddTx stamps entries with the height their validation ran against.
func (s *CosmosTxStore) SetHeight(height uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.height = height
}

// ValidatedAt returns the height the stored copy of tx was last validated at.
// The entry must be the same tx object — a same-nonce replacement does not
// vouch for the tx it replaced — so replaced or absent txs report false.
func (s *CosmosTxStore) ValidatedAt(tx sdk.Tx) (uint64, bool) {
	if !isPointerTx(tx) {
		return 0, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	height, ok := s.byTx[tx]
	return height, ok
}

// isPointerTx reports whether tx's dynamic type is a pointer. Only pointer
// txs enter byTx (the mempool pipeline shares one decoded object per tx, and
// interface map keys must be comparable); anything else takes the slow paths.
func isPointerTx(tx sdk.Tx) bool {
	return reflect.ValueOf(tx).Kind() == reflect.Pointer
}

// AddTx adds a single tx to the store while constructing a validated snapshot.
func (s *CosmosTxStore) AddTx(tx sdk.Tx) {
	s.mu.Lock()
	defer s.mu.Unlock()

	indexable := isPointerTx(tx)

	// fast path: re-adding a known tx only refreshes its stamp — its bucket
	// entry is already correct
	if indexable {
		if _, ok := s.byTx[tx]; ok {
			s.byTx[tx] = s.height
			return
		}
	}

	storedTx := s.newCosmosTxWithMetadata(tx)

	if storedTx.signerKey == "" {
		storedTx.signerKey = unkeyedSignerKey
	}
	if storedTx.txKey == "" {
		storedTx.txKey = s.newUnkeyedStoreKey()
	}

	// unkeyed txs are unremovable, so they are never indexed
	if storedTx.signerKey == unkeyedSignerKey {
		indexable = false
	}

	// bucket.txs is sorted by (nonceSum, txKey): overwrite an occupied slot —
	// each recheck pass re-adds still-valid txs and the newest wins.
	bucket := s.txs[storedTx.signerKey]
	i, found := slices.BinarySearchFunc(bucket.txs, storedTx, compareCosmosTxWithMetadata)
	if found {
		// a replacement occupies the replaced tx's slot; its stamp must not
		// vouch for the tx it replaced
		if old := bucket.txs[i].tx; isPointerTx(old) {
			delete(s.byTx, old)
		}
		bucket.txs[i] = storedTx
		if indexable {
			s.byTx[tx] = s.height
		}
		return
	}

	if bucket.signers == nil {
		bucket.signers = signerSetFromNonceMap(storedTx.nonceMap)
		for signer := range bucket.signers {
			if s.signerBuckets[signer] == nil {
				s.signerBuckets[signer] = make(map[string]struct{})
			}
			s.signerBuckets[signer][storedTx.signerKey] = struct{}{}
		}
	}
	bucket.txs = slices.Insert(bucket.txs, i, storedTx)
	s.txs[storedTx.signerKey] = bucket
	if indexable {
		s.byTx[tx] = s.height
	}
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

	return s.filterSignerBucketsLocked(storedTx.nonceMap, func(t cosmosTxWithMetadata) bool {
		return invalidatesCosmosTx(t, storedTx.nonceMap)
	})
}

// InvalidateReplaced removes a replaced tx (and txs validated on top of its
// nonces) when its signer set differs from the replacement's, which
// InvalidateFrom(newTx) cannot see in newTx's own bucket. Same-set
// replacements stay with InvalidateFrom. Returns the number of txs removed.
func (s *CosmosTxStore) InvalidateReplaced(oldTx, newTx sdk.Tx) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldStored := s.newCosmosTxWithMetadata(oldTx)
	newStored := s.newCosmosTxWithMetadata(newTx)
	if oldStored.signerKey == "" || oldStored.txKey == "" || oldStored.signerKey == newStored.signerKey {
		return 0
	}

	return s.filterSignerBucketsLocked(oldStored.nonceMap, func(t cosmosTxWithMetadata) bool {
		return invalidatesCosmosTx(t, oldStored.nonceMap)
	})
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

	removed := s.filterBucketLocked(storedTx.signerKey, s.txs[storedTx.signerKey], func(t cosmosTxWithMetadata) bool {
		return t.txKey == storedTx.txKey
	})
	return removed > 0
}

// filterBucketLocked removes every tx in the bucket at signerKey for which
// match returns true (dropping it from the byTx index too), deleting the
// bucket if it empties. Callers must hold s.mu. Returns the number of txs
// removed.
func (s *CosmosTxStore) filterBucketLocked(signerKey string, bucket cosmosTxBucket, match func(cosmosTxWithMetadata) bool) int {
	next := slices.DeleteFunc(bucket.txs, func(t cosmosTxWithMetadata) bool {
		if !match(t) {
			return false
		}
		if isPointerTx(t.tx) {
			delete(s.byTx, t.tx)
		}
		return true
	})
	removed := len(bucket.txs) - len(next)
	if removed == 0 {
		return 0
	}

	if len(next) == 0 {
		delete(s.txs, signerKey)
		for signer := range bucket.signers {
			delete(s.signerBuckets[signer], signerKey)
			if len(s.signerBuckets[signer]) == 0 {
				delete(s.signerBuckets, signer)
			}
		}
		return removed
	}
	bucket.txs = next
	s.txs[signerKey] = bucket
	return removed
}

// filterSignerBucketsLocked removes every tx matching match from the buckets sharing a signer
// with nonceMap, via signer index. Callers must hold s.mu. Returns the number of txs removed.
func (s *CosmosTxStore) filterSignerBucketsLocked(nonceMap map[string]uint64, match func(cosmosTxWithMetadata) bool) int {
	removed := 0
	for signer := range nonceMap {
		// repeat visits of a multi-signer bucket match nothing
		for signerKey := range s.signerBuckets[signer] {
			removed += s.filterBucketLocked(signerKey, s.txs[signerKey], match)
		}
	}
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

// txKeyZeroPad left-pads nonces to the width of MaxUint64 so the string
// ordering of keys matches numeric nonce ordering.
const txKeyZeroPad = "00000000000000000000"

func cosmosTxKey(nonceMap map[string]uint64) string {
	var b strings.Builder
	for i, k := range sortedSignerKeys(nonceMap) {
		if i > 0 {
			b.WriteByte('|')
		}
		// equivalent to fmt.Fprintf(&b, "%s/%020d", ...) without fmt's
		// reflection; this runs for every tx added on every recheck pass
		nonce := strconv.FormatUint(nonceMap[k], 10)
		b.WriteString(k)
		b.WriteByte('/')
		b.WriteString(txKeyZeroPad[:len(txKeyZeroPad)-len(nonce)])
		b.WriteString(nonce)
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
