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

// RunTxBroadcasting tests transaction broadcasting and duplicate handling in a multi-node network.
//
// This test verifies two critical aspects of transaction propagation:
//
//  1. Mempool Broadcasting: Transactions submitted to one node are gossiped to all other nodes
//     via the mempool gossip protocol BEFORE blocks are produced.
//
//  2. Duplicate Detection: The RPC layer properly rejects duplicate transactions submitted
//     by users via JSON-RPC (returning txpool.ErrAlreadyKnown), while internal gossip remains silent.
//
// The test uses a 5-second consensus timeout to create a larger window for verifying that
// transactions appear in other nodes' mempools before blocks are committed.
func RunTxBroadcasting(t *testing.T, base *suite.BaseTestSuite) {
	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		{
			// Scenario 1: Basic Broadcasting and Transaction Promotion
			//
			// This scenario verifies that:
			// 1. Transactions are gossiped to all nodes BEFORE blocks are committed
			// 2. Queued transactions (with nonce gaps) are NOT gossiped
			// 3. When gaps are filled, queued txs are promoted and then gossiped
			//
			// Broadcasting Flow:
			//   User -> JSON-RPC -> Mempool (pending) -> Gossip to peers
			//   When nonce gap filled: Queued -> Promoted to pending -> Gossiped
			name: "tx broadcast to other nodes %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					// Step 1: Send tx with nonce 0 to node0
					// Expected: tx is added to node0's pending pool (nonce is correct)
					signer := s.Acc(0)

					// Send transaction to node0
					tx1, err := s.SendTx(t, s.Node(0), signer.ID, 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx to node0")

					// Step 2: Verify tx appears in nodes 1, 2, 3 mempools within 3 seconds
					// Expected: tx is gossiped to all nodes BEFORE any block is committed (5s timeout)
					// This proves mempool gossip works, not just block propagation
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
					// Step 3: Send tx with nonce 2 to node1 (creating a nonce gap)
					// Current nonce is 1 (after previous tx), so nonce 2 creates a gap
					// Expected: tx is added to node1's QUEUED pool (not pending due to gap)
					signer := s.Acc(0)

					// Send tx with nonce 2 to node1 (creating a gap since current nonce is 1)
					tx3, err := s.SendTx(t, s.Node(1), signer.ID, 2, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx with nonce 2")

					// Step 4: Verify tx is in node1's QUEUED pool (not pending)
					// Queued txs cannot execute until the nonce gap is filled
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

					// Step 5: Verify queued tx is NOT gossiped to other nodes
					// Queued txs should stay local until they become pending (executable)
					// This prevents network spam from non-executable transactions
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

					// Step 6: Send tx with nonce 1 to node2 (filling the gap)
					// Expected: tx is added to node2's pending pool and gossiped
					tx2, err := s.SendTx(t, s.Node(2), signer.ID, 1, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx with nonce 1")

					// Step 7: Verify BOTH txs appear in all nodes' pending pools
					// - tx2 (nonce=1) should be gossiped immediately (it's pending)
					// - tx3 (nonce=2) should be promoted from queued to pending on node1
					// - Promoted tx3 should then be gossiped to all other nodes
					// This proves queued txs get rebroadcast when promoted
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
		{
			// Scenario 2: Duplicate Detection on Same Node
			//
			// This scenario verifies that when a user submits the same transaction twice
			// to the same node via JSON-RPC, the second submission returns an error.
			//
			// Error Handling:
			//   RPC Layer: Checks mempool.Has(txHash) -> returns txpool.ErrAlreadyKnown
			//   Users get immediate error feedback (not silent failure)
			name: "duplicate tx rejected on same node %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					// Step 1: Send tx with nonce 3 to node0
					// Expected: tx is accepted and added to pending pool
					signer := s.Acc(0)

					// Send transaction to node0
					tx1, err := s.SendTx(t, s.Node(0), signer.ID, 3, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx to node0")

					// Step 2: Verify tx is in node0's pending pool
					pendingTxs, _, err := s.TxPoolContent(s.Node(0), suite.TxTypeEVM, 5*time.Second)
					require.NoError(t, err)
					require.Contains(t, pendingTxs, tx1.TxHash, "transaction should be in pending pool")

					// Step 3: Send the SAME transaction again to node0 via JSON-RPC
					// Expected: Error returned (txpool.ErrAlreadyKnown)
					// Users must receive error feedback for duplicate submissions
					_, err = s.SendTx(t, s.Node(0), signer.ID, 3, s.GasPriceMultiplier(10), nil)
					require.Error(t, err, "duplicate tx via JSON-RPC must return error")
					require.Contains(t, err.Error(), "already known", "error should indicate transaction is already known")

					t.Logf("✓ Duplicate transaction correctly rejected with 'already known' error")

					ctx.SetExpPendingTxs(tx1)
				},
			},
		},
		{
			// Scenario 3: Duplicate Detection After Gossip
			//
			// This scenario verifies that even when a node receives a transaction via
			// internal gossip (not user submission), attempting to submit that same
			// transaction again via JSON-RPC returns an error.
			//
			// Flow:
			//   1. User submits tx to node0 -> added to mempool -> gossiped to node1
			//   2. User tries to submit same tx to node1 via JSON-RPC
			//   3. RPC layer detects duplicate (mempool.Has) and returns error
			//
			// This ensures duplicate detection works regardless of how the node
			// originally received the transaction (user submission vs gossip).
			name: "duplicate tx rejected after gossip %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					// Step 1: Send tx with nonce 4 to node0
					// Expected: tx is accepted, added to pending, and gossiped
					signer := s.Acc(0)

					// Send transaction to node0
					tx1, err := s.SendTx(t, s.Node(0), signer.ID, 4, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx to node0")

					// Step 2: Wait for tx to be gossiped to node1
					// Expected: tx appears in node1's pending pool within 3 seconds
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
								tx1.TxHash, maxWaitTime,
							))
						case <-ticker.C:
							pendingTxs, _, err := s.TxPoolContent(s.Node(1), suite.TxTypeEVM, 5*time.Second)
							if err != nil {
								continue
							}
							if slices.Contains(pendingTxs, tx1.TxHash) {
								t.Logf("✓ Transaction %s broadcast to node1 via gossip", tx1.TxHash)
								found = true
							}
						}
					}

					// Step 3: Try to send the SAME transaction to node1 via JSON-RPC
					// Expected: Error returned (txpool.ErrAlreadyKnown)
					// Even though node1 received it via gossip (not user submission),
					// the RPC layer should still detect and reject the duplicate
					_, err = s.SendTx(t, s.Node(1), signer.ID, 4, s.GasPriceMultiplier(10), nil)
					require.Error(t, err, "duplicate tx via JSON-RPC should return error even after gossip")
					require.Contains(t, err.Error(), "already known", "error should indicate transaction is already known")

					t.Logf("✓ JSON-RPC correctly rejects duplicate that node already has from gossip")

					ctx.SetExpPendingTxs(tx1)
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
