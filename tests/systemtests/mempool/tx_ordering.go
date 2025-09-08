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
			name: "Basic ordering of pending txs %s",
			actions: []func(s suite.TestSuite){
				func(s suite.TestSuite) {
					expPendingTxs := make([]*suite.TxInfo, 5)
					for i := 0; i < 5; i++ {
						// nonce order of submitted txs: 3,4,0,1,2
						nonceIdx := uint64((i + 3) % 5)
						txInfo, err := s.SendTx(t, s.GetNode(), "acc0", nonceIdx, s.BaseFee(), new(big.Int).Mul(big.NewInt(1000), big.NewInt(int64(i))))
						require.NoError(t, err, "failed to send tx")

						// nonce order of committed txs: 0,1,2,3,4
						expPendingTxs[(i+3)%5] = txInfo
					}

					s.SetExpPendingTxs(expPendingTxs...)
				},
			},
		},
	}

	s := suite.NewSystemTestSuite(t)
	s.SetupTest(t)

	for _, to := range s.DefaultTestOption() {
		for _, tc := range testCases {
			s.TestOption = to
			tc.name = fmt.Sprintf(tc.name, to.TestType)
			t.Run(tc.name, func(t *testing.T) {
				if tc.bypass {
					return
				}

				s.BeforeEach(t)
				for _, action := range tc.actions {
					action(s)
					// s.JustAfterEach(t)
				}
				s.AfterEach(t)
			})
		}
	}
}
