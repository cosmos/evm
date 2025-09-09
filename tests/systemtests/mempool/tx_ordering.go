//go:build system_test

package mempool

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/test-go/testify/require"
)

func TestTxsOrdering(t *testing.T) {
	testCases := []struct {
		name    string
		actions []func(s suite.TestSuite)
		bypass  bool
	}{
		{
			name: "ordering of pending txs %s",
			actions: []func(s suite.TestSuite){
				func(s suite.TestSuite) {
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

						txInfo, err := s.SendTx(t, nodeId, "acc0", nonceIdx, s.BaseFee(), big.NewInt(1))
						require.NoError(t, err, "failed to send tx")

						// nonce order of committed txs: 0,1,2,3,4
						expPendingTxs[nonceIdx] = txInfo
					}

					s.SetExpPendingTxs(expPendingTxs...)
				},
			},
		},
	}

	testOptions := []*suite.TestOptions{
		{
			Description:    "EVM LegacyTx send",
			TxType:         suite.TxTypeEVM,
			IsDynamicFeeTx: false,
		},
		{
			Description:    "EVM DynamicFeeTx send",
			TxType:         suite.TxTypeEVM,
			IsDynamicFeeTx: true,
		},
	}

	s := suite.NewSystemTestSuite(t)
	s.SetupTest(t)

	for _, to := range testOptions {
		s.SetOptions(to)
		for _, tc := range testCases {
			testName := fmt.Sprintf(tc.name, to.Description)
			t.Run(testName, func(t *testing.T) {
				if tc.bypass {
					return
				}

				s.BeforeEach(t)
				for _, action := range tc.actions {
					action(s)
					s.JustAfterEach(t)
				}
				s.AfterEach(t)
			})
		}
	}
}
