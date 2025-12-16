//go:build system_test

package mempool

import (
	"fmt"
	"testing"

	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/test-go/testify/require"
)

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
