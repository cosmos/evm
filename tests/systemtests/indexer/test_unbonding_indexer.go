//go:build system_test

package indexer

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/cosmos/cosmos-sdk/testutil/systemtests"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/evm/indexer"
	"github.com/cosmos/evm/tests/systemtests/suite"
)

const (
	shortUnbondingTime = 20 * time.Second
)

// CompleteUnbonding event signature matching the transformer
var completeUnbondingEventSig = crypto.Keccak256Hash([]byte("CompleteUnbonding(address,address,uint256)"))

// RunCosmosIndexerUnbondingComplete tests that complete_unbonding events
// from BeginBlock are properly transformed and indexed.
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

	// Get a validator address using CLI
	cli := systemtests.NewCLIWrapper(t, sut, systemtests.Verbose)

	// Verify unbonding time is set correctly
	stakingParamsJSON := cli.CustomQuery("q", "staking", "params")
	unbondingTime := gjson.Get(stakingParamsJSON, "params.unbonding_time").String()
	t.Logf("Staking params unbonding_time: %s", unbondingTime)

	validatorsJSON := cli.CustomQuery("q", "staking", "validators")
	validatorAddr := gjson.Get(validatorsJSON, "validators.0.operator_address").String()
	require.NotEmpty(t, validatorAddr, "Should have at least one validator")
	t.Logf("Using validator: %s", validatorAddr)

	validator, err := sdk.ValAddressFromBech32(validatorAddr)
	require.NoError(t, err, "Failed to parse validator address")

	// Delegate tokens to the validator
	t.Log("Delegating tokens to validator...")
	delegateTxHash, err := s.SendCosmosDelegate(
		t,
		s.Node(0),
		acc0.ID,
		validator,
		delegateAmount,
		gasPrice,
	)
	require.NoError(t, err, "Failed to send delegate tx")
	t.Logf("Delegate tx hash: %s", delegateTxHash)

	err = s.WaitForCommit(s.Node(0), delegateTxHash)
	require.NoError(t, err, "Delegate tx should be committed")

	// Wait a block to ensure delegation is processed
	s.AwaitNBlocks(t, 1)

	// Undelegate tokens (starts unbonding)
	t.Log("Undelegating tokens from validator...")
	undelegateTxHash, err := s.SendCosmosUndelegate(
		t,
		s.Node(0),
		acc0.ID,
		validator,
		delegateAmount,
		gasPrice,
	)
	require.NoError(t, err, "Failed to send undelegate tx")
	t.Logf("Undelegate tx hash: %s", undelegateTxHash)

	err = s.WaitForCommit(s.Node(0), undelegateTxHash)
	require.NoError(t, err, "Undelegate tx should be committed")

	// Record the start height
	startHeight := sut.CurrentHeight()
	t.Logf("Unbonding started at height %d", startHeight)

	// Wait for unbonding period to pass
	// With 20s unbonding time and ~2s block time, wait about 15 blocks
	waitBlocks := int64(shortUnbondingTime.Seconds()/2) + 5
	t.Logf("Waiting %d blocks for unbonding to complete...", waitBlocks)
	s.AwaitNBlocks(t, waitBlocks)

	endHeight := sut.CurrentHeight()
	t.Logf("After waiting, at height %d", endHeight)

	// Search for the complete_unbonding event in block phase receipts
	// Check all phases: PreBlock, BeginBlock, EndBlock
	var foundReceipt bool
	var completeUnbondingTxHash common.Hash

	t.Logf("Searching for complete_unbonding event from height %d to %d", startHeight+1, endHeight)

	phases := []indexer.BlockPhase{
		indexer.BlockPhasePreBlock,
		indexer.BlockPhaseBeginBlock,
		indexer.BlockPhaseEndBlock,
	}

	for height := startHeight + 1; height <= endHeight; height++ {
		// Get block hash for this height
		blockResult, err := s.EthClient.GetBlockByNumber(s.Node(0), big.NewInt(height))
		if err != nil {
			t.Logf("Failed to get block at height %d: %v", height, err)
			continue
		}

		blockHash := blockResult.Hash()

		// Check all phases
		for _, phase := range phases {
			syntheticTxHash := indexer.GenerateSyntheticEthTxHash(
				[]byte(phase),
				blockHash.Bytes(),
			)

			// Try to get the receipt
			receipt, err := s.EthClient.GetTransactionReceipt(s.Node(0), syntheticTxHash)
			if err != nil || receipt == nil {
				continue
			}

			t.Logf("Height %d, Phase %s: Found receipt with %d logs", height, phase, len(receipt.Logs))

			// Check if any log is a CompleteUnbonding event
			for i, log := range receipt.Logs {
				t.Logf("  Log %d: Address=%s, Topics[0]=%s", i, log.Address.Hex(), log.Topics[0].Hex())
				if len(log.Topics) > 0 && log.Topics[0] == completeUnbondingEventSig {
					foundReceipt = true
					completeUnbondingTxHash = syntheticTxHash
					t.Logf("Found complete_unbonding event in block %d phase %s, tx hash: %s", height, phase, syntheticTxHash.Hex())
					break
				}
			}
			if foundReceipt {
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

			delegatorFromLog := common.BytesToAddress(log.Topics[1].Bytes())
			validatorFromLog := common.BytesToAddress(log.Topics[2].Bytes())

			t.Logf("CompleteUnbonding log verified:")
			t.Logf("  delegator: %s", delegatorFromLog.Hex())
			t.Logf("  validator: %s", validatorFromLog.Hex())
			t.Logf("  amount: %s", logAmount.String())
			break
		}
	}

	require.True(t, foundLog, "Receipt should contain CompleteUnbonding log")
	t.Log("Successfully verified complete_unbonding event indexing")
}
