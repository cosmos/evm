package legacypool

import (
	"math/big"
	"sync"
	"testing"

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
	store := NewTxStore()

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
	store := NewTxStore()

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
	store := NewTxStore()

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

func TestTxStoreRemoveTx(t *testing.T) {
	store := NewTxStore()

	addr1 := common.HexToAddress("0x1")
	tx1 := createTestTx(0, big.NewInt(1e9), big.NewInt(2e9))
	tx2 := createTestTx(1, big.NewInt(1e9), big.NewInt(2e9))

	store.AddTx(addr1, tx1)
	store.AddTx(addr1, tx2)
	store.RemoveTx(addr1, tx1)

	result := store.Txs(txpool.PendingFilter{})
	require.Len(t, result[addr1], 1)
	require.Equal(t, uint64(1), result[addr1][0].Tx.Nonce())
}

func TestTxStoreConcurrentRemove(t *testing.T) {
	store := NewTxStore()

	addr1 := common.HexToAddress("0x1")
	var numTxs uint64 = 1000
	var nonce uint64 = 0

	for ; nonce < numTxs; nonce++ {
		store.AddTx(addr1, createTestTx(nonce, big.NewInt(1e9), big.NewInt(2e9)))
	}

	// concurrently remove even-nonce txs
	var wg sync.WaitGroup
	for nonce = 0; nonce < numTxs; nonce += 2 {
		wg.Add(1)
		go func(nonce uint64) {
			defer wg.Done()
			store.RemoveTx(addr1, createTestTx(nonce, big.NewInt(1e9), big.NewInt(2e9)))
		}(nonce)
	}
	wg.Wait()

	result := store.Txs(txpool.PendingFilter{})
	require.Len(t, result[addr1], 500)
}

func TestTxStoreBlobTxsFiltered(t *testing.T) {
	store := NewTxStore()

	addr1 := common.HexToAddress("0x1")
	store.AddTx(addr1, createTestTx(0, big.NewInt(1e9), big.NewInt(2e9)))

	result := store.Txs(txpool.PendingFilter{OnlyBlobTxs: true})
	require.Nil(t, result)
}
