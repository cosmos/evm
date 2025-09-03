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

func TestNonceGappedTransaction(t *testing.T) {
	testCases := []struct {
		name     string
		malleate func(s *suite.SystemTestSuite, transferFunc types.TransferFunc) (expQueuedTxHashes, expPendingTxHashes []string)
		verify   func(s *suite.SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []string, transferFunc types.TransferFunc)
		bypass   bool
	}{

		{
			name: "Multiple nonce gap fill %s",
			malleate: func(s *suite.SystemTestSuite, transferFunc types.TransferFunc) (expQueuedTxHashes, expPendingTxHashes []string) {
				nonces, err := s.FutureNonces("node0", "acc0", 1)
				require.NoError(t, err)

				lowFeeEVMTxHash, err := transferFunc("node0", "acc0", nonces[1], s.BaseFee, nil)
				require.NoError(t, err)

				highGasEVMTxHash, err := transferFunc("node0", "acc0", nonces[1], s.BaseFeeX2, big.NewInt(1))
				require.NoError(t, err)

				expQueuedTxHashes = []string{highGasEVMTxHash, lowFeeEVMTxHash}
				expPendingTxHashes = []string{}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(s *suite.SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []string, transferFunc types.TransferFunc) {
				// send nonce-gap-filling tx
				nonces, err := s.FutureNonces("node0", "acc0", 0)
				require.NoError(t, err)

				txHash, err := transferFunc("node0", "acc0", nonces[0], s.BaseFee, nil)
				require.NoError(t, err)

				expPendingTxHashes = []string{txHash, expQueuedTxHashes[0]}

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
