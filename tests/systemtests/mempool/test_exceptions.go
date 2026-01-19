package mempool

import (
	"fmt"
	"testing"

	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/test-go/testify/require"
)

func RunMinimumGasPricesZero(t *testing.T) {
	testCases := []struct {
		name    string
		actions []func(s TestSuite)
	}{
		{
			name: "sequencial pending txs %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					tx1, err := s.SendTx(t, s.Node(0), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					tx2, err := s.SendTx(t, s.Node(1), "acc0", 1, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					tx3, err := s.SendTx(t, s.Node(2), "acc0", 2, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx1, tx2, tx3)
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

	s := NewTestSuite(t)
	ctx := NewTestContext()
	s.SetupTest(t, suite.MinimumGasPriceZeroArgs()...)

	for _, to := range testOptions {
		s.SetOptions(to)
		for _, tc := range testCases {
			testName := fmt.Sprintf(tc.name, to.Description)
			t.Run(testName, func(t *testing.T) {
				s.BeforeEachCase(t, ctx)
				for _, action := range tc.actions {
					action(s)
					s.AfterEachAction(t, ctx)
				}
				s.AfterEachCase(t, ctx)
			})
		}
	}
}
