package mempool

import (
	"fmt"
	"testing"

	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/test-go/testify/require"
)

// TestCosmosTxsCompatibility tests that cosmos txs are still functional and interacting with the mempool properly.
func TestCosmosTxsCompatibility(t *testing.T) {
	testCases := []struct {
		name    string
		actions []func(s TestSuite)
	}{
		{
			name: "single pending tx submitted to same nodes %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					tx1, err := s.SendTx(t, s.Node(0), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					s.SetExpPendingTxs(tx1)
				},
			},
		},
		{
			name: "multiple pending txs submitted to same nodes %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					tx1, err := s.SendTx(t, s.Node(0), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					tx2, err := s.SendTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					tx3, err := s.SendTx(t, s.Node(0), "acc0", 2, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx1, tx2, tx3)
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

	s := suite.NewSystemTestSuite(t)
	s.SetupTest(t)

	for _, to := range testOptions {
		s.SetOptions(to)
		for _, tc := range testCases {
			testName := fmt.Sprintf(tc.name, to.Description)
			t.Run(testName, func(t *testing.T) {
				s.BeforeEachCase(t)
				for _, action := range tc.actions {
					action(s)
				}
				s.AfterEachCase(t)
			})
		}
	}
}
