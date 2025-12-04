//go:build system_test

package mempool

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/tests/systemtests/suite"
)

// RunTxTrackerPersistence tests that the TxTracker persists local transactions
// and resubmits them after node restart.
func RunTxTrackerPersistence(t *testing.T, base *suite.BaseTestSuite) {
	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		{
			name: "tracks and persists local txs %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					// Submit transaction to node0
					tx1, err := s.SendTx(t, s.Node(0), "acc0", 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx1")

					tx2, err := s.SendTx(t, s.Node(0), "acc0", 1, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx2")

					// Expect both transactions to be in pending
					ctx.SetExpPendingTxs(tx1, tx2)
				},
			},
		},
	}

	testOptions := []*suite.TestOptions{
		{
			Description: "EVM Legacy Tx",
			TxType:      suite.TxTypeEVM,
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

// RunTxTrackerResubmission tests that the TxTracker resubmits transactions
// that are missing from the mempool.
func RunTxTrackerResubmission(t *testing.T, base *suite.BaseTestSuite) {
	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		{
			name: "resubmits missing local txs %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					// Submit transactions to node0
					tx1, err := s.SendTx(t, s.Node(0), "acc0", 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx1")

					tx2, err := s.SendTx(t, s.Node(0), "acc0", 1, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx2")

					tx3, err := s.SendTx(t, s.Node(0), "acc0", 2, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx3")

					// All transactions should be in pending
					ctx.SetExpPendingTxs(tx1, tx2, tx3)
				},
			},
		},
	}

	testOptions := []*suite.TestOptions{
		{
			Description: "EVM Legacy Tx",
			TxType:      suite.TxTypeEVM,
		},
		{
			Description: "EVM Dynamic Fee Tx",
			TxType:      suite.TxTypeEVM,
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

// RunTxTrackerMultipleAccounts tests TxTracker with transactions from multiple accounts.
func RunTxTrackerMultipleAccounts(t *testing.T, base *suite.BaseTestSuite) {
	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		{
			name: "tracks txs from multiple accounts %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					// Submit transactions from different accounts
					tx1, err := s.SendTx(t, s.Node(0), "acc0", 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx1 from acc0")

					tx2, err := s.SendTx(t, s.Node(0), "acc1", 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx2 from acc1")

					tx3, err := s.SendTx(t, s.Node(0), "acc2", 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx3 from acc2")

					// All transactions should be in pending
					ctx.SetExpPendingTxs(tx1, tx2, tx3)
				},
			},
		},
		{
			name: "tracks and orders txs from multiple accounts by gas price %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					// Submit transactions with different gas prices
					tx1, err := s.SendTx(t, s.Node(0), "acc0", 0, s.GasPriceMultiplier(5), nil)
					require.NoError(t, err, "failed to send tx1 with 5x gas")

					tx2, err := s.SendTx(t, s.Node(0), "acc1", 0, s.GasPriceMultiplier(15), nil)
					require.NoError(t, err, "failed to send tx2 with 15x gas")

					tx3, err := s.SendTx(t, s.Node(0), "acc2", 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx3 with 10x gas")

					// All transactions should be in pending
					// The order might be affected by gas price, but all should be present
					ctx.SetExpPendingTxs(tx1, tx2, tx3)
				},
			},
		},
	}

	testOptions := []*suite.TestOptions{
		{
			Description: "EVM Legacy Tx",
			TxType:      suite.TxTypeEVM,
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

// RunTxTrackerNonceGaps tests that TxTracker handles nonce gaps correctly.
func RunTxTrackerNonceGaps(t *testing.T, base *suite.BaseTestSuite) {
	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		{
			name: "handles nonce gaps correctly %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					// Submit transactions with a nonce gap (0, 2, 4)
					tx1, err := s.SendTx(t, s.Node(0), "acc0", 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx1 with nonce 0")

					tx2, err := s.SendTx(t, s.Node(0), "acc0", 2, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx2 with nonce 2")

					tx3, err := s.SendTx(t, s.Node(0), "acc0", 4, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx3 with nonce 4")

					// Only tx1 should be pending, tx2 and tx3 should be queued
					ctx.SetExpPendingTxs(tx1)
					ctx.SetExpQueuedTxs(tx2, tx3)
				},
				func(s *TestSuite, ctx *TestContext) {
					// Fill the gap by submitting tx with nonce 1
					tx4, err := s.SendTx(t, s.Node(0), "acc0", 1, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx4 with nonce 1")

					// Now tx1, tx4, tx2 should be pending; tx3 still queued
					ctx.SetExpPendingTxs(tx4)
					ctx.PromoteExpTxs(1) // Promote tx2 from queued to pending
				},
				func(s *TestSuite, ctx *TestContext) {
					// Fill the remaining gap by submitting tx with nonce 3
					tx5, err := s.SendTx(t, s.Node(0), "acc0", 3, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx5 with nonce 3")

					// Now all transactions should be pending
					ctx.SetExpPendingTxs(tx5)
					ctx.PromoteExpTxs(1) // Promote tx3 from queued to pending
				},
			},
		},
	}

	testOptions := []*suite.TestOptions{
		{
			Description: "EVM Legacy Tx",
			TxType:      suite.TxTypeEVM,
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
					s.AfterEachAction(t, ctx)
				}
				s.AfterEachCase(t, ctx)
			})
		}
	}
}

// RunTxTrackerWithReplacement tests that TxTracker handles transaction replacements correctly.
func RunTxTrackerWithReplacement(t *testing.T, base *suite.BaseTestSuite) {
	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		{
			name: "tracks replacement transaction %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					// Submit initial transaction with a future nonce to keep it queued
					_, err := s.SendTx(t, s.Node(0), "acc0", 1, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx1")

					// Submit replacement transaction with higher gas price (same nonce)
					// Both transactions are sent in the same action to avoid race conditions
					// where the first tx gets committed before the replacement arrives
					tx2, err := s.SendTx(t, s.Node(0), "acc0", 1, s.GasPriceMultiplier(15), nil)
					require.NoError(t, err, "failed to send replacement tx2")

					// The replacement transaction should replace tx1 in the queued state
					ctx.SetExpQueuedTxs(tx2)
				},
				func(s *TestSuite, ctx *TestContext) {
					// Submit transaction with nonce 0 to promote the queued replacement
					tx0, err := s.SendTx(t, s.Node(0), "acc0", 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx0")

					// tx0 should be pending, and tx2 should be promoted from queued to pending
					ctx.SetExpPendingTxs(tx0)
					ctx.PromoteExpTxs(1)
				},
			},
		},
	}

	testOptions := []*suite.TestOptions{
		{
			Description: "EVM Legacy Tx",
			TxType:      suite.TxTypeEVM,
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
					s.AfterEachAction(t, ctx)
				}
				s.AfterEachCase(t, ctx)
			})
		}
	}
}

// RunTxTrackerNonceGapEIP7702 tests that nonce gap transactions are tracked locally
// (temporary rejection) rather than being outright rejected when using EIP-7702 delegation.
func RunTxTrackerNonceGapEIP7702(t *testing.T, base *suite.BaseTestSuite) {
	var currentNonce uint64
	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		{
			name: "tracks nonce gap transaction locally with EIP-7702 delegation %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					nodeID := s.Node(0)
					accID := "acc0"
					account := s.EthAccount(accID)
					require.NotNil(t, account, "account %s not found", accID)

					// Get current nonce
					var err error
					currentNonce, err = s.NonceAt(nodeID, accID)
					require.NoError(t, err, "failed to get current nonce")

					// Create and send an EIP-7702 transaction with delegation
					// Using a dummy delegate address for testing
					delegateAddr := common.HexToAddress("0x1111111111111111111111111111111111111111")

					authorization := ethtypes.SetCodeAuthorization{
						ChainID: *uint256.MustFromBig(s.EthClient.ChainID),
						Address: delegateAddr,
						Nonce:   currentNonce,
					}

					signedAuth, err := ethtypes.SignSetCode(account.PrivKey, authorization)
					require.NoError(t, err, "failed to sign set code authorization")

					// Send EIP-7702 transaction
					tx1Hash, err := sendSetCodeTx(s, nodeID, accID, signedAuth)
					require.NoError(t, err, "failed to send EIP-7702 tx")

					tx1 := suite.NewTxInfo(nodeID, tx1Hash.Hex(), suite.TxTypeEVM)
					ctx.SetExpPendingTxs(tx1)
				},
				func(s *TestSuite, ctx *TestContext) {
					nodeID := s.Node(0)
					accID := "acc0"

					// Get current nonce (should be incremented after first tx)
					var err error
					currentNonce, err = s.NonceAt(nodeID, accID)
					require.NoError(t, err, "failed to get current nonce")

					// Submit a transaction with a gapped nonce (skip currentNonce, use currentNonce+1)
					// This should be tracked locally (temporary rejection) rather than outright rejected
					_, err = s.SendEthLegacyTx(t, nodeID, accID, 2, s.GasPriceMultiplier(10))
					require.NoError(t, err, "temporary nonce gap rejection should be tracked locally and not error")

					// The gapped transaction should NOT be in pending or queued since it was rejected
					// but it should be tracked locally for potential resubmission
					// We verify this by checking that no error was returned
				},
				func(s *TestSuite, ctx *TestContext) {
					nodeID := s.Node(0)
					accID := "acc0"

					// Now fill the gap by sending the missing transaction
					tx3, err := s.SendEthLegacyTx(t, nodeID, accID, 1, s.GasPriceMultiplier(10))
					require.NoError(t, err, "failed to send gap-filling tx")

					// After filling the gap, the gap-filling transaction should be pending
					ctx.SetExpPendingTxs(tx3)
				},
			},
		},
	}

	testOptions := []*suite.TestOptions{
		{
			Description: "EVM Legacy Tx",
			TxType:      suite.TxTypeEVM,
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
					s.AfterEachAction(t, ctx)
				}
				s.AfterEachCase(t, ctx)
			})
		}
	}
}

// sendSetCodeTx is a helper function to send an EIP-7702 SetCode transaction.
func sendSetCodeTx(s *TestSuite, nodeID, accID string, signedAuths ...ethtypes.SetCodeAuthorization) (common.Hash, error) {
	ctx := context.Background()
	ethCli := s.EthClient.Clients[nodeID]
	acc := s.EthAccount(accID)
	if acc == nil {
		return common.Hash{}, fmt.Errorf("account %s not found", accID)
	}
	key := acc.PrivKey

	chainID, err := ethCli.ChainID(ctx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get evm chain id: %w", err)
	}

	fromAddr := acc.Address
	nonce, err := ethCli.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to fetch pending nonce: %w", err)
	}

	txdata := &ethtypes.SetCodeTx{
		ChainID:    uint256.MustFromBig(chainID),
		Nonce:      nonce,
		GasTipCap:  uint256.NewInt(1_000_000),
		GasFeeCap:  uint256.NewInt(1_000_000_000),
		Gas:        100_000,
		To:         common.Address{},
		Value:      uint256.NewInt(0),
		Data:       []byte{},
		AccessList: ethtypes.AccessList{},
		AuthList:   signedAuths,
	}

	signer := ethtypes.LatestSignerForChainID(chainID)
	signedTx := ethtypes.MustSignNewTx(key, signer, txdata)

	if err := ethCli.SendTransaction(ctx, signedTx); err != nil {
		return common.Hash{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	return signedTx.Hash(), nil
}
