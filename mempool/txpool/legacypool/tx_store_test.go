package legacypool

import (
	"math/big"
	"testing"

	"cosmossdk.io/log/v2"
	"github.com/cosmos/evm/mempool/txpool"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func createTestTx(nonce uint64, gasTipCap *big.Int, gasFeeCap *big.Int) *types.Transaction {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)

	return types.NewTx(&types.DynamicFeeTx{
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       21000,
		To:        &addr,
		Value:     big.NewInt(100),
	})
}

func TestTxStoreAddAndGet(t *testing.T) {
	store := NewTxStore(log.NewNopLogger())

	addr1 := common.HexToAddress("0x1")
	addr2 := common.HexToAddress("0x2")

	tx1 := createTestTx(0, big.NewInt(1e9), big.NewInt(2e9))
	tx2 := createTestTx(1, big.NewInt(1e9), big.NewInt(2e9))
	tx3 := createTestTx(0, big.NewInt(1e9), big.NewInt(2e9))

	store.AddTx(addr1, tx1)
	store.AddTx(addr1, tx2)
	store.AddTx(addr2, tx3)

	result := store.Txs(txpool.PendingFilter{})
	require.Len(t, result[addr1], 2)
	require.Len(t, result[addr2], 1)
}

func TestTxStoreMinTipFilter(t *testing.T) {
	store := NewTxStore(log.NewNopLogger())

	addr1 := common.HexToAddress("0x1")

	// nonce 0: 2 gwei tip, nonce 1: 0.1 gwei tip
	txHighTip := createTestTx(0, big.NewInt(2e9), big.NewInt(3e9))
	txLowTip := createTestTx(1, big.NewInt(1e8), big.NewInt(2e9))

	store.AddTx(addr1, txHighTip)
	store.AddTx(addr1, txLowTip)

	filter := txpool.PendingFilter{
		MinTip:  uint256.MustFromBig(big.NewInt(1e9)),
		BaseFee: uint256.MustFromBig(big.NewInt(1e9)),
	}
	result := store.Txs(filter)

	// should only get the high tip tx (nonce 0), low tip at nonce 1 is
	// filtered
	require.Len(t, result[addr1], 1)
	require.Equal(t, uint64(0), result[addr1][0].Tx.Nonce())
}

func TestTxStoreSortedByNonce(t *testing.T) {
	store := NewTxStore(log.NewNopLogger())

	addr1 := common.HexToAddress("0x1")

	// add in reverse nonce order
	store.AddTx(addr1, createTestTx(2, big.NewInt(1e9), big.NewInt(2e9)))
	store.AddTx(addr1, createTestTx(0, big.NewInt(1e9), big.NewInt(2e9)))
	store.AddTx(addr1, createTestTx(1, big.NewInt(1e9), big.NewInt(2e9)))

	result := store.Txs(txpool.PendingFilter{})
	require.Len(t, result[addr1], 3)

	for i, lazy := range result[addr1] {
		require.Equal(t, uint64(i), lazy.Tx.Nonce())
	}
}

// TestTxStoreRetainsPreviousTxs tests that if you remove a middle nonce, the earlier nonce txs stay retained.
func TestTxStoreRetainsPreviousTxs(t *testing.T) {
	store := NewTxStore(log.NewNopLogger())

	addr1 := common.HexToAddress("0x1")

	tx1 := createTestTx(0, big.NewInt(1e9), big.NewInt(2e9))
	tx2 := createTestTx(1, big.NewInt(1e9), big.NewInt(2e9))
	tx3 := createTestTx(2, big.NewInt(1e9), big.NewInt(2e9))
	tx4 := createTestTx(3, big.NewInt(1e9), big.NewInt(2e9))
	tx5 := createTestTx(4, big.NewInt(1e9), big.NewInt(2e9))
	txs := []*types.Transaction{tx1, tx2, tx3, tx4, tx5}
	for _, tx := range txs {
		store.AddTx(addr1, tx)
	}

	store.RemoveTx(addr1, tx4)

	result := store.Txs(txpool.PendingFilter{})
	require.Len(t, result[addr1], 3) // should just have 0,1,2.
	for i, tx := range result[addr1] {
		require.Equal(t, uint64(i), tx.Tx.Nonce())
	}
}

func TestTxStoreRemoveTx(t *testing.T) {
	store := NewTxStore(log.NewNopLogger())

	addr1 := common.HexToAddress("0x1")
	tx1 := createTestTx(0, big.NewInt(1e9), big.NewInt(2e9))
	tx2 := createTestTx(1, big.NewInt(1e9), big.NewInt(2e9))

	store.AddTx(addr1, tx1)
	store.AddTx(addr1, tx2)
	store.RemoveTx(addr1, tx1)

	result := store.Txs(txpool.PendingFilter{})
	require.Len(t, result[addr1], 0)
}

func TestTxStoreBlobTxsFiltered(t *testing.T) {
	store := NewTxStore(log.NewNopLogger())

	addr1 := common.HexToAddress("0x1")
	store.AddTx(addr1, createTestTx(0, big.NewInt(1e9), big.NewInt(2e9)))

	result := store.Txs(txpool.PendingFilter{OnlyBlobTxs: true})
	require.Nil(t, result)
}
