package mempool_test

import (
	"crypto/ecdsa"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/cometbft/cometbft/crypto/tmhash"

	"github.com/cosmos/evm/mempool"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// mockEncoder implements the EVMCosmosTxEncoder interface for testing
type mockEncoder struct {
	evmBytesPerTx    uint64
	cosmosBytesPerTx uint64
	failEVMNonces    map[uint64]bool
	failCosmosHashes map[string]bool
}

func (m *mockEncoder) EVMTx(tx *types.Transaction) ([]byte, error) {
	if m.failEVMNonces != nil && m.failEVMNonces[tx.Nonce()] {
		return nil, errors.New("encoding failed")
	}
	if m.evmBytesPerTx > 0 {
		// Include unique tx hash prefix to avoid collisions
		result := make([]byte, m.evmBytesPerTx)
		hash := tx.Hash().Bytes()
		copy(result, hash)
		return result, nil
	}
	// Variable size based on nonce for some tests
	return make([]byte, 100+(tx.Nonce()*10)), nil
}

func (m *mockEncoder) CosmosTx(tx sdk.Tx) ([]byte, error) {
	// Create a deterministic byte representation for testing
	// Use the tx id to ensure uniqueness
	mockTx, ok := tx.(*mockCosmosTx)
	var txBytes []byte
	if ok {
		txBytes = []byte(fmt.Sprintf("cosmos-tx-%d", mockTx.id))
	} else {
		txBytes = []byte(fmt.Sprintf("cosmos-tx-%p", tx))
	}

	hash := fmt.Sprintf("%X", tmhash.Sum(txBytes))

	if m.failCosmosHashes != nil && m.failCosmosHashes[hash] {
		return nil, errors.New("encoding failed")
	}
	if m.cosmosBytesPerTx > 0 {
		// Create bytes with unique prefix to avoid hash collisions
		result := make([]byte, m.cosmosBytesPerTx)
		copy(result, txBytes)
		return result, nil
	}
	return txBytes, nil
}

func newDeterministicEncoder(evmBytesPerTx, cosmosBytesPerTx uint64) *mockEncoder {
	return &mockEncoder{
		evmBytesPerTx:    evmBytesPerTx,
		cosmosBytesPerTx: cosmosBytesPerTx,
	}
}

func newFailingEVMEncoder(failNonces map[uint64]bool) *mockEncoder {
	return &mockEncoder{
		failEVMNonces: failNonces,
	}
}

var _ sdk.FeeTx = &mockCosmosTx{}

// mockCosmosTx implements sdk.Tx and sdk.FeeTx for testing
type mockCosmosTx struct {
	gas      uint64
	id       int // unique identifier for each tx
	msgs     []sdk.Msg
	feePayer sdk.AccAddress
}

func (m *mockCosmosTx) GetMsgs() []sdk.Msg {
	return m.msgs
}

func (m *mockCosmosTx) GetMsgsV2() ([]proto.Message, error) {
	return nil, nil
}

func (m *mockCosmosTx) ValidateBasic() error {
	return nil
}

func (m *mockCosmosTx) GetGas() uint64 {
	return m.gas
}

func (m *mockCosmosTx) GetFee() sdk.Coins {
	return sdk.NewCoins(sdk.NewInt64Coin("stake", 100))
}

func (m *mockCosmosTx) FeePayer() []byte {
	return m.feePayer
}

func (m *mockCosmosTx) FeeGranter() []byte {
	return nil
}

func newMockCosmosTx(id int, gas uint64) *mockCosmosTx {
	return &mockCosmosTx{
		gas: gas,
		id:  id,
	}
}

// Helper function to create a test EVM transaction with specific gas
func testEVMTx(t *testing.T, key *ecdsa.PrivateKey, nonce uint64, gas uint64) *types.Transaction {
	t.Helper()

	tx := types.NewTransaction(
		nonce,
		common.Address{},
		big.NewInt(txValue),
		gas,
		big.NewInt(1),
		nil,
	)
	signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, key)
	require.NoError(t, err)
	return signedTx
}

func TestReapList_EmptyList(t *testing.T) {
	rl := mempool.NewReapList(newDeterministicEncoder(100, 100))

	result := rl.Reap(0, 0)

	require.Empty(t, result, "reaping empty list should return empty result")
}

func TestReapList_SingleTransaction(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(newDeterministicEncoder(100, 100))
	tx := testEVMTx(t, key, 0, 21000)
	err = rl.PushEVMTx(tx)
	require.NoError(t, err)

	result := rl.Reap(0, 0)

	require.Len(t, result, 1, "should reap single transaction")
	require.Len(t, result[0], 100, "transaction should have expected size")

	// Second reap should return empty as tx was removed
	result = rl.Reap(0, 0)
	require.Empty(t, result, "second reap should return empty")

	// Push the same tx again to ensure that it is not able to be reaped again
	err = rl.PushEVMTx(tx)
	require.NoError(t, err)

	result = rl.Reap(0, 0)
	require.Len(t, result, 0, "should reap no transactions")
}

func TestReapList_NoLimits(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(newDeterministicEncoder(100, 100))

	// Add 10 transactions
	for i := uint64(0); i < 10; i++ {
		tx := testEVMTx(t, key, i, 21000)
		err = rl.PushEVMTx(tx)
		require.NoError(t, err)
	}

	result := rl.Reap(0, 0)

	require.Len(t, result, 10, "should reap all transactions with no limits")
}

func TestReapList_MaxBytesLimit(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Each tx is 100 bytes
	rl := mempool.NewReapList(newDeterministicEncoder(100, 100))

	// Add 10 transactions
	for i := uint64(0); i < 10; i++ {
		tx := testEVMTx(t, key, i, 21000)
		err = rl.PushEVMTx(tx)
		require.NoError(t, err)
	}

	// Limit to 350 bytes (should get 3 transactions)
	result := rl.Reap(350, 0)

	require.Len(t, result, 3, "should reap 3 transactions with 350 byte limit")

	// Next reap should get remaining 7
	result = rl.Reap(0, 0)
	require.Len(t, result, 7, "should have 7 transactions remaining")
}

func TestReapList_MaxGasLimit(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(newDeterministicEncoder(100, 100))

	// Add transactions with varying gas
	txGases := []uint64{21000, 30000, 40000, 50000, 60000}
	var nonce uint64
	for _, gas := range txGases {
		tx := testEVMTx(t, key, nonce, gas)
		err = rl.PushEVMTx(tx)
		require.NoError(t, err)
		nonce++
	}

	// Limit to 100000 gas (should get first 3 txs: 21000 + 30000 + 40000 = 91000)
	result := rl.Reap(0, 100000)

	require.Len(t, result, 3, "should reap 3 transactions with 100000 gas limit")

	// Next reap should get remaining 2
	result = rl.Reap(0, 0)
	require.Len(t, result, 2, "should have 2 transactions remaining")
}

func TestReapList_BothLimits(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(newDeterministicEncoder(100, 100))

	// Add transactions with varying gas
	txGases := []uint64{21000, 30000, 40000, 50000, 60000}
	var nonce uint64
	for _, gas := range txGases {
		tx := testEVMTx(t, key, nonce, gas)
		err = rl.PushEVMTx(tx)
		require.NoError(t, err)
		nonce++
	}

	// Limit to 250 bytes (2.5 txs) and 70000 gas (first 3 txs would be 91000)
	// Should be limited by bytes, so only 2 transactions
	result := rl.Reap(250, 70000)

	require.Len(t, result, 2, "should be limited by bytes, returning 2 transactions")

	// Next reap with gas limit should get next 2 txs (40000 + 50000 = 90000 < 100000)
	result = rl.Reap(0, 100000)
	require.Len(t, result, 2, "should reap next 2 transactions within gas limit")

	// Final reap should get last tx
	result = rl.Reap(0, 0)
	require.Len(t, result, 1, "should have 1 transaction remaining")
}

func TestReapList_ExactBytesLimit(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Each tx is 100 bytes
	rl := mempool.NewReapList(newDeterministicEncoder(100, 100))

	// Add 5 transactions
	for i := uint64(0); i < 5; i++ {
		tx := testEVMTx(t, key, i, 21000)
		err = rl.PushEVMTx(tx)
		require.NoError(t, err)
	}

	// Limit to exactly 300 bytes (should get exactly 3 transactions)
	result := rl.Reap(300, 0)

	require.Len(t, result, 3, "should reap exactly 3 transactions with exact byte limit")
}

func TestReapList_ExactGasLimit(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(newDeterministicEncoder(100, 100))

	// Add transactions with specific gas amounts
	txGases := []uint64{21000, 30000, 40000}
	var nonce uint64
	for _, gas := range txGases {
		tx := testEVMTx(t, key, nonce, gas)
		err = rl.PushEVMTx(tx)
		require.NoError(t, err)
		nonce++
	}

	// Limit to exactly 51000 gas (21000 + 30000 = 51000, exactly 2 txs)
	result := rl.Reap(0, 51000)

	require.Len(t, result, 2, "should reap exactly 2 transactions with exact gas limit")
}

func TestReapList_EncodingFailure(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Create encoder that fails for nonce 1 and 3
	failNonces := map[uint64]bool{1: true, 3: true}
	rl := mempool.NewReapList(newFailingEVMEncoder(failNonces))

	// Add 5 transactions (nonces 0-4)
	// Nonces 1 and 3 will fail during Push, so only 0, 2, 4 will be added
	for i := uint64(0); i < 5; i++ {
		tx := testEVMTx(t, key, i, 21000)
		_ = rl.PushEVMTx(tx) // Ignore error for failing encodings
	}

	result := rl.Reap(0, 0)

	// Should get 3 transactions (0, 2, 4) - nonces 1 and 3 fail encoding during Push
	require.Len(t, result, 3, "should have 3 transactions that succeeded encoding")

	// Verify we got the correct transactions by checking sizes
	// Nonce 0: size = 100 + 0*10 = 100
	// Nonce 2: size = 100 + 2*10 = 120
	// Nonce 4: size = 100 + 4*10 = 140
	require.Len(t, result[0], 100, "first tx should be nonce 0")
	require.Len(t, result[1], 120, "second tx should be nonce 2")
	require.Len(t, result[2], 140, "third tx should be nonce 4")
}

// nonceEncoder embeds nonce in bytes for order verification testing
type nonceEncoder struct{}

func (e *nonceEncoder) EVMTx(tx *types.Transaction) ([]byte, error) {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, tx.Nonce())
	return buf, nil
}

func (e *nonceEncoder) CosmosTx(tx sdk.Tx) ([]byte, error) {
	return []byte("cosmos-tx"), nil
}

func TestReapList_OrderPreservation(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Create encoder that embeds nonce in the bytes for verification
	rl := mempool.NewReapList(&nonceEncoder{})

	// Add transactions in specific order
	var nonce uint64
	for nonce = 0; nonce < 5; nonce++ {
		tx := testEVMTx(t, key, nonce, 21000)
		err = rl.PushEVMTx(tx)
		require.NoError(t, err)
	}

	result := rl.Reap(0, 0)

	require.Len(t, result, 5, "should reap all transactions")

	// Verify order is preserved (oldest to newest)
	var expectedNonce uint64
	for expectedNonce = 0; expectedNonce < 5; expectedNonce++ {
		actualNonce := binary.LittleEndian.Uint64(result[expectedNonce])
		require.Equal(t, expectedNonce, actualNonce, "transactions should be in order")
	}
}

func TestReapList_MultipleReaps(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(newDeterministicEncoder(100, 100))

	// Add 10 transactions
	var nonce uint64
	for nonce = 0; nonce < 10; nonce++ {
		tx := testEVMTx(t, key, nonce, 21000)
		err = rl.PushEVMTx(tx)
		require.NoError(t, err)
	}

	// First reap: get 3
	result := rl.Reap(300, 0)
	require.Len(t, result, 3)

	// Second reap: get 2
	result = rl.Reap(200, 0)
	require.Len(t, result, 2)

	// Third reap: get remaining 5
	result = rl.Reap(0, 0)
	require.Len(t, result, 5)

	// Fourth reap: empty
	result = rl.Reap(0, 0)
	require.Empty(t, result)
}

func TestReapList_PushAfterReap(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(newDeterministicEncoder(100, 100))

	// Add 5 transactions
	var nonce uint64
	for nonce = 0; nonce < 5; nonce++ {
		tx := testEVMTx(t, key, nonce, 21000)
		err = rl.PushEVMTx(tx)
		require.NoError(t, err)
	}

	// Reap 3
	result := rl.Reap(300, 0)
	require.Len(t, result, 3)

	// Add 3 more
	for nonce = 5; nonce < 8; nonce++ {
		tx := testEVMTx(t, key, nonce, 21000)
		err = rl.PushEVMTx(tx)
		require.NoError(t, err)
	}

	// Should have 5 total (2 remaining + 3 new)
	result = rl.Reap(0, 0)
	require.Len(t, result, 5)
}

func TestReapList_ConcurrentPushAndReap(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(newDeterministicEncoder(100, 100))

	var wg sync.WaitGroup
	var totalReaped int
	var mu sync.Mutex

	// Pusher goroutine: continuously add transactions
	wg.Add(1)
	go func() {
		defer wg.Done()
		var nonce uint64
		for nonce = 0; nonce < 100; nonce++ {
			tx := testEVMTx(t, key, nonce, 21000)
			_ = rl.PushEVMTx(tx)
		}
	}()

	// Reaper goroutine: continuously reap transactions
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			result := rl.Reap(200, 0) // Reap 2 at a time
			mu.Lock()
			totalReaped += len(result)
			mu.Unlock()
		}
	}()

	wg.Wait()

	// Final reap to get any remaining
	result := rl.Reap(0, 0)
	totalReaped += len(result)

	// We should have reaped close to 100 transactions (may vary due to timing)
	// The exact number depends on race timing, but should be reasonable
	require.GreaterOrEqual(t, totalReaped, 50, "should reap at least half of pushed transactions")
	require.LessOrEqual(t, totalReaped, 100, "should not reap more than pushed")
}

func TestReapList_FirstTransactionExceedsLimit(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(newDeterministicEncoder(1000, 1000))

	// Add transaction
	tx := testEVMTx(t, key, 0, 21000)
	err = rl.PushEVMTx(tx)
	require.NoError(t, err)

	// Try to reap with limit smaller than first tx
	result := rl.Reap(500, 0)

	// Should return empty as first tx exceeds limit
	require.Empty(t, result, "should return empty when first tx exceeds limit")

	// Transaction should still be in list for next reap with higher limit
	result = rl.Reap(1000, 0)
	require.Len(t, result, 1, "transaction should still be available with higher limit")
}

// alwaysFailEncoder always returns an error for encoding
type alwaysFailEncoder struct{}

func (e *alwaysFailEncoder) EVMTx(tx *types.Transaction) ([]byte, error) {
	return nil, errors.New("encoding always fails")
}

func (e *alwaysFailEncoder) CosmosTx(tx sdk.Tx) ([]byte, error) {
	return nil, errors.New("encoding always fails")
}

func TestReapList_AllTransactionsFailEncoding(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	// Encoder that always fails
	rl := mempool.NewReapList(&alwaysFailEncoder{})

	// Add transactions - all will fail during Push
	var nonce uint64
	for nonce = 0; nonce < 5; nonce++ {
		tx := testEVMTx(t, key, nonce, 21000)
		_ = rl.PushEVMTx(tx) // Ignore errors
	}

	result := rl.Reap(0, 0)

	// Should return empty as all encodings fail during Push
	require.Empty(t, result, "should return empty when all transactions fail encoding")
}

// Tests for Cosmos transactions

func TestReapList_PushCosmosTx(t *testing.T) {
	rl := mempool.NewReapList(newDeterministicEncoder(100, 150))

	// Add Cosmos transactions
	for i := 0; i < 5; i++ {
		tx := newMockCosmosTx(i, 50000)
		err := rl.PushCosmosTx(tx)
		require.NoError(t, err)
	}

	result := rl.Reap(0, 0)

	require.Len(t, result, 5, "should reap all Cosmos transactions")
	// Each Cosmos tx should be 150 bytes
	for _, txBytes := range result {
		require.Len(t, txBytes, 150, "Cosmos tx should have expected size")
	}
}

func TestReapList_MixedEVMAndCosmosTx(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	// EVM txs are 100 bytes, Cosmos txs are 150 bytes
	rl := mempool.NewReapList(newDeterministicEncoder(100, 150))

	// Add mixed transactions
	evmTx1 := testEVMTx(t, key, 0, 21000)
	err = rl.PushEVMTx(evmTx1)
	require.NoError(t, err)

	cosmosTx1 := newMockCosmosTx(0, 50000)
	err = rl.PushCosmosTx(cosmosTx1)
	require.NoError(t, err)

	evmTx2 := testEVMTx(t, key, 1, 30000)
	err = rl.PushEVMTx(evmTx2)
	require.NoError(t, err)

	cosmosTx2 := newMockCosmosTx(1, 60000)
	err = rl.PushCosmosTx(cosmosTx2)
	require.NoError(t, err)

	// Reap with byte limit: 100 + 150 + 100 = 350, should get first 3
	result := rl.Reap(350, 0)
	require.Len(t, result, 3, "should reap 3 mixed transactions")

	// Verify sizes: 100, 150, 100
	require.Len(t, result[0], 100, "first tx should be EVM (100 bytes)")
	require.Len(t, result[1], 150, "second tx should be Cosmos (150 bytes)")
	require.Len(t, result[2], 100, "third tx should be EVM (100 bytes)")

	// Reap remaining
	result = rl.Reap(0, 0)
	require.Len(t, result, 1, "should have 1 Cosmos tx remaining")
	require.Len(t, result[0], 150, "remaining tx should be Cosmos (150 bytes)")
}

func TestReapList_CosmosTxWithGasLimit(t *testing.T) {
	rl := mempool.NewReapList(newDeterministicEncoder(100, 100))

	// Add Cosmos transactions with varying gas
	txGases := []uint64{30000, 40000, 50000, 60000}
	for i, gas := range txGases {
		tx := newMockCosmosTx(i, gas)
		err := rl.PushCosmosTx(tx)
		require.NoError(t, err)
	}

	// Limit to 120000 gas (should get first 3: 30000 + 40000 + 50000 = 120000)
	result := rl.Reap(0, 120000)

	require.Len(t, result, 3, "should reap 3 transactions within gas limit")
}

// selectiveCosmosFailEncoder fails encoding for specific cosmos tx IDs
type selectiveCosmosFailEncoder struct {
	failID int
}

func (e *selectiveCosmosFailEncoder) EVMTx(tx *types.Transaction) ([]byte, error) {
	hash := tx.Hash().Bytes()
	result := make([]byte, 100)
	copy(result, hash)
	return result, nil
}

func (e *selectiveCosmosFailEncoder) CosmosTx(tx sdk.Tx) ([]byte, error) {
	mockTx, ok := tx.(*mockCosmosTx)
	if ok && mockTx.id == e.failID {
		return nil, errors.New("encoding failed for specific tx id")
	}

	var txBytes []byte
	if ok {
		txBytes = []byte(fmt.Sprintf("cosmos-tx-%d", mockTx.id))
	} else {
		txBytes = []byte(fmt.Sprintf("cosmos-tx-%p", tx))
	}

	result := make([]byte, 100)
	copy(result, txBytes)
	return result, nil
}

func TestReapList_CosmosEncodingFailure(t *testing.T) {
	// Create encoder that fails for tx with id=1
	failEncoder := &selectiveCosmosFailEncoder{failID: 1}

	rl := mempool.NewReapList(failEncoder)

	// Add Cosmos transactions - tx with id=1 will fail
	for i := 0; i < 5; i++ {
		tx := newMockCosmosTx(i, 50000)
		_ = rl.PushCosmosTx(tx) // Ignore errors for failing encodings
	}

	result := rl.Reap(0, 0)

	// Should get 4 transactions (all except the one that failed)
	require.Len(t, result, 4, "should have 4 transactions that succeeded encoding")
}

// Tests for Drop functionality

func TestReapList_DropEVMTx(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(newDeterministicEncoder(100, 100))

	// Add 5 EVM transactions
	txs := make([]*types.Transaction, 5)
	for i := uint64(0); i < 5; i++ {
		tx := testEVMTx(t, key, i, 21000)
		txs[i] = tx
		err = rl.PushEVMTx(tx)
		require.NoError(t, err)
	}

	// Drop the middle transaction (index 2)
	rl.DropEVMTx(txs[2])

	// Reap should get 4 transactions (the dropped one should be skipped)
	result := rl.Reap(0, 0)
	require.Len(t, result, 4, "should reap 4 transactions after dropping 1")
}

func TestReapList_DropCosmosTx(t *testing.T) {
	rl := mempool.NewReapList(newDeterministicEncoder(100, 100))

	// Add 5 Cosmos transactions
	txs := make([]*mockCosmosTx, 5)
	for i := 0; i < 5; i++ {
		tx := newMockCosmosTx(i, 50000)
		txs[i] = tx
		err := rl.PushCosmosTx(tx)
		require.NoError(t, err)
	}

	// Drop the first and last transactions
	rl.DropCosmosTx(txs[0])
	rl.DropCosmosTx(txs[4])

	// Reap should get 3 transactions
	result := rl.Reap(0, 0)
	require.Len(t, result, 3, "should reap 3 transactions after dropping 2")
}

func TestReapList_DropAfterReap(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(newDeterministicEncoder(100, 100))

	// Add 5 transactions
	txs := make([]*types.Transaction, 5)
	for i := uint64(0); i < 5; i++ {
		tx := testEVMTx(t, key, i, 21000)
		txs[i] = tx
		err = rl.PushEVMTx(tx)
		require.NoError(t, err)
	}

	// Reap first 3
	result := rl.Reap(300, 0)
	require.Len(t, result, 3)

	// Drop one of the reaped transactions (should be no-op since already reaped)
	rl.DropEVMTx(txs[1])

	// Drop one of the remaining transactions
	rl.DropEVMTx(txs[4])

	// Reap remaining should get only 1 transaction (index 3, since 4 was dropped)
	result = rl.Reap(0, 0)
	require.Len(t, result, 1, "should have 1 transaction remaining after drop")
}

func TestReapList_DropNonExistent(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(newDeterministicEncoder(100, 100))

	// Add 3 transactions
	for i := uint64(0); i < 3; i++ {
		tx := testEVMTx(t, key, i, 21000)
		err = rl.PushEVMTx(tx)
		require.NoError(t, err)
	}

	// Try to drop a transaction that was never added
	nonExistentTx := testEVMTx(t, key, 99, 21000)
	rl.DropEVMTx(nonExistentTx)

	// Should still have all 3 transactions
	result := rl.Reap(0, 0)
	require.Len(t, result, 3, "dropping non-existent tx should not affect list")
}

func TestReapList_MixedDrops(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	rl := mempool.NewReapList(newDeterministicEncoder(100, 150))

	// Add mixed transactions
	evmTxs := make([]*types.Transaction, 3)
	for i := uint64(0); i < 3; i++ {
		tx := testEVMTx(t, key, i, 21000)
		evmTxs[i] = tx
		err = rl.PushEVMTx(tx)
		require.NoError(t, err)
	}

	cosmosTxs := make([]*mockCosmosTx, 3)
	for i := 0; i < 3; i++ {
		tx := newMockCosmosTx(i, 50000)
		cosmosTxs[i] = tx
		err = rl.PushCosmosTx(tx)
		require.NoError(t, err)
	}

	// Drop one EVM and one Cosmos tx
	rl.DropEVMTx(evmTxs[1])
	rl.DropCosmosTx(cosmosTxs[2])

	// Should have 4 remaining (2 EVM + 2 Cosmos)
	result := rl.Reap(0, 0)
	require.Len(t, result, 4, "should have 4 transactions after mixed drops")
}
