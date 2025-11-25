package mempool

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/test-go/testify/require"
)

func RunTxsOrdering(t *testing.T, base *suite.BaseTestSuite) {
	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		{
			name: "ordering of pending txs %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					signer := s.Acc(0)

					t.Logf("Starting test with signer: %s (address: %s)", signer.ID, signer.Address.Hex())

					expPendingTxs := make([]*suite.TxInfo, 5)
					for i := 0; i < 5; i++ {
						// nonce order of submitted txs: 3,4,0,1,2
						nonceIdx := uint64((i + 3) % 5)

						// For cosmos tx, we should send tx to one node.
						// Because cosmos pool does not manage queued txs.
						nodeId := "node0"
						if s.GetOptions().TxType == suite.TxTypeEVM {
							// target node order of submitted txs: 0,1,2,3,0
							nodeId = s.Node(i % 4)
						}

						t.Logf("Sending tx %d: nonce=%d to node=%s", i, nonceIdx, nodeId)
						txInfo, err := s.SendTx(t, nodeId, signer.ID, nonceIdx, s.GasPriceMultiplier(10), big.NewInt(1))
						require.NoError(t, err, "failed to send tx")
						t.Logf("Sent tx %d: hash=%s, nonce=%d, node=%s", i, txInfo.TxHash, nonceIdx, nodeId)

						// nonce order of committed txs: 0,1,2,3,4
						expPendingTxs[nonceIdx] = txInfo
					}

					t.Logf("All txs sent, waiting for 4 blocks for gossip and commit")
					// Because txs are sent to different nodes, we need to wait for some blocks
					// so that all nonce-gapped txs are gossiped to all nodes and committed sequentially.
					s.AwaitNBlocks(t, 4)
					t.Logf("Finished waiting for blocks")

					// Log current mempool state for debugging
					for _, nodeID := range s.Nodes() {
						pending, queued, err := s.TxPoolContent(nodeID, s.GetOptions().TxType, 5*time.Second)
						if err != nil {
							t.Logf("WARNING: Failed to get txpool content from %s: %v", nodeID, err)
						} else {
							t.Logf("Node %s mempool state: pending=%d, queued=%d", nodeID, len(pending), len(queued))
							if len(pending) > 0 {
								t.Logf("  Pending txs on %s: %v", nodeID, pending)
							}
							if len(queued) > 0 {
								t.Logf("  Queued txs on %s: %v", nodeID, queued)
							}
						}
					}

					ctx.SetExpPendingTxs(expPendingTxs...)
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
					// NOTE: In this test, we don't need to check mempool state after each action
					// because we check the final state after all actions are done.
					// s.AfterEachAction(t, ctx) --- IGNORE ---
				}
				s.AfterEachCase(t, ctx)
			})
		}
	}
}
