package mempool

import (
	"testing"

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
