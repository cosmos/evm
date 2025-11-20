//go:build system_test

package mempool

import (
	"fmt"
	"testing"

	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/test-go/testify/require"
)

func RunTxRebroadcasting(t *testing.T, base *suite.BaseTestSuite) {
	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		{
			name: "ordering of pending txs %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					signer := s.Acc(0)

					tx1, err := s.SendTx(t, s.Node(0), signer.ID, 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					tx2, err := s.SendTx(t, s.Node(1), signer.ID, 1, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					tx3, err := s.SendTx(t, s.Node(2), signer.ID, 2, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					// Skip tx4 with nonce 3

					tx5, err := s.SendTx(t, s.Node(3), signer.ID, 4, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					tx6, err := s.SendTx(t, s.Node(0), signer.ID, 5, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					// At AfterEachAction hook, we will check expected queued txs are not broadcasted.
					ctx.SetExpPendingTxs(tx1, tx2, tx3)
					ctx.SetExpQueuedTxs(tx5, tx6)
				},
				func(s *TestSuite, ctx *TestContext) {
					// Wait for 3 blocks.
					// It is because tx1, tx2, tx3 are sent to different nodes, tx3 needs maximum 3 blocks to be committed.
					// e.g. node3 is 1st proposer -> tx3 will tale 1 block to be committed.
					// e.g. node3 is 3rd proposer -> tx3 will take 3 blocks to be committed.
					s.AwaitNBlocks(t, 3)

					// current nonce is 3.
					// so, we should set nonce idx to 0.
					nonce3Idx := uint64(0)

					signer := s.Acc(0)

					tx4, err := s.SendTx(t, s.Node(2), signer.ID, nonce3Idx, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					// At AfterEachAction hook, we will check expected pending txs are broadcasted.
					ctx.SetExpPendingTxs(tx4)
					ctx.PromoteExpTxs(2)
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

func RunMinimumGasPricesZero(t *testing.T, base *suite.BaseTestSuite) {
	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		{
			name: "sequencial pending txs %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					signer := s.Acc(0)

					tx1, err := s.SendTx(t, s.Node(0), signer.ID, 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					tx2, err := s.SendTx(t, s.Node(1), signer.ID, 1, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					tx3, err := s.SendTx(t, s.Node(2), signer.ID, 2, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpPendingTxs(tx1, tx2, tx3)
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
	s.SetupTest(t, suite.MinimumGasPriceZeroArgs()...)

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
