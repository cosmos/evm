package mempool

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/cosmos/evm/tests/systemtests/suite"
	types "github.com/cosmos/evm/tests/systemtests/types"
	"github.com/stretchr/testify/require"
)

func TestTransactionReplacement(t *testing.T) {
	testCases := []struct {
		name     string
		malleate func(s *suite.SystemTestSuite, option types.TestOption) (expQueuedTxHashes, expPendingTxHashes []string)
		verify   func(s *suite.SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []string, option types.TestOption)
		bypass   bool
	}{
		{
			name: "Replacement of single pending tx %s",
			malleate: func(s *suite.SystemTestSuite, option types.TestOption) (expQueuedTxHashes, expPendingTxHashes []string) {
				nonces, err := s.FutureNonces("node0", "acc0", 0)
				require.NoError(t, err)

				lowFeeEVMTxHash, err := option.Transfer("node0", "acc0", nonces[0], s.BaseFee, nil)
				require.NoError(t, err)

				highGasEVMTxHash, err := option.Transfer("node0", "acc0", nonces[0], s.BaseFeeX2, big.NewInt(1))
				require.NoError(t, err)

				expQueuedTxHashes = []string{highGasEVMTxHash, lowFeeEVMTxHash}
				expPendingTxHashes = []string{}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(s *suite.SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []string, option types.TestOption) {
				for _, expSuccessTxHash := range expPendingTxHashes {
					err := option.WaitForCommit("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
				}
			},
			bypass: true,
		},
		{
			name: "Replacement of multiple pending txs %s",
			malleate: func(s *suite.SystemTestSuite, option types.TestOption) (expQueuedTxHashes, expPendingTxHashes []string) {
				nonces, err := s.FutureNonces("node0", "acc0", 1)
				require.NoError(t, err)

				_, err = option.Transfer("node0", "acc0", nonces[0], s.BaseFee, nil)
				require.NoError(t, err)

				_, err = option.Transfer("node0", "acc0", nonces[0], s.BaseFee, nil)
				require.NoError(t, err)

				tx3, err := option.Transfer("node0", "acc0", nonces[1], s.BaseFeeX2, big.NewInt(1))
				require.NoError(t, err)

				tx4, err := option.Transfer("node0", "acc0", nonces[1], s.BaseFeeX2, big.NewInt(1))
				require.NoError(t, err)

				expQueuedTxHashes = []string{}
				expPendingTxHashes = []string{tx3, tx4}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(s *suite.SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []string, option types.TestOption) {
				for _, expSuccessTxHash := range expPendingTxHashes {
					err := option.WaitForCommit("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
				}
			},
			bypass: true,
		},
		{
			name: "Replacement of single queued tx %s",
			malleate: func(s *suite.SystemTestSuite, option types.TestOption) (expQueuedTxHashes, expPendingTxHashes []string) {
				nonces, err := s.FutureNonces("node0", "acc0", 1)
				require.NoError(t, err)

				_, err = option.Transfer("node0", "acc0", nonces[1], s.BaseFee, nil)
				require.NoError(t, err)

				tx2, err := option.Transfer("node0", "acc0", nonces[1], s.BaseFeeX2, big.NewInt(1))
				require.NoError(t, err)

				expQueuedTxHashes = []string{tx2}
				expPendingTxHashes = []string{}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(s *suite.SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []string, option types.TestOption) {
				// send nonce-gap-filling tx
				nonces, err := s.FutureNonces("node0", "acc0", 0)
				require.NoError(t, err)

				txHash, err := option.Transfer("node0", "acc0", nonces[0], s.BaseFee, nil)
				require.NoError(t, err)

				expPendingTxHashes = []string{txHash, expQueuedTxHashes[0]}

				for _, expSuccessTxHash := range expPendingTxHashes {
					err := option.WaitForCommit("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
				}
			},
		},
		{
			name: "Replacement of multiple queued txs %s",
			malleate: func(s *suite.SystemTestSuite, option types.TestOption) (expQueuedTxHashes, expPendingTxHashes []string) {
				nonces, err := s.FutureNonces("node0", "acc0", 2)
				require.NoError(t, err)

				_, err = option.Transfer("node0", "acc0", nonces[1], s.BaseFee, nil)
				require.NoError(t, err)

				tx2, err := option.Transfer("node0", "acc0", nonces[1], s.BaseFeeX2, big.NewInt(1))
				require.NoError(t, err)

				_, err = option.Transfer("node0", "acc0", nonces[2], s.BaseFee, nil)
				require.NoError(t, err)

				tx4, err := option.Transfer("node0", "acc0", nonces[2], s.BaseFeeX2, big.NewInt(1))
				require.NoError(t, err)

				expQueuedTxHashes = []string{tx2, tx4}
				expPendingTxHashes = []string{}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(s *suite.SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []string, option types.TestOption) {
				// send nonce-gap-filling tx
				nonces, err := s.FutureNonces("node0", "acc0", 0)
				require.NoError(t, err)

				txHash, err := option.Transfer("node0", "acc0", nonces[0], s.BaseFee, nil)
				require.NoError(t, err)

				expPendingTxHashes = []string{txHash, expQueuedTxHashes[0], expQueuedTxHashes[1]}

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
