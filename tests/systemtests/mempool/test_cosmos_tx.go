//go:build system_test

package mempool

import (
	"fmt"
	"testing"

	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/test-go/testify/require"
)

// RunCosmosTxsCompatibility tests that cosmos txs are still functional and interacting with the mempool properly.
func RunCosmosTxsCompatibility(t *testing.T, base *suite.BaseTestSuite) {
	testCases := []struct {
		name    string
		actions []func(*TestSuite, *TestContext)
	}{
		{
			name: "single pending tx submitted to same nodes %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					tx1, err := s.SendTx(t, s.Node(0), "acc0", 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")
					ctx.SetExpPendingTxs(tx1)
				},
			},
		},
		{
			name: "multiple pending txs submitted to same nodes %s",
			actions: []func(*TestSuite, *TestContext){
				func(s *TestSuite, ctx *TestContext) {
					tx1, err := s.SendTx(t, s.Node(0), "acc0", 0, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					tx2, err := s.SendTx(t, s.Node(0), "acc0", 1, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					tx3, err := s.SendTx(t, s.Node(0), "acc0", 2, s.GasPriceMultiplier(10), nil)
					require.NoError(t, err, "failed to send tx")

					ctx.SetExpPendingTxs(tx1, tx2, tx3)
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
				}
				s.AfterEachCase(t, ctx)
			})
		}
	}
}
