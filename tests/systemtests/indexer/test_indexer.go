//go:build system_test

package indexer

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/tests/systemtests/suite"
)

// RunCosmosIndexerBankSend tests that a cosmos bank send transaction
// is indexed and can be queried via eth_getTransactionReceipt with transformed ERC20 logs.
// All events from the same cosmos tx are merged into a single receipt with multiple logs.
// A bank send generates 4 logs:
//   - coin_spent (transfer amount): sender -> zero
//   - coin_received (transfer amount): zero -> receiver
//   - coin_spent (fee): sender -> zero
//   - coin_received (fee): zero -> fee collector
func RunCosmosIndexerBankSend(t *testing.T, base *suite.BaseTestSuite) {
	s := NewTestSuite(base)
	s.SetupTest(t)

	acc0 := s.Acc(0)
	acc1 := s.Acc(1)

	gasPrice := big.NewInt(1000000000000)
	amount := big.NewInt(1000000)

	// Send a cosmos bank send transaction
	cosmosTxHash, err := s.SendCosmosBankSend(
		t,
		s.Node(0),
		acc0.ID,
		acc1.Cosmos.AccAddress,
		amount,
		gasPrice,
	)
	require.NoError(t, err, "Failed to send cosmos bank send")
	require.NotEmpty(t, cosmosTxHash, "Transaction hash should not be empty")

	t.Logf("Cosmos tx hash: %s", cosmosTxHash)

	// Wait for the transaction to be committed
	err = s.WaitForCommit(s.Node(0), cosmosTxHash)
	require.NoError(t, err, "Transaction should be committed successfully")

	// Wait for one more block to ensure indexing is complete
	s.AwaitNBlocks(t, 1)

	// Generate transformed eth tx hash from cosmos tx hash
	// All events from the same cosmos tx share the same transformed eth tx hash
	transformedTxHash := generateTransformedTxHash(t, cosmosTxHash)
	t.Logf("Transformed eth tx hash: %s", transformedTxHash.Hex())

	// Query the transaction receipt
	receipt, err := s.EthClient.GetTransactionReceipt(s.Node(0), transformedTxHash)
	require.NoError(t, err, "Failed to get transaction receipt")
	require.NotNil(t, receipt, "Receipt should not be nil")

	t.Logf("Receipt: status=%d, logs count=%d", receipt.Status, len(receipt.Logs))

	// Verify the receipt has exactly 4 logs (transfer + fee events)
	require.Equal(t, 4, len(receipt.Logs), "Receipt should have exactly 4 Transfer logs")

	// ERC20 Transfer event signature: Transfer(address,address,uint256)
	transferEventSig := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))
	zeroAddrTopic := common.BytesToHash(common.LeftPadBytes(common.Address{}.Bytes(), 32))

	// Verify each log is an ERC20 Transfer event with correct structure
	for i, log := range receipt.Logs {
		require.Equal(t, 3, len(log.Topics), "Log %d should have exactly 3 topics", i)
		require.Equal(t, transferEventSig, log.Topics[0], "Log %d first topic should be Transfer event signature", i)
		require.Equal(t, uint(i), log.Index, "Log %d should have correct index", i)

		fromAddr := common.BytesToAddress(log.Topics[1].Bytes())
		toAddr := common.BytesToAddress(log.Topics[2].Bytes())
		logAmount := new(big.Int).SetBytes(log.Data)
		t.Logf("Log[%d]: from=%s, to=%s, amount=%s", i, fromAddr.Hex(), toAddr.Hex(), logAmount.String())
	}

	// Verify log pattern: coin_spent/coin_received pairs
	// Log[0]: coin_spent (transfer) - from=sender, to=zero
	require.NotEqual(t, zeroAddrTopic, receipt.Logs[0].Topics[1], "Log[0] from should not be zero (coin_spent)")
	require.Equal(t, zeroAddrTopic, receipt.Logs[0].Topics[2], "Log[0] to should be zero (coin_spent)")

	// Log[1]: coin_received (transfer) - from=zero, to=receiver
	require.Equal(t, zeroAddrTopic, receipt.Logs[1].Topics[1], "Log[1] from should be zero (coin_received)")
	require.NotEqual(t, zeroAddrTopic, receipt.Logs[1].Topics[2], "Log[1] to should not be zero (coin_received)")

	// Log[2]: coin_spent (fee) - from=sender, to=zero
	require.NotEqual(t, zeroAddrTopic, receipt.Logs[2].Topics[1], "Log[2] from should not be zero (coin_spent)")
	require.Equal(t, zeroAddrTopic, receipt.Logs[2].Topics[2], "Log[2] to should be zero (coin_spent)")

	// Log[3]: coin_received (fee) - from=zero, to=fee_collector
	require.Equal(t, zeroAddrTopic, receipt.Logs[3].Topics[1], "Log[3] from should be zero (coin_received)")
	require.NotEqual(t, zeroAddrTopic, receipt.Logs[3].Topics[2], "Log[3] to should not be zero (coin_received)")

	// Verify amounts match between pairs
	log0Amount := new(big.Int).SetBytes(receipt.Logs[0].Data)
	log1Amount := new(big.Int).SetBytes(receipt.Logs[1].Data)
	require.Equal(t, log0Amount, log1Amount, "Log[0] and Log[1] amounts should match (transfer pair)")

	log2Amount := new(big.Int).SetBytes(receipt.Logs[2].Data)
	log3Amount := new(big.Int).SetBytes(receipt.Logs[3].Data)
	require.Equal(t, log2Amount, log3Amount, "Log[2] and Log[3] amounts should match (fee pair)")

	// Verify sender is consistent in coin_spent events
	require.Equal(t, receipt.Logs[0].Topics[1], receipt.Logs[2].Topics[1], "Sender should be same in both coin_spent events")

	// Verify transaction fields via eth_getTransactionByHash
	tx, err := s.EthClient.GetTransactionByHash(s.Node(0), transformedTxHash)
	require.NoError(t, err, "Failed to get transaction by hash")
	require.NotNil(t, tx, "Transaction should not be nil")

	// Verify tx hash matches
	require.Equal(t, transformedTxHash, tx.Hash, "Transaction hash should match")

	// Verify block info is present
	require.NotNil(t, tx.BlockHash, "BlockHash should not be nil")
	require.NotNil(t, tx.BlockNumber, "BlockNumber should not be nil")
	require.NotNil(t, tx.TransactionIndex, "TransactionIndex should not be nil")
	t.Logf("Transaction: blockHash=%s, blockNumber=%s, txIndex=%s",
		tx.BlockHash.Hex(), *tx.BlockNumber, *tx.TransactionIndex)

	// Verify To field points to token contract (non-nil for transfer)
	require.NotNil(t, tx.To, "To should not be nil for transfer transaction")
	t.Logf("Transaction To: %s", tx.To.Hex())

	// Verify Gas is set
	require.NotEmpty(t, tx.Gas, "Gas should not be empty")
	t.Logf("Transaction Gas: %s", tx.Gas)

	// Verify input has correct format: selector (4 bytes) + to (32 bytes) + amount (32 bytes)
	// ERC20 transfer(address,uint256) function selector
	transferSelector := crypto.Keccak256([]byte("transfer(address,uint256)"))[:4]
	require.GreaterOrEqual(t, len(tx.Input), 68, "Input should be at least 68 bytes (4 + 32 + 32)")
	require.Equal(t, transferSelector, []byte(tx.Input[:4]), "Input should start with transfer function selector")
	t.Logf("Transaction input: 0x%s", common.Bytes2Hex(tx.Input))

	t.Logf("Successfully verified transformed ERC20 Transfer receipt and transaction for cosmos bank send")
}

// generateTransformedTxHash generates the expected transformed eth tx hash
// using the same algorithm as indexer.GenerateEthTxHash
func generateTransformedTxHash(t *testing.T, cosmosTxHashHex string) common.Hash {
	// Remove 0x prefix if present
	hashHex := strings.TrimPrefix(cosmosTxHashHex, "0x")

	cosmosTxHash, err := hex.DecodeString(hashHex)
	require.NoError(t, err, "Failed to decode cosmos tx hash")

	// Same algorithm as indexer.GenerateEthTxHash: keccak256(cosmosTxHash)
	return crypto.Keccak256Hash(cosmosTxHash)
}
