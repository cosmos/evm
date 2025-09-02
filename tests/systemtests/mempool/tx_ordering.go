package mempool

import (
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/tests/systemtests/suite"
	types "github.com/evmos/tests/systemtests/types"
	"github.com/stretchr/testify/require"
)

func TestTransactionOrdering(t *testing.T) {
	testCases := []struct {
		name     string
		malleate func(s *suite.SystemTestSuite, transferFunc types.TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash)
		verify   func(s *suite.SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc types.TransferFunc)
		bypass   bool
	}{
		{
			name: "Basic ordering of pending txs %s",
			malleate: func(s *suite.SystemTestSuite, transferFunc types.TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				nonces, err := s.FutureNonces("node0", "acc0", 5)
				require.NoError(t, err)

				expPendingTxHashes = make([]common.Hash, 5)
				for i := 0; i < 5; i++ {
					// nonce order of submitted txs: 3,4,0,1,2
					nonce := nonces[(i+3)%5]
					txHash, err := transferFunc("node0", "acc0", uint64(nonce), s.BaseFee, nil)
					require.NoError(t, err)

					// nonce order of committed txs: 0,1,2,3,4
					expPendingTxHashes[i] = txHash
				}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(s *suite.SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc types.TransferFunc) {
				for _, expSuccessTxHash := range expPendingTxHashes {
					receipt, err := s.EthClient.WaitForTransaction("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
					require.True(t, receipt.Status == uint64(1))
				}
			},
		},
	}

	s := suite.NewSystemTestSuite(t)
	s.SetupTest(t)

	for _, to := range s.DefaultTestOption() {
		for _, tc := range testCases {
			tc.name = fmt.Sprintf(tc.name, to.TxType)
			t.Run(tc.name, func(t *testing.T) {
				if tc.bypass {
					return
				}

				s.BeforeEach(t)

				expQueuedTxHashes, expPendingTxHashes := tc.malleate(s, to.TransferFunc)
				tc.verify(s, expQueuedTxHashes, expPendingTxHashes, to.TransferFunc)
			})
		}
	}
}
