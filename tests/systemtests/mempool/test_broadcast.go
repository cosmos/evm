//go:build system_test

package mempool

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/test-go/testify/require"

	"github.com/cosmos/evm/tests/systemtests/suite"
)

// RunTxDuplicateHandling tests that duplicate transactions are properly rejected when submitted via JSON-RPC.
//
// IMPORTANT: This test currently FAILS because ErrAlreadyKnown is silently converted to success in check_tx.go.
// The fix needs to be implemented on the RPC side to distinguish between:
//   - User-submitted duplicates (JSON-RPC) -> MUST return error
//   - Internal rebroadcast/gossip -> Should be silent (current behavior is correct for this)
//
// When a duplicate transaction is sent via JSON-RPC, the txpool correctly returns ErrAlreadyKnown,
// but CheckTx converts it to success. The RPC handler should intercept this and return an appropriate
// error to the user instead.
func RunTxDuplicateHandling(t *testing.T, base *suite.BaseTestSuite) {
	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		{
			name: "duplicate tx handling %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					signer := s.Acc(0)

					// Send transaction to node0
					tx1, err := s.SendTx(t, s.Node(0), signer.ID, 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx to node0")

					// Verify the transaction is in the pool
					pendingTxs, _, err := s.TxPoolContent(s.Node(0), suite.TxTypeEVM, 5*time.Second)
					require.NoError(t, err)
					require.Contains(t, pendingTxs, tx1.TxHash, "transaction should be in pending pool")

					// Send the SAME transaction again to the same node via JSON-RPC
					// This SHOULD return an error - users need to know when they're sending duplicates
					// Currently this test will FAIL because ErrAlreadyKnown is silently converted to success in check_tx.go
					// TODO: Fix this on the RPC side to return proper error for user-submitted duplicates
					tx1Duplicate, err := s.SendTx(t, s.Node(0), signer.ID, 0, s.GasPriceMultiplier(10), nil)
					require.Error(t, err, "duplicate tx via JSON-RPC MUST return error (currently fails - needs RPC-side fix)")
					require.Contains(t, err.Error(), "already known", "error should indicate transaction is already known")

					// If we got an error (correct behavior), tx1Duplicate will be nil or have empty hash
					// If no error (current broken behavior), verify no duplication occurred
					if err == nil {
						require.Equal(t, tx1.TxHash, tx1Duplicate.TxHash, "duplicate tx should have same hash")

						// Verify the transaction is still in the pool (not duplicated)
						pendingTxs, _, err = s.TxPoolContent(s.Node(0), suite.TxTypeEVM, 5*time.Second)
						require.NoError(t, err)
						require.Contains(t, pendingTxs, tx1.TxHash, "transaction should still be in pending pool")

						// Count occurrences - should be exactly 1
						count := 0
						for _, hash := range pendingTxs {
							if hash == tx1.TxHash {
								count++
							}
						}
						require.Equal(t, 1, count, "transaction should appear exactly once in pending pool, not duplicated")
					}

					t.Logf("✓ Duplicate transaction correctly rejected with error from JSON-RPC")

					ctx.SetExpPendingTxs(tx1)
				},
				func(s *TestSuite, ctx *TestContext) {
					// Test re-gossiping scenario: send duplicate to different node after broadcast
					signer := s.Acc(0)

					// Send transaction to node0
					tx2, err := s.SendTx(t, s.Node(0), signer.ID, 1, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx to node0")

					// Wait for it to be broadcast to node1
					maxWaitTime := 3 * time.Second
					checkInterval := 100 * time.Millisecond

					timeoutCtx, cancel := context.WithTimeout(context.Background(), maxWaitTime)
					defer cancel()

					ticker := time.NewTicker(checkInterval)
					defer ticker.Stop()

					found := false
					for !found {
						select {
						case <-timeoutCtx.Done():
							require.FailNow(t, fmt.Sprintf(
								"transaction %s was not broadcast to node1 within %s",
								tx2.TxHash, maxWaitTime,
							))
						case <-ticker.C:
							pendingTxs, _, err := s.TxPoolContent(s.Node(1), suite.TxTypeEVM, 5*time.Second)
							if err != nil {
								continue
							}
							if slices.Contains(pendingTxs, tx2.TxHash) {
								t.Logf("✓ Transaction %s broadcast to node1", tx2.TxHash)
								found = true
							}
						}
					}

					// Now try to send the same transaction to node1 via JSON-RPC
					// Even though node1 already has it (from gossip), sending it again via JSON-RPC should error
					// This is user-submitted, not internal rebroadcast
					tx2Duplicate, err := s.SendTx(t, s.Node(1), signer.ID, 1, s.GasPriceMultiplier(10), nil)
					require.Error(t, err, "duplicate tx via JSON-RPC should return error even on different node")
					require.Contains(t, err.Error(), "already known", "error should indicate transaction is already known")

					if err == nil {
						require.Equal(t, tx2.TxHash, tx2Duplicate.TxHash, "duplicate tx should have same hash")
					}

					t.Logf("✓ JSON-RPC correctly rejects duplicate transaction that node1 already has from gossip")

					ctx.SetExpPendingTxs(tx2)
				},
			},
		},
	}

	testOptions := []*suite.TestOptions{
		{
			Description:    "EVM LegacyTx",
			TxType:         suite.TxTypeEVM,
			IsDynamicFeeTx: false,
		},
		{
			Description:    "EVM DynamicFeeTx",
			TxType:         suite.TxTypeEVM,
			IsDynamicFeeTx: true,
		},
	}

	s := NewTestSuite(base)
	s.SetupTest(t)

	for _, to := range testOptions {
		s.SetOptions(to)
		for _, tc := range testCases {
			testName := fmt.Sprintf(tc.name, to.Description)
			t.Run(testName, func(t *testing.T) {
				ctx := NewTestContext()
				s.BeforeEachCase(t, ctx)
				for _, action := range tc.actions {
					action(s, ctx)
				}
				s.AfterEachCase(t, ctx)
			})
		}
	}
}

// RunTxBroadcasting tests that transactions are broadcast to other nodes via mempool gossip
// before blocks are committed. This verifies that the mempool rebroadcast functionality works
// correctly and transactions propagate through the network via the mempool gossip protocol,
// not just via block propagation.
//
// The test uses a slower block time (5 seconds) to ensure we have enough time to verify
// that transactions appear in other nodes' mempools before a block is produced.
func RunTxBroadcasting(t *testing.T, base *suite.BaseTestSuite) {
	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		{
			name: "tx broadcast to other nodes %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					signer := s.Acc(0)

					// Send transaction to node0
					tx1, err := s.SendTx(t, s.Node(0), signer.ID, 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx to node0")

					// Verify the transaction appears in other nodes' mempools BEFORE any block is committed
					// This proves transactions are broadcast via mempool gossip, not just block propagation
					maxWaitTime := 3 * time.Second
					checkInterval := 100 * time.Millisecond

					for _, nodeIdx := range []int{1, 2, 3} {
						func(nodeIdx int) {
							nodeID := s.Node(nodeIdx)
							found := false

							timeoutCtx, cancel := context.WithTimeout(context.Background(), maxWaitTime)
							defer cancel()

							ticker := time.NewTicker(checkInterval)
							defer ticker.Stop()

							for !found {
								select {
								case <-timeoutCtx.Done():
									require.FailNow(t, fmt.Sprintf(
										"transaction %s was not broadcast to %s within %s - mempool gossip may not be working",
										tx1.TxHash, nodeID, maxWaitTime,
									))
								case <-ticker.C:
									pendingTxs, _, err := s.TxPoolContent(nodeID, suite.TxTypeEVM, 5*time.Second)
									if err != nil {
										// Retry on error
										continue
									}

									if slices.Contains(pendingTxs, tx1.TxHash) {
										t.Logf("✓ Transaction %s successfully broadcast to %s", tx1.TxHash, nodeID)
										found = true
									}
								}
							}
						}(nodeIdx)
					}

					// Now set expected state and let the transaction commit normally
					ctx.SetExpPendingTxs(tx1)
				},
				func(s *TestSuite, ctx *TestContext) {
					// Test with nonce-gapped transactions to verify rebroadcast/promotion
					signer := s.Acc(0)

					// Send tx with nonce 2 to node1 (creating a gap since current nonce is 1)
					tx3, err := s.SendTx(t, s.Node(1), signer.ID, 2, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx with nonce 2")

					// This transaction should be queued on node1, not pending
					maxWaitTime := 2 * time.Second
					checkInterval := 100 * time.Millisecond

					timeoutCtx, cancel := context.WithTimeout(context.Background(), maxWaitTime)
					defer cancel()

					ticker := time.NewTicker(checkInterval)
					defer ticker.Stop()

					queuedOnNode1 := false
					for !queuedOnNode1 {
						select {
						case <-timeoutCtx.Done():
							require.FailNow(t, fmt.Sprintf(
								"transaction %s was not queued on node1 within %s",
								tx3.TxHash, maxWaitTime,
							))
						case <-ticker.C:
							_, queuedTxs, err := s.TxPoolContent(s.Node(1), suite.TxTypeEVM, 5*time.Second)
							if err != nil {
								continue
							}

							if slices.Contains(queuedTxs, tx3.TxHash) {
								t.Logf("✓ Transaction %s is queued on node1 (as expected due to nonce gap)", tx3.TxHash)
								queuedOnNode1 = true
							}
						}
					}

					// Verify the queued transaction is NOT broadcast to other nodes
					// (queued txs should not be gossiped)
					time.Sleep(1 * time.Second) // Give some time for any potential gossip

					for _, nodeIdx := range []int{0, 2, 3} {
						nodeID := s.Node(nodeIdx)
						pendingTxs, queuedTxs, err := s.TxPoolContent(nodeID, suite.TxTypeEVM, 5*time.Second)
						require.NoError(t, err, "failed to get txpool content from %s", nodeID)

						require.NotContains(t, pendingTxs, tx3.TxHash,
							"queued transaction should not be in pending pool of %s", nodeID)
						require.NotContains(t, queuedTxs, tx3.TxHash,
							"queued transaction should not be broadcast to %s", nodeID)
					}

					// Now send the missing transaction (nonce 1)
					tx2, err := s.SendTx(t, s.Node(2), signer.ID, 1, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx with nonce 1")

					// tx2 should be broadcast to all nodes
					// tx3 should be promoted to pending on node1 and then broadcast to all nodes
					maxWaitTime = 3 * time.Second
					ticker2 := time.NewTicker(checkInterval)
					defer ticker2.Stop()

					for _, nodeIdx := range []int{0, 1, 3} {
						func(nodeIdx int) {
							nodeID := s.Node(nodeIdx)
							foundTx2 := false
							foundTx3 := false

							timeoutCtx2, cancel2 := context.WithTimeout(context.Background(), maxWaitTime)
							defer cancel2()

							for !foundTx2 || !foundTx3 {
								select {
								case <-timeoutCtx2.Done():
									if !foundTx2 {
										require.FailNow(t, fmt.Sprintf(
											"transaction %s was not broadcast to %s within %s",
											tx2.TxHash, nodeID, maxWaitTime,
										))
									}
									if !foundTx3 {
										require.FailNow(t, fmt.Sprintf(
											"transaction %s (promoted from queued) was not broadcast to %s within %s",
											tx3.TxHash, nodeID, maxWaitTime,
										))
									}
								case <-ticker2.C:
									pendingTxs, _, err := s.TxPoolContent(nodeID, suite.TxTypeEVM, 5*time.Second)
									if err != nil {
										continue
									}

									if !foundTx2 && slices.Contains(pendingTxs, tx2.TxHash) {
										t.Logf("✓ Transaction %s broadcast to %s", tx2.TxHash, nodeID)
										foundTx2 = true
									}

									if !foundTx3 && slices.Contains(pendingTxs, tx3.TxHash) {
										t.Logf("✓ Transaction %s (promoted) broadcast to %s", tx3.TxHash, nodeID)
										foundTx3 = true
									}
								}
							}
						}(nodeIdx)
					}

					ctx.SetExpPendingTxs(tx2, tx3)
				},
			},
		},
	}

	testOptions := []*suite.TestOptions{
		{
			Description:    "EVM LegacyTx",
			TxType:         suite.TxTypeEVM,
			IsDynamicFeeTx: false,
		},
		{
			Description:    "EVM DynamicFeeTx",
			TxType:         suite.TxTypeEVM,
			IsDynamicFeeTx: true,
		},
	}

	s := NewTestSuite(base)

	// First, setup the chain with default configuration
	s.SetupTest(t)

	// Now modify the consensus timeout to slow down block production
	// This gives us time to verify broadcasting happens before blocks are committed
	s.ModifyConsensusTimeout(t, "5s")

	for _, to := range testOptions {
		s.SetOptions(to)
		for _, tc := range testCases {
			testName := fmt.Sprintf(tc.name, to.Description)
			t.Run(testName, func(t *testing.T) {
				ctx := NewTestContext()
				s.BeforeEachCase(t, ctx)
				for _, action := range tc.actions {
					action(s, ctx)
					// NOTE: We don't call AfterEachAction here because we're manually
					// checking the mempool state in the action functions
				}
				s.AfterEachCase(t, ctx)
			})
		}
	}
}