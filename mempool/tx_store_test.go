package mempool

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/gogoproto/proto"

	"cosmossdk.io/log/v2"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
)

// mockTx is a minimal sdk.Tx implementation for testing.
type mockTx struct {
	id int
}

var _ sdk.Tx = (*mockTx)(nil)

func (m *mockTx) GetMsgs() []proto.Message              { return nil }
func (m *mockTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

func newMockTx(id int) sdk.Tx {
	return &mockTx{id: id}
}

type keyedMockTx struct {
	pubKey   cryptotypes.PubKey
	sequence uint64
}

type multiKeyedMockTx struct {
	pubKeys   []cryptotypes.PubKey
	sequences []uint64
}

type feeKeyedMockTx struct {
	pubKey   cryptotypes.PubKey
	sequence uint64
	gas      uint64
	fee      sdk.Coins
}

var (
	_ sdk.Tx                      = (*keyedMockTx)(nil)
	_ authsigning.SigVerifiableTx = (*keyedMockTx)(nil)
	_ sdk.Tx                      = (*multiKeyedMockTx)(nil)
	_ authsigning.SigVerifiableTx = (*multiKeyedMockTx)(nil)
	_ sdk.Tx                      = (*feeKeyedMockTx)(nil)
	_ authsigning.SigVerifiableTx = (*feeKeyedMockTx)(nil)
	_ sdk.FeeTx                   = (*feeKeyedMockTx)(nil)
)

func newKeyedMockTx(t *testing.T, sequence uint64) sdk.Tx {
	t.Helper()

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	pubKeyBytes := crypto.CompressPubkey(&key.PublicKey)
	return newKeyedMockTxWithPubKey(pubKeyBytes, sequence)
}

func newKeyedMockTxWithPubKey(pubKeyBytes []byte, sequence uint64) sdk.Tx {
	return &keyedMockTx{
		pubKey:   &ethsecp256k1.PubKey{Key: pubKeyBytes},
		sequence: sequence,
	}
}

func (m *keyedMockTx) GetMsgs() []proto.Message              { return nil }
func (m *keyedMockTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }
func (m *keyedMockTx) GetSigners() ([][]byte, error) {
	return [][]byte{m.pubKey.Address().Bytes()}, nil
}

func (m *keyedMockTx) GetPubKeys() ([]cryptotypes.PubKey, error) {
	return []cryptotypes.PubKey{m.pubKey}, nil
}

func (m *keyedMockTx) GetSignaturesV2() ([]signingtypes.SignatureV2, error) {
	return []signingtypes.SignatureV2{{
		PubKey:   m.pubKey,
		Sequence: m.sequence,
	}}, nil
}

// unorderedMockTx is a keyedMockTx flagged unordered: its ChooseNonce value is
// the timeout timestamp, not the (zero) sequence.
type unorderedMockTx struct {
	keyedMockTx
	timeout time.Time
}

var _ sdk.TxWithUnordered = (*unorderedMockTx)(nil)

func newUnorderedMockTxWithPubKey(pubKeyBytes []byte, timeout time.Time) sdk.Tx {
	return &unorderedMockTx{
		keyedMockTx: keyedMockTx{pubKey: &ethsecp256k1.PubKey{Key: pubKeyBytes}},
		timeout:     timeout,
	}
}

func (m *unorderedMockTx) GetUnordered() bool             { return true }
func (m *unorderedMockTx) GetTimeoutTimeStamp() time.Time { return m.timeout }

func newMultiKeyedMockTx(pubKeyBytes [][]byte, sequences []uint64) sdk.Tx {
	pubKeys := make([]cryptotypes.PubKey, 0, len(pubKeyBytes))
	for _, pubKey := range pubKeyBytes {
		pubKeys = append(pubKeys, &ethsecp256k1.PubKey{Key: pubKey})
	}

	return &multiKeyedMockTx{
		pubKeys:   pubKeys,
		sequences: sequences,
	}
}

func (m *multiKeyedMockTx) GetMsgs() []proto.Message              { return nil }
func (m *multiKeyedMockTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }
func (m *multiKeyedMockTx) GetSigners() ([][]byte, error) {
	signers := make([][]byte, 0, len(m.pubKeys))
	for _, pubKey := range m.pubKeys {
		signers = append(signers, pubKey.Address().Bytes())
	}
	return signers, nil
}

func (m *multiKeyedMockTx) GetPubKeys() ([]cryptotypes.PubKey, error) {
	return m.pubKeys, nil
}

func (m *multiKeyedMockTx) GetSignaturesV2() ([]signingtypes.SignatureV2, error) {
	sigs := make([]signingtypes.SignatureV2, 0, len(m.pubKeys))
	for i, pubKey := range m.pubKeys {
		sigs = append(sigs, signingtypes.SignatureV2{
			PubKey:   pubKey,
			Sequence: m.sequences[i],
		})
	}
	return sigs, nil
}

const feeKeyedMockTxDenom = "atest"

func newFeeKeyedMockTxWithPubKey(pubKeyBytes []byte, sequence uint64, gasPrice int64) sdk.Tx {
	const gas uint64 = 100_000

	return &feeKeyedMockTx{
		pubKey:   &ethsecp256k1.PubKey{Key: pubKeyBytes},
		sequence: sequence,
		gas:      gas,
		fee:      sdk.NewCoins(sdk.NewInt64Coin(feeKeyedMockTxDenom, gasPrice*int64(gas))),
	}
}

func (m *feeKeyedMockTx) GetMsgs() []proto.Message              { return nil }
func (m *feeKeyedMockTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }
func (m *feeKeyedMockTx) GetSigners() ([][]byte, error) {
	return [][]byte{m.pubKey.Address().Bytes()}, nil
}

func (m *feeKeyedMockTx) GetPubKeys() ([]cryptotypes.PubKey, error) {
	return []cryptotypes.PubKey{m.pubKey}, nil
}

func (m *feeKeyedMockTx) GetSignaturesV2() ([]signingtypes.SignatureV2, error) {
	return []signingtypes.SignatureV2{{
		PubKey:   m.pubKey,
		Sequence: m.sequence,
	}}, nil
}

func (m *feeKeyedMockTx) GetGas() uint64 {
	return m.gas
}

func (m *feeKeyedMockTx) GetFee() sdk.Coins {
	return m.fee
}

func (m *feeKeyedMockTx) FeePayer() []byte {
	return m.pubKey.Address().Bytes()
}

func (m *feeKeyedMockTx) FeeGranter() []byte {
	return nil
}

func TestCosmosTxStoreAddAndGet(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	tx1 := newMockTx(1)
	tx2 := newMockTx(2)
	tx3 := newMockTx(3)

	store.AddTx(tx1)
	store.AddTx(tx2)
	store.AddTx(tx3)

	txs := store.Txs()
	require.Len(t, txs, 3)
}

func TestCosmosTxStoreDedup(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	tx := newKeyedMockTx(t, 1)

	store.AddTx(tx)
	store.AddTx(tx)
	store.AddTx(tx)

	require.Equal(t, 1, store.Len())
}

func TestCosmosTxStoreIterator(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	tx1 := newMockTx(1)
	tx2 := newMockTx(2)
	tx3 := newMockTx(3)

	store.AddTx(tx1)
	store.AddTx(tx2)
	store.AddTx(tx3)

	iter := store.Iterator()
	require.NotNil(t, iter)

	var collected []sdk.Tx
	for ; iter != nil; iter = iter.Next() {
		collected = append(collected, iter.Tx())
	}
	require.Len(t, collected, 3)
}

func TestCosmosTxStoreIteratorEmpty(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())
	require.Nil(t, store.Iterator())
}

func TestCosmosTxStoreIteratorSnapshotIsolation(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	tx1 := newMockTx(1)
	tx2 := newMockTx(2)

	store.AddTx(tx1)
	store.AddTx(tx2)

	iter := store.Iterator()
	require.NotNil(t, iter)

	// mutate the store after creating the iterator
	store.AddTx(newMockTx(3))

	// iterator should still see the original 2 txs
	var count int
	for ; iter != nil; iter = iter.Next() {
		count++
	}
	require.Equal(t, 2, count)
}

func newPubKeyBytes(t *testing.T) []byte {
	t.Helper()
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	return crypto.CompressPubkey(&key.PublicKey)
}

func TestCosmosTxStoreRemoveTx(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	signer := newPubKeyBytes(t)
	tx0 := newKeyedMockTxWithPubKey(signer, 0)
	tx1 := newKeyedMockTxWithPubKey(signer, 1)

	store.AddTx(tx0)
	store.AddTx(tx1)
	require.Equal(t, 2, store.Len())

	require.True(t, store.RemoveTx(tx0))
	require.Equal(t, 1, store.Len())

	// removing again is a no-op
	require.False(t, store.RemoveTx(tx0))
	require.Equal(t, 1, store.Len())

	// the remaining tx is the one we did not remove
	require.True(t, store.RemoveTx(tx1))
	require.Equal(t, 0, store.Len())
}

func TestCosmosTxStoreCloneIsIndependent(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	signer := newPubKeyBytes(t)
	store.AddTx(newKeyedMockTxWithPubKey(signer, 0))
	store.AddTx(newKeyedMockTxWithPubKey(signer, 1))
	store.AddTx(newKeyedMockTxWithPubKey(signer, 2))
	// carry a committed watermark forward too: drops nonce 0, leaving 1 and 2
	store.PruneCommitted(newKeyedMockTxWithPubKey(signer, 0))
	require.Equal(t, 2, store.Len())

	clone := store.Clone()
	require.Equal(t, store.Len(), clone.Len())

	// mutating the clone must not affect the source
	clone.AddTx(newKeyedMockTxWithPubKey(signer, 3))
	require.Equal(t, 2, store.Len())
	require.Equal(t, 3, clone.Len())

	// mutating the source must not affect the clone
	require.True(t, store.RemoveTx(newKeyedMockTxWithPubKey(signer, 1)))
	require.Equal(t, 1, store.Len())
	require.Equal(t, 3, clone.Len())

	// the committed watermark is carried: the clone still rejects the consumed nonce
	clone.AddTx(newKeyedMockTxWithPubKey(signer, 0))
	require.Equal(t, 3, clone.Len())
}

// A watermark blocks re-adds for its own generation plus one aging, then is
// retired: two completed recheck passes have covered the commit by then.
func TestCosmosTxStoreAgeWatermarks(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	signer := newPubKeyBytes(t)
	store.AddTx(newKeyedMockTxWithPubKey(signer, 0))
	require.Equal(t, 1, store.PruneCommitted(newKeyedMockTxWithPubKey(signer, 0)))

	store.AddTx(newKeyedMockTxWithPubKey(signer, 0))
	require.Equal(t, 0, store.Len())

	// first aging keeps the mark one more generation
	store.AgeWatermarks()
	store.AddTx(newKeyedMockTxWithPubKey(signer, 0))
	require.Equal(t, 0, store.Len())

	// second aging retires it; a stale (e.g. never-committed) mark heals here
	store.AgeWatermarks()
	store.AddTx(newKeyedMockTxWithPubKey(signer, 0))
	require.Equal(t, 1, store.Len())
}

func TestCosmosTxStorePruneCommitted(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	signer := newPubKeyBytes(t)
	store.AddTx(newKeyedMockTxWithPubKey(signer, 0))
	store.AddTx(newKeyedMockTxWithPubKey(signer, 1))
	store.AddTx(newKeyedMockTxWithPubKey(signer, 2))
	require.Equal(t, 3, store.Len())

	// committing nonce 0 drops nonce 0, keeps 1 and 2
	require.Equal(t, 1, store.PruneCommitted(newKeyedMockTxWithPubKey(signer, 0)))
	require.Equal(t, 2, store.Len())

	// a re-add of the committed nonce is rejected by the watermark
	store.AddTx(newKeyedMockTxWithPubKey(signer, 0))
	require.Equal(t, 2, store.Len())

	// committing nonce 1 drops nonce 1, keeps 2
	require.Equal(t, 1, store.PruneCommitted(newKeyedMockTxWithPubKey(signer, 1)))
	require.Equal(t, 1, store.Len())
	store.AddTx(newKeyedMockTxWithPubKey(signer, 1))
	require.Equal(t, 1, store.Len())
}

func TestCosmosTxStorePruneCommittedMultiSignerOnClone(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	signerA := newPubKeyBytes(t)
	signerB := newPubKeyBytes(t)
	store.AddTx(newMultiKeyedMockTx([][]byte{signerA, signerB}, []uint64{0, 0}))

	clone := store.Clone()
	require.Equal(t, 1, clone.PruneCommitted(newKeyedMockTxWithPubKey(signerA, 0)))
	require.Equal(t, 0, clone.Len())
	require.Equal(t, 1, store.Len(), "pruning the clone must not touch the source")
}

// Unkeyed txs must not be carried across heights: they get a fresh key on
// every AddTx and cannot be removed, so a carried copy duplicates every pass.
func TestCosmosTxStoreCloneDropsUnkeyed(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	store.AddTx(newMockTx(1)) // no signers: stored under the unkeyed bucket
	store.AddTx(newKeyedMockTxWithPubKey(newPubKeyBytes(t), 0))
	require.Equal(t, 2, store.Len())

	clone := store.Clone()
	require.Equal(t, 1, clone.Len(), "clone must not carry the unkeyed bucket")

	// the re-add a recheck pass would perform yields exactly one copy again
	clone.AddTx(newMockTx(1))
	require.Equal(t, 2, clone.Len())
}

// A replaced multi-signer tx lives in a bucket InvalidateFrom(newTx) cannot
// see; InvalidateReplaced drops it (and its dependents) by its own identity.
func TestCosmosTxStoreInvalidateReplaced(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	signerA := newPubKeyBytes(t)
	signerB := newPubKeyBytes(t)
	oldTx := newMultiKeyedMockTx([][]byte{signerA, signerB}, []uint64{5, 0})
	dependent := newMultiKeyedMockTx([][]byte{signerA, signerB}, []uint64{6, 1})
	newTx := newKeyedMockTxWithPubKey(signerA, 5)

	store.AddTx(oldTx)
	store.AddTx(dependent)
	require.Equal(t, 2, store.Len())

	// same signer set is a no-op: InvalidateFrom owns that case
	require.Equal(t, 0, store.InvalidateReplaced(oldTx, oldTx))
	require.Equal(t, 2, store.Len())

	// different signer set drops the old tx and anything atop its nonces
	require.Equal(t, 2, store.InvalidateReplaced(oldTx, newTx))
	require.Equal(t, 0, store.Len())
}

// Committing an unordered tx must not watermark the signer — that would
// blacklist their ordered txs. Only the exact tx is dropped.
func TestCosmosTxStorePruneCommittedUnordered(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	signer := newPubKeyBytes(t)
	timeout := time.Unix(1_700_000_000, 0)
	unordered := newUnorderedMockTxWithPubKey(signer, timeout)
	earlier := newUnorderedMockTxWithPubKey(signer, timeout.Add(-time.Second))
	ordered := newKeyedMockTxWithPubKey(signer, 5)

	store.AddTx(unordered)
	store.AddTx(earlier)
	store.AddTx(ordered)
	require.Equal(t, 3, store.Len())

	// only the committed unordered tx is dropped, not the signer's other txs
	require.Equal(t, 1, store.PruneCommitted(unordered))
	require.Equal(t, 2, store.Len())

	// no watermark was recorded: the signer's ordered txs stay addable
	store.AddTx(newKeyedMockTxWithPubKey(signer, 6))
	require.Equal(t, 3, store.Len())
}

// A committed single-signer tx must evict a pooled multi-signer tx that shares
// that signer/nonce — the exact case the deferred-removal comment warns about.
func TestCosmosTxStorePruneCommittedMultiSigner(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	signerA := newPubKeyBytes(t)
	signerB := newPubKeyBytes(t)

	// a tx signed by both A@0 and B@0
	multi := newMultiKeyedMockTx([][]byte{signerA, signerB}, []uint64{0, 0})
	store.AddTx(multi)
	require.Equal(t, 1, store.Len())

	// committing A@0 (single signer) must drop the multi-signer tx
	require.Equal(t, 1, store.PruneCommitted(newKeyedMockTxWithPubKey(signerA, 0)))
	require.Equal(t, 0, store.Len())

	// and it stays out even if a recheck tries to re-add it
	store.AddTx(multi)
	require.Equal(t, 0, store.Len())
}

// AddTx overwrites the tx occupying a signer/nonce slot rather than dropping the
// update, so a carried-forward store reflects the latest tx for that slot.
func TestCosmosTxStoreAddOverwritesSlot(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	signer := newPubKeyBytes(t)
	store.AddTx(newFeeKeyedMockTxWithPubKey(signer, 0, 1))
	store.AddTx(newFeeKeyedMockTxWithPubKey(signer, 0, 5)) // same slot, higher fee

	require.Equal(t, 1, store.Len())
	txs := store.Txs()
	require.Len(t, txs, 1)
	feeTx, ok := txs[0].(sdk.FeeTx)
	require.True(t, ok)
	require.Equal(t, sdk.NewInt64Coin(feeKeyedMockTxDenom, 5*100_000), feeTx.GetFee()[0])
}

func TestCosmosTxStoreOrdersBucketByNonceSum(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	pubKeyBytes := crypto.CompressPubkey(&key.PublicKey)
	tx3 := newKeyedMockTxWithPubKey(pubKeyBytes, 3)
	tx1 := newKeyedMockTxWithPubKey(pubKeyBytes, 1)
	tx2 := newKeyedMockTxWithPubKey(pubKeyBytes, 2)

	store.AddTx(tx3)
	store.AddTx(tx1)
	store.AddTx(tx2)

	require.Equal(t, []sdk.Tx{tx1, tx2, tx3}, store.Txs())
}

func TestCosmosTxStoreOrderedIteratorByPriceAndNonce(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	keyA, err := crypto.GenerateKey()
	require.NoError(t, err)
	keyB, err := crypto.GenerateKey()
	require.NoError(t, err)

	txA0 := newFeeKeyedMockTxWithPubKey(crypto.CompressPubkey(&keyA.PublicKey), 0, 1)
	txA1 := newFeeKeyedMockTxWithPubKey(crypto.CompressPubkey(&keyA.PublicKey), 1, 100)
	txB0 := newFeeKeyedMockTxWithPubKey(crypto.CompressPubkey(&keyB.PublicKey), 0, 5)

	store.AddTx(txA0)
	store.AddTx(txA1)
	store.AddTx(txB0)

	iter := store.OrderedIterator(feeKeyedMockTxDenom, nil)
	var txs []sdk.Tx
	for ; iter != nil; iter = iter.Next() {
		txs = append(txs, iter.Tx())
	}
	require.Equal(t, []sdk.Tx{txB0, txA0, txA1}, txs)
}

func TestCosmosTxStoreInvalidateFromUsesStoredNonceMap(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	pubKeyBytes := crypto.CompressPubkey(&key.PublicKey)
	tx1 := newKeyedMockTxWithPubKey(pubKeyBytes, 1)
	tx2 := newKeyedMockTxWithPubKey(pubKeyBytes, 2)
	tx3 := newKeyedMockTxWithPubKey(pubKeyBytes, 3)

	store.AddTx(tx1)
	store.AddTx(tx2)
	store.AddTx(tx3)

	removed := store.InvalidateFrom(tx2)
	require.Equal(t, 2, removed)
	require.Equal(t, []sdk.Tx{tx1}, store.Txs())
}

func TestCosmosTxStoreInvalidateFromFreshTxNoOp(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	pubKeyBytes := crypto.CompressPubkey(&key.PublicKey)
	tx1 := newKeyedMockTxWithPubKey(pubKeyBytes, 1)
	tx2 := newKeyedMockTxWithPubKey(pubKeyBytes, 2)

	store.AddTx(tx1)

	removed := store.InvalidateFrom(tx2)
	require.Zero(t, removed)
	require.Equal(t, []sdk.Tx{tx1}, store.Txs())
}

func TestCosmosTxStoreInvalidateFromCrossesSignerBuckets(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	bobKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	aliceKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	bobPubKey := crypto.CompressPubkey(&bobKey.PublicKey)
	alicePubKey := crypto.CompressPubkey(&aliceKey.PublicKey)

	bobTx4 := newKeyedMockTxWithPubKey(bobPubKey, 4)
	bobTx5 := newKeyedMockTxWithPubKey(bobPubKey, 5)
	multiTx7 := newMultiKeyedMockTx([][]byte{alicePubKey, bobPubKey}, []uint64{7, 7})
	multiTx8 := newMultiKeyedMockTx([][]byte{alicePubKey, bobPubKey}, []uint64{8, 8})

	store.AddTx(bobTx4)
	store.AddTx(bobTx5)
	store.AddTx(multiTx7)
	store.AddTx(multiTx8)

	removed := store.InvalidateFrom(bobTx5)
	require.Equal(t, 3, removed)

	txs := store.Txs()
	require.Equal(t, []sdk.Tx{bobTx4}, txs)
}

func TestCosmosTxStoreInvalidateFromMultiSignerEvictsSingleSigner(t *testing.T) {
	store := NewCosmosTxStore(log.NewNopLogger())

	aliceKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	bobKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	carolKey, err := crypto.GenerateKey()
	require.NoError(t, err)
	eveKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	alicePubKey := crypto.CompressPubkey(&aliceKey.PublicKey)
	bobPubKey := crypto.CompressPubkey(&bobKey.PublicKey)
	carolPubKey := crypto.CompressPubkey(&carolKey.PublicKey)
	evePubKey := crypto.CompressPubkey(&eveKey.PublicKey)

	aliceTx5 := newKeyedMockTxWithPubKey(alicePubKey, 5)
	bobTx3 := newKeyedMockTxWithPubKey(bobPubKey, 3)     // below B-threshold; survives
	bobTx5 := newKeyedMockTxWithPubKey(bobPubKey, 5)     // at B-threshold; evicted
	carolTx7 := newKeyedMockTxWithPubKey(carolPubKey, 7) // above C-threshold; evicted
	eveTx9 := newKeyedMockTxWithPubKey(evePubKey, 9)     // unrelated signer; survives

	multiTx := newMultiKeyedMockTx(
		[][]byte{alicePubKey, bobPubKey, carolPubKey},
		[]uint64{5, 5, 5},
	)

	store.AddTx(aliceTx5)
	store.AddTx(bobTx3)
	store.AddTx(bobTx5)
	store.AddTx(carolTx7)
	store.AddTx(eveTx9)
	store.AddTx(multiTx)

	removed := store.InvalidateFrom(multiTx)
	// evicted: aliceTx5 (A:5>=5), bobTx5 (B:5>=5), carolTx7 (C:7>=5), multiTx itself.
	require.Equal(t, 4, removed)

	require.ElementsMatch(t, []sdk.Tx{bobTx3, eveTx9}, store.Txs())
}
