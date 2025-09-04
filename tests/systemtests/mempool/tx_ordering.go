package mempool

import (
	"fmt"
	"testing"

	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/stretchr/testify/require"
)

func TestTransactionOrdering(t *testing.T) {
	testCases := []struct {
		name       string
		malleate   func(s TestSuite)
		postAction func(s TestSuite)
		bypass     bool
	}{
		{
			name: "Basic ordering of pending txs %s",
			malleate: func(s TestSuite) {
				expPendingTxs := make([]string, 5)
				for i := 0; i < 5; i++ {
					// nonce order of submitted txs: 3,4,0,1,2
					nonceIdx := uint64((i + 3) % 5)
					txHash, err := s.SendTx(s.GetNode(), "acc0", nonceIdx, s.BaseFee(), nil)
					require.NoError(t, err)

					// nonce order of committed txs: 0,1,2,3,4
					expPendingTxs[i] = txHash
				}

				if s.OnlyEthTxs() {
					s.SetExpQueuedTxs(expPendingTxs...)
				}
			},
			postAction: func(s TestSuite) {},
		},
	}

	s := suite.NewSystemTestSuite(t)
	s.SetupTest(t)

	for _, to := range s.DefaultTestOption() {
		for _, tc := range testCases {
			tc.name = fmt.Sprintf(tc.name, to.TestType)
			t.Run(tc.name, func(t *testing.T) {
				if tc.bypass {
					return
				}

				s.BeforeEach(t)

				tc.malleate(s)
				tc.postAction(s)

				s.AfterEach(t)
			})
		}
	}
}
