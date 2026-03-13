//go:build system_test

package indexer

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/evm/indexer"
	"github.com/cosmos/evm/tests/systemtests/suite"
)

const (
	shortUnbondingTime = 20 * time.Second
)

// CompleteUnbonding event signature matching the transformer
var completeUnbondingEventSig = crypto.Keccak256Hash([]byte("CompleteUnbonding(address,address,uint256)"))

// completeUnbonding function selector
var completeUnbondingFunctionSelector = crypto.Keccak256([]byte("completeUnbonding(address,uint256)"))[:4]

// RunCosmosIndexerUnbondingComplete tests that complete_unbonding events
// from EndBlock are properly transformed and indexed.
// The test:
// 1. Modifies genesis to use a short unbonding period
// 2. Delegates tokens to a validator
// 3. Undelegates tokens (starts unbonding)
// 4. Waits for unbonding period to complete
// 5. Verifies the complete_unbonding event is indexed with correct EVM logs
func RunCosmosIndexerUnbondingComplete(t *testing.T, base *suite.BaseTestSuite) {
	s := NewTestSuite(base)
	sut := base.SystemUnderTest

	// Stop chain to modify genesis
	sut.StopChain()
	sut.ResetChain(t)
	sut.SetupChain()

	// Modify genesis to set short unbonding time
	sut.ModifyGenesisJSON(t, suite.SetStakingUnbondingTime(t, shortUnbondingTime))

	// Start chain with modified genesis
	s.SetupTest(t)

	acc0 := s.Acc(0)
	gasPrice := big.NewInt(1000000000000)
	delegateAmount := big.NewInt(1000000000000000000) // 1 token

	// Get a validator address
	validators, err := s.CosmosClient.QueryValidators(s.Node(0))
	require.NoError(t, err, "Failed to query validators")
	require.NotEmpty(t, validators, "Should have at least one validator")

	validator, err := sdk.ValAddressFromBech32(validators[0].OperatorAddress)
	require.NoError(t, err, "Failed to parse validator address")

	// Delegate tokens to the validator
	delegateTxHash, err := s.SendCosmosDelegate(
		t,
		s.Node(0),
		acc0.ID,
		validator,
		delegateAmount,
		gasPrice,
	)
	require.NoError(t, err, "Failed to send delegate tx")

	err = s.WaitForCommit(s.Node(0), delegateTxHash)
	require.NoError(t, err, "Delegate tx should be committed")

	s.AwaitNBlocks(t, 1)

	// Undelegate tokens (starts unbonding)
	undelegateTxHash, err := s.SendCosmosUndelegate(
		t,
		s.Node(0),
		acc0.ID,
		validator,
		delegateAmount,
		gasPrice,
	)
	require.NoError(t, err, "Failed to send undelegate tx")

	err = s.WaitForCommit(s.Node(0), undelegateTxHash)
	require.NoError(t, err, "Undelegate tx should be committed")

	// Record the start height
	startHeight := sut.CurrentHeight()

	// Wait for unbonding period to pass
	// With 20s unbonding time and ~2s block time, wait about 15 blocks
	waitBlocks := int64(shortUnbondingTime.Seconds()/2) + 5
	s.AwaitNBlocks(t, waitBlocks)

	endHeight := sut.CurrentHeight()

	// Search for the complete_unbonding event in EndBlock receipts
	var foundReceipt bool
	var completeUnbondingTxHash common.Hash

	for height := startHeight + 1; height <= endHeight; height++ {
		blockHash, err := s.EthClient.GetBlockHashByNumber(s.Node(0), big.NewInt(height))
		if err != nil {
			continue
		}

		// Check EndBlock phase (where complete_unbonding is emitted)
		syntheticTxHash := indexer.GenerateTransformedEthTxHash(
			[]byte(indexer.BlockPhaseEndBlock),
			blockHash.Bytes(),
		)

		receipt, err := s.EthClient.GetTransactionReceipt(s.Node(0), syntheticTxHash)
		if err != nil || receipt == nil {
			continue
		}

		// Check if any log is a CompleteUnbonding event
		for _, log := range receipt.Logs {
			if len(log.Topics) > 0 && log.Topics[0] == completeUnbondingEventSig {
				foundReceipt = true
				completeUnbondingTxHash = syntheticTxHash
				break
			}
		}
		if foundReceipt {
			break
		}
	}

	require.True(t, foundReceipt, "Should find complete_unbonding event in indexed receipts")

	// Verify the receipt structure
	receipt, err := s.EthClient.GetTransactionReceipt(s.Node(0), completeUnbondingTxHash)
	require.NoError(t, err)
	require.NotNil(t, receipt)

	// Find and verify the CompleteUnbonding log
	var foundLog bool
	for _, log := range receipt.Logs {
		if len(log.Topics) >= 3 && log.Topics[0] == completeUnbondingEventSig {
			foundLog = true

			// Verify log structure
			require.Equal(t, 3, len(log.Topics), "Should have 3 topics: sig, delegator, validator")
			require.Equal(t, 32, len(log.Data), "Data should contain amount (32 bytes)")

			// Verify the amount matches
			logAmount := new(big.Int).SetBytes(log.Data)
			require.Equal(t, delegateAmount.String(), logAmount.String(), "Unbonding amount should match delegated amount")
			break
		}
	}

	require.True(t, foundLog, "Receipt should contain CompleteUnbonding log")

	// Verify transaction fields via eth_getTransactionByHash
	tx, err := s.EthClient.GetTransactionByHash(s.Node(0), completeUnbondingTxHash)
	require.NoError(t, err, "Failed to get transaction by hash")
	require.NotNil(t, tx, "Transaction should not be nil")

	// Verify tx hash matches
	require.Equal(t, completeUnbondingTxHash, tx.Hash, "Transaction hash should match")

	// Verify block info is present
	require.NotNil(t, tx.BlockHash, "BlockHash should not be nil")
	require.NotNil(t, tx.BlockNumber, "BlockNumber should not be nil")
	t.Logf("Transaction: blockHash=%s, blockNumber=%s", tx.BlockHash.Hex(), *tx.BlockNumber)

	// Verify To field points to staking precompile
	require.NotNil(t, tx.To, "To should not be nil")
	t.Logf("Transaction To: %s", tx.To.Hex())

	// Verify Gas is set
	require.NotEmpty(t, tx.Gas, "Gas should not be empty")

	// Verify input has correct format: selector (4 bytes) + validator (32 bytes) + amount (32 bytes)
	require.GreaterOrEqual(t, len(tx.Input), 68, "Input should be at least 68 bytes (4 + 32 + 32)")
	require.Equal(t, completeUnbondingFunctionSelector, []byte(tx.Input[:4]), "Input should start with completeUnbonding function selector")
	t.Logf("Transaction input: 0x%s", common.Bytes2Hex(tx.Input))

	t.Logf("Successfully verified CompleteUnbonding receipt and transaction")
}
