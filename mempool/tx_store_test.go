package mempool

import (
	"slices"
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

var (
	_ sdk.Tx                      = (*keyedMockTx)(nil)
	_ authsigning.SigVerifiableTx = (*keyedMockTx)(nil)
	_ sdk.Tx                      = (*multiKeyedMockTx)(nil)
	_ authsigning.SigVerifiableTx = (*multiKeyedMockTx)(nil)
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

func TestCosmosTxStoreInvalidateFromDoesNotCrossSignerBuckets(t *testing.T) {
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
	require.Equal(t, 1, removed)

	txs := store.Txs()
	require.Len(t, txs, 3)
	require.ElementsMatch(t, []sdk.Tx{bobTx4, multiTx7, multiTx8}, txs)
	require.Less(t, slices.Index(txs, multiTx7), slices.Index(txs, multiTx8))
}
