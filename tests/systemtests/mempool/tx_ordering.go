package mempool

import (
	"fmt"
	"testing"
	"time"

	"github.com/cosmos/evm/tests/systemtests/suite"
	types "github.com/cosmos/evm/tests/systemtests/types"
	"github.com/stretchr/testify/require"
)

func TestTransactionOrdering(t *testing.T) {
	testCases := []struct {
		name     string
		malleate func(s *suite.SystemTestSuite, option types.TestOption) (expQueuedTxHashes, expPendingTxHashes []string)
		verify   func(s *suite.SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []string, option types.TestOption)
		bypass   bool
	}{
		{
			name: "Basic ordering of pending txs %s",
			malleate: func(s *suite.SystemTestSuite, option types.TestOption) (expQueuedTxHashes, expPendingTxHashes []string) {
				nonces, err := s.FutureNonces("node0", "acc0", 5)
				require.NoError(t, err)

				expPendingTxHashes = make([]string, 5)
				for i := 0; i < 5; i++ {
					// nonce order of submitted txs: 3,4,0,1,2
					nonce := nonces[(i+3)%5]
					txHash, err := option.Transfer("node0", "acc0", uint64(nonce), s.BaseFee, nil)
					require.NoError(t, err)

					// nonce order of committed txs: 0,1,2,3,4
					expPendingTxHashes[i] = txHash
				}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(s *suite.SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []string, option types.TestOption) {
				for _, expSuccessTxHash := range expPendingTxHashes {
					err := option.WaitForCommit("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
				}
			},
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

				expQueuedTxHashes, expPendingTxHashes := tc.malleate(s, to)
				tc.verify(s, expQueuedTxHashes, expPendingTxHashes, to)
			})
		}
	}
}
