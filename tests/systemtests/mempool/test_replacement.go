//go:build system_test

package mempool

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/test-go/testify/require"
)

func RunTxsReplacement(t *testing.T, base *suite.BaseTestSuite) {
	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		// Note: These test cases are unstable in the GitHub CI environment.
		// When running it locally, please uncomment it and run the test.
		//
		// {
		// 	name: "single pending tx submitted to same nodes %s",
		// 	actions: []func(*TestSuite, *TestContext){
		// 		func(s *TestSuite, ctx *TestContext) {
		// 			signer := s.Acc(0)
		// 			_, err := s.SendTx(t, s.Node(0), signer.ID, 0, s.GasPriceMultiplier(10), nil)
		// 			require.NoError(t, err, "failed to send tx")
		// 			tx2, err := s.SendTx(t, s.Node(1), signer.ID, 0, s.GasPriceMultiplier(20), big.NewInt(1))
		// 			require.NoError(t, err, "failed to send tx")

		// 			ctx.SetExpPendingTxs(tx2)
		// 		},
		// 	},
		// },
		// {
		// 	name: "multiple pending txs submitted to same nodes %s",
		// 	actions: []func(*TestSuite, *TestContext){
		// 		func(s *TestSuite, ctx *TestContext) {
		// 			signer := s.Acc(0)
		// 			_, err := s.SendTx(t, s.Node(0), signer.ID, 0, s.GasPriceMultiplier(10), nil)
		// 			require.NoError(t, err, "failed to send tx")
		// 			tx2, err := s.SendTx(t, s.Node(1), signer.ID, 0, s.GasPriceMultiplier(20), big.NewInt(1))
		// 			require.NoError(t, err, "failed to send tx")

		// 			_, err = s.SendTx(t, s.Node(0), signer.ID, 1, s.GasPriceMultiplier(10), nil)
		// 			require.NoError(t, err, "failed to send tx")
		// 			tx4, err := s.SendTx(t, s.Node(1), signer.ID, 1, s.GasPriceMultiplier(20), big.NewInt(1))
		// 			require.NoError(t, err, "failed to send tx")

		// 			_, err = s.SendTx(t, s.Node(0), signer.ID, 2, s.GasPriceMultiplier(10), nil)
		// 			require.NoError(t, err, "failed to send tx")
		// 			tx6, err := s.SendTx(t, s.Node(1), signer.ID, 2, s.GasPriceMultiplier(20), big.NewInt(1))
		// 			require.NoError(t, err, "failed to send tx")

		// 			ctx.SetExpPendingTxs(tx2, tx4, tx6)
		// 		},
		// 	},
		// },
		{
			name: "single queued tx %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					signer := s.Acc(0)
					_, err := s.SendTx(t, s.Node(0), signer.ID, 1, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendTx(t, s.Node(0), signer.ID, 1, s.GasPriceMultiplier(20), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpQueuedTxs(tx2)
				},
				func(s *TestSuite, ctx *TestContext) {
					signer := s.Acc(0)
					txHash, err := s.SendTx(t, s.Node(1), signer.ID, 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpPendingTxs(txHash)
					ctx.PromoteExpTxs(1)
				},
			},
		},
		{
			name: "multiple queued txs %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					signer := s.Acc(0)
					_, err := s.SendTx(t, s.Node(0), signer.ID, 1, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendTx(t, s.Node(0), signer.ID, 1, s.GasPriceMultiplier(20), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					_, err = s.SendTx(t, s.Node(1), signer.ID, 2, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")
					tx4, err := s.SendTx(t, s.Node(1), signer.ID, 2, s.GasPriceMultiplier(20), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					_, err = s.SendTx(t, s.Node(2), signer.ID, 3, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")
					tx6, err := s.SendTx(t, s.Node(2), signer.ID, 3, s.GasPriceMultiplier(20), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpQueuedTxs(tx2, tx4, tx6)
				},
				func(s *TestSuite, ctx *TestContext) {
					signer := s.Acc(0)
					tx, err := s.SendTx(t, s.Node(3), signer.ID, 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpPendingTxs(tx)
					ctx.PromoteExpTxs(3)
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
					s.AfterEachAction(t, ctx)
				}
				s.AfterEachCase(t, ctx)
			})
		}
	}
}

func RunTxsReplacementWithCosmosTx(t *testing.T, base *suite.BaseTestSuite) {
	t.Skip("This test does not work.")

	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		{
			name: "single pending tx submitted to same nodes %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					// NOTE: Currently EVMD cannot handle tx reordering correctly when cosmos tx is used.
					// It is because of CheckTxHandler cannot handle errors from SigVerificationDecorator properly.
					// After modifying CheckTxHandler, we can also modify this test case
					// : high prio cosmos tx should replace low prio evm tx.
					signer := s.Acc(0)
					tx1, err := s.SendTx(t, s.Node(0), signer.ID, 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")
					_, err = s.SendTx(t, s.Node(1), signer.ID, 0, s.GasPriceMultiplier(20), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpPendingTxs(tx1)
				},
			},
		},
		{
			name: "multiple pending txs submitted to same nodes %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					// NOTE: Currently EVMD cannot handle tx reordering correctly when cosmos tx is used.
					// It is because of CheckTxHandler cannot handle errors from SigVerificationDecorator properly.
					// After modifying CheckTxHandler, we can also modify this test case
					// : high prio cosmos tx should replace low prio evm tx.
					signer := s.Acc(0)
					tx1, err := s.SendTx(t, s.Node(0), signer.ID, 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")
					_, err = s.SendTx(t, s.Node(1), signer.ID, 0, s.GasPriceMultiplier(20), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					tx3, err := s.SendTx(t, s.Node(0), signer.ID, 1, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")
					_, err = s.SendTx(t, s.Node(1), signer.ID, 1, s.GasPriceMultiplier(20), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					tx5, err := s.SendTx(t, s.Node(0), signer.ID, 2, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")
					_, err = s.SendTx(t, s.Node(1), signer.ID, 2, s.GasPriceMultiplier(20), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpPendingTxs(tx1, tx3, tx5)
				},
			},
		},
	}

	testOptions := []*suite.TestOptions{
		{
			Description: "Cosmos LegacyTx",
			TxType:      suite.TxTypeCosmos,
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

func RunMixedTxsReplacementEVMAndCosmos(t *testing.T, base *suite.BaseTestSuite) {
	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		{
			name: "single pending tx (low prio evm tx first) %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					tx1, err := s.SendEthTx(t, s.Node(0), "acc0", 0, s.BaseFeeMultiplier(10), s.BaseFeeMultiplier(10))
					require.NoError(t, err, "failed to send tx")

					_, err = s.SendCosmosTx(t, s.Node(0), "acc0", 0, s.BaseFeeMultiplier(20), nil)
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpPendingTxs(tx1)
				},
			},
		},
		{
			name: "single pending tx (high prio evm tx first) %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					tx1, err := s.SendEthTx(t, s.Node(0), "acc0", 0, s.BaseFeeMultiplier(20), s.BaseFeeMultiplier(20))
					require.NoError(t, err, "failed to send tx")
					_, err = s.SendCosmosTx(t, s.Node(1), "acc0", 0, s.BaseFeeMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpPendingTxs(tx1)
				},
			},
		},
		{
			name: "single pending tx (low prio cosmos tx first) %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					_, err := s.SendCosmosTx(t, s.Node(0), "acc0", 0, s.BaseFeeMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendEthTx(t, s.Node(1), "acc0", 0, s.BaseFeeMultiplier(20), s.BaseFeeMultiplier(20))
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpPendingTxs(tx2)
				},
			},
		},
		{
			// NOTE: Skip this test for now
			// This test case is non-deterministic for unknown reason.
			// We need to manually test this case, find the reason, and enable this test case.
			//
			// name: "single pending tx (high prio cosmos tx first) %s",
			// actions: []func(*TestSuite, *TestContext){
			// 	func(s *TestSuite, ctx *TestContext) {
			// 		tx1, err := s.SendCosmosTx(t, s.Node(0), "acc0", 0, s.BaseFeeMultiplier(20), nil)
			// 		require.NoError(t, err, "failed to send tx")
			// 		_, err = s.SendEthTx(t, s.Node(1), "acc0", 0, s.BaseFeeMultiplier(10), nil)
			// 		require.NoError(t, err, "failed to send tx")

			// 		ctx.SetExpPendingTxs(tx1)
			// 	},
			// },
		},
		{
			name: "single queued tx (low prio evm tx first) %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					tx1, err := s.SendEthTx(t, s.Node(0), "acc0", 1, s.BaseFeeMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")
					_, err = s.SendCosmosTx(t, s.Node(0), "acc0", 1, s.BaseFeeMultiplier(20), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					// CosmosTx is not queued in local mempool
					ctx.SetExpQueuedTxs(tx1)
				},
				func(s *TestSuite, ctx *TestContext) {
					tx3, err := s.SendEthTx(t, s.Node(1), "acc0", 0, s.BaseFeeMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpPendingTxs(tx3)
					ctx.PromoteExpTxs(1)
				},
			},
		},
		{
			name: "single queued tx (high prio evm tx first) %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					tx1, err := s.SendEthTx(t, s.Node(0), "acc0", 1, s.BaseFeeMultiplier(20), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")
					_, err = s.SendCosmosTx(t, s.Node(0), "acc0", 1, s.BaseFeeMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					// CosmosTx is not queued in local mempool
					ctx.SetExpQueuedTxs(tx1)
				},
				func(s *TestSuite, ctx *TestContext) {
					tx3, err := s.SendEthTx(t, s.Node(1), "acc0", 0, s.BaseFeeMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpPendingTxs(tx3)
					ctx.PromoteExpTxs(1)
				},
			},
		},
		{
			name: "single queued tx (low prio cosmos tx first) %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					_, err := s.SendCosmosTx(t, s.Node(0), "acc0", 1, s.BaseFeeMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendEthTx(t, s.Node(0), "acc0", 1, s.BaseFeeMultiplier(20), s.BaseFeeMultiplier(20))
					require.NoError(t, err, "failed to send tx")

					// CosmosTx is not queued in local mempool
					ctx.SetExpQueuedTxs(tx2)
				},
				func(s *TestSuite, ctx *TestContext) {
					tx3, err := s.SendEthTx(t, s.Node(1), "acc0", 0, s.BaseFeeMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpPendingTxs(tx3)
					ctx.PromoteExpTxs(1)
				},
			},
		},
		{
			name: "single queued tx (high prio cosmos tx first) %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					_, err := s.SendCosmosTx(t, s.Node(0), "acc0", 1, s.BaseFeeMultiplier(20), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendEthTx(t, s.Node(0), "acc0", 1, s.BaseFeeMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					// CosmosTx is not queued in local mempool
					ctx.SetExpQueuedTxs(tx2)
				},
				func(s *TestSuite, ctx *TestContext) {
					tx3, err := s.SendEthTx(t, s.Node(1), "acc0", 0, s.BaseFeeMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpPendingTxs(tx3)
					ctx.PromoteExpTxs(1)
				},
			},
		},
	}

	testOptions := []*suite.TestOptions{
		{
			Description: "EVM LegacyTx & Cosmos LegacyTx",
		},
		{
			Description:    "EVM DynamicTx & Cosmos LegacyTx",
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
					s.AfterEachAction(t, ctx)
				}
				s.AfterEachCase(t, ctx)
			})
		}
	}
}

func RunMixedTxsReplacementLegacyAndDynamicFee(t *testing.T, base *suite.BaseTestSuite) {
	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		{
			name: "dynamic fee tx should not replace legacy tx",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					signer := s.Acc(0)
					tx1, err := s.SendEthLegacyTx(t, s.Node(0), signer.ID, 1, s.GasPriceMultiplier(10))
					require.NoError(t, err, "failed to send eth legacy tx")

					_, err = s.SendEthDynamicFeeTx(t, s.Node(0), signer.ID, 1, s.GasPriceMultiplier(20), big.NewInt(1))
					require.Error(t, err)
					require.Contains(t, err.Error(), "replacement transaction underpriced")

					ctx.SetExpQueuedTxs(tx1)
				},
				func(s *TestSuite, ctx *TestContext) {
					signer := s.Acc(0)
					txHash, err := s.SendEthLegacyTx(t, s.Node(0), signer.ID, 0, s.GasPriceMultiplier(10))
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpPendingTxs(txHash)
					ctx.PromoteExpTxs(1)
				},
			},
		},
		{
			name: "dynamic fee tx should replace legacy tx",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					signer := s.Acc(0)
					_, err := s.SendEthLegacyTx(t, s.Node(0), signer.ID, 1, s.GasPriceMultiplier(10))
					require.NoError(t, err, "failed to send eth legacy tx")

					tx2, err := s.SendEthDynamicFeeTx(t, s.Node(0), signer.ID, 1,
						s.GasPriceMultiplier(20),
						s.GasPriceMultiplier(20),
					)
					require.NoError(t, err)

					ctx.SetExpQueuedTxs(tx2)
				},
				func(s *TestSuite, ctx *TestContext) {
					signer := s.Acc(0)
					txHash, err := s.SendEthLegacyTx(t, s.Node(0), signer.ID, 0, s.GasPriceMultiplier(10))
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpPendingTxs(txHash)
					ctx.PromoteExpTxs(1)
				},
			},
		},
		{
			name: "legacy should never replace dynamic fee tx",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					signer := s.Acc(0)
					tx1, err := s.SendEthDynamicFeeTx(t, s.Node(0), signer.ID, 1, s.GasPriceMultiplier(20),
						new(big.Int).Sub(s.GasPriceMultiplier(10), big.NewInt(1)))
					require.NoError(t, err)

					_, err = s.SendEthLegacyTx(t, s.Node(0), signer.ID, 1, s.GasPriceMultiplier(10))
					require.Error(t, err, "failed to send eth legacy tx")
					require.Contains(t, err.Error(), "replacement transaction underpriced")

					// Legacy tx cannot replace dynamic fee tx.
					ctx.SetExpQueuedTxs(tx1)
				},
				func(s *TestSuite, ctx *TestContext) {
					signer := s.Acc(0)
					txHash, err := s.SendEthLegacyTx(t, s.Node(0), signer.ID, 0, s.GasPriceMultiplier(10))
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpPendingTxs(txHash)
					ctx.PromoteExpTxs(1)
				},
			},
		},
	}

	s := NewTestSuite(base)
	s.SetupTest(t)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
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
