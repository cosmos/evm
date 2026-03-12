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

var _ sdk.Tx = (*keyedMockTx)(nil)
var _ authsigning.SigVerifiableTx = (*keyedMockTx)(nil)

func newKeyedMockTx(t *testing.T, sequence uint64) sdk.Tx {
	t.Helper()

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	pubKeyBytes := crypto.CompressPubkey(&key.PublicKey)
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
