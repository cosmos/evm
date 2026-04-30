//go:build system_test

package mempool

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/tests/systemtests/suite"
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

						txInfo, err := s.SendTx(t, nodeId, signer.ID, nonceIdx, s.GasPriceMultiplier(10), big.NewInt(1))
						require.NoError(t, err, "failed to send tx to node %s, nonce %d", nodeId, nonceIdx)

						// nonce order of committed txs: 0,1,2,3,4
						expPendingTxs[nonceIdx] = txInfo
					}

					// Because txs are sent to different nodes, we need to wait for some blocks
					// so that all nonce-gapped txs are gossiped to all nodes and committed sequentially.
					s.AwaitNBlocks(t, 4)
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

// RunSetCode7702QueuedTxPromotion proves that a queued tx whose nonce gap
// is closed by a self-sponsored 7702 actually promotes and lands. Submits
// tx5 (gap of 4 from chain head), then a self-sponsored 7702 with 4
// self-auths to bump the sender's chain nonce by 5. Asserts both land.
func RunSetCode7702QueuedTxPromotion(t *testing.T, base *suite.BaseTestSuite) {
	s := NewTestSuite(base)
	s.SetupTest(t)
	s.SetOptions(&suite.TestOptions{
		Description: "EVM SetCodeTx promotion lag",
		TxType:      suite.TxTypeEVM,
	})

	t.Run("queued tx promotes after self-sponsored 7702 closes the gap", func(t *testing.T) {
		ctx := NewTestContext()
		s.BeforeEachCase(t, ctx)

		signer := s.Acc(0)
		nodeID := "node0"

		startNonce, err := s.NonceAt(nodeID, signer.ID)
		require.NoError(t, err)

		// Queued tx with gap of 4 from chain head.
		queuedTxInfo, err := s.SendTx(t, nodeID, signer.ID, 5, s.GasPriceMultiplier(10), big.NewInt(1))
		require.NoError(t, err, "failed to send queued tx")
		require.NoError(t, s.CheckTxsQueuedAsync([]*suite.TxInfo{queuedTxInfo}))

		// Self-sponsored 7702 with 4 self-auths at startNonce+1..+4.
		// Sender bump (+1) plus 4 auth bumps -> chain nonce += 5.
		sevenSeven02Hash := sendSelfSponsored7702BulkAuths(t, s, nodeID, signer.ID, startNonce, 4)

		require.NoError(t, s.WaitForCommit(nodeID, sevenSeven02Hash.Hex(), suite.TxTypeEVM, 60*time.Second))
		require.NoError(t, s.WaitForCommit(nodeID, queuedTxInfo.TxHash, suite.TxTypeEVM, 60*time.Second))

		// Chain nonce must reflect 7702 (+5) and queued tx (+1).
		finalNonce, err := s.NonceAt(nodeID, signer.ID)
		require.NoError(t, err)
		require.GreaterOrEqual(t, finalNonce, startNonce+6)

		// Clear delegation so subsequent shared-suite cases see acc0 clean.
		clearDelegation(t, s, nodeID, signer.ID, finalNonce)
	})
}

// sendSelfSponsored7702BulkAuths constructs and sends a SetCodeTx where
// the sender is also the authority for `numAuths` consecutive auth slots
// starting at startNonce+1. Returns the tx hash.
func sendSelfSponsored7702BulkAuths(
	t *testing.T,
	s *TestSuite,
	nodeID string,
	accID string,
	startNonce uint64,
	numAuths uint64,
) common.Hash {
	t.Helper()

	ctx := context.Background()
	ethCli := s.EthClient.Clients[nodeID]
	acc := s.EthAccount(accID)
	require.NotNil(t, acc, "account %s not found", accID)

	chainID, err := ethCli.ChainID(ctx)
	require.NoError(t, err)

	// Each auth points at a non-zero placeholder so the delegation
	// actually applies (zero target is the "clear delegation" sentinel
	// and would still bump the nonce, but using a real target keeps the
	// test honest about a delegation actually being installed).
	target := common.Address{0x42}

	auths := make([]ethtypes.SetCodeAuthorization, numAuths)
	for i := uint64(0); i < numAuths; i++ {
		raw := ethtypes.SetCodeAuthorization{
			ChainID: *uint256.MustFromBig(chainID),
			Address: target,
			Nonce:   startNonce + 1 + i,
		}
		signed, signErr := ethtypes.SignSetCode(acc.PrivKey, raw)
		require.NoError(t, signErr, "failed to sign auth %d", i)
		auths[i] = signed
	}

	txdata := &ethtypes.SetCodeTx{
		ChainID:    uint256.MustFromBig(chainID),
		Nonce:      startNonce,
		GasTipCap:  uint256.NewInt(1_000_000),
		GasFeeCap:  uint256.NewInt(1_000_000_000),
		Gas:        200_000,
		To:         common.Address{},
		Value:      uint256.NewInt(0),
		Data:       []byte{},
		AccessList: ethtypes.AccessList{},
		AuthList:   auths,
	}

	txSigner := ethtypes.LatestSignerForChainID(chainID)
	signedTx := ethtypes.MustSignNewTx(acc.PrivKey, txSigner, txdata)

	require.NoError(t, ethCli.SendTransaction(ctx, signedTx),
		fmt.Sprintf("failed to broadcast 7702 tx (nonce=%d, %d auths)", startNonce, numAuths))

	return signedTx.Hash()
}

// clearDelegation sends a self-sponsored 7702 with a single zero-address
// auth to remove any code delegation on the sender.
func clearDelegation(
	t *testing.T,
	s *TestSuite,
	nodeID string,
	accID string,
	currentNonce uint64,
) {
	t.Helper()

	ctx := context.Background()
	ethCli := s.EthClient.Clients[nodeID]
	acc := s.EthAccount(accID)
	require.NotNil(t, acc)

	chainID, err := ethCli.ChainID(ctx)
	require.NoError(t, err)

	clearAuth := ethtypes.SetCodeAuthorization{
		ChainID: *uint256.MustFromBig(chainID),
		Address: common.Address{},
		Nonce:   currentNonce + 1,
	}
	signedClear, err := ethtypes.SignSetCode(acc.PrivKey, clearAuth)
	require.NoError(t, err)

	txdata := &ethtypes.SetCodeTx{
		ChainID:    uint256.MustFromBig(chainID),
		Nonce:      currentNonce,
		GasTipCap:  uint256.NewInt(1_000_000),
		GasFeeCap:  uint256.NewInt(1_000_000_000),
		Gas:        100_000,
		To:         common.Address{},
		Value:      uint256.NewInt(0),
		Data:       []byte{},
		AccessList: ethtypes.AccessList{},
		AuthList:   []ethtypes.SetCodeAuthorization{signedClear},
	}

	txSigner := ethtypes.LatestSignerForChainID(chainID)
	signedTx := ethtypes.MustSignNewTx(acc.PrivKey, txSigner, txdata)

	require.NoError(t, ethCli.SendTransaction(ctx, signedTx),
		"failed to broadcast delegation-clear 7702")
	require.NoError(t, s.WaitForCommit(nodeID, signedTx.Hash().Hex(), suite.TxTypeEVM, 60*time.Second))
}
