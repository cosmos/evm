package systemtests

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestTransactionOrdering(t *testing.T) {
	testCases := []struct {
		name     string
		malleate func(s *SystemTestSuite, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash)
		verify   func(s *SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc)
		bypass   bool
	}{
		{
			name: "Basic ordering of pending txs (EVM-only %s)",
			malleate: func(s *SystemTestSuite, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				nonces, err := FutureNonces(s.EthClient, "node0", "acc0", 5)
				require.NoError(t, err)

				expPendingTxHashes = make([]common.Hash, 5)
				for i := 0; i < 5; i++ {
					// nonce order of submitted txs: 3,4,0,1,2
					nonce := nonces[(i+3)%5]
					txHash, err := transferFunc(s.EthClient, "node0", "acc0", uint64(nonce), s.BaseFee, nil)
					require.NoError(t, err)

					// nonce order of committed txs: 0,1,2,3,4
					expPendingTxHashes[i] = txHash
				}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(s *SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc) {
				for _, expSuccessTxHash := range expPendingTxHashes {
					receipt, err := s.EthClient.WaitForTransaction("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
					require.True(t, receipt.Status == uint64(1))
				}
			},
		},
	}

	txTypeOptions := []TestOptions{
		{
			TxType:       "LegacyTx",
			TransferFunc: TransferLegacyTx,
		},
		{
			TxType:       "DynamicFeeTx",
			TransferFunc: TransferDynamicFeeTx,
		},
	}

	s := NewSystemTestSuite(t)
	s.SetupTest(t)

	for _, tto := range txTypeOptions {
		for _, tc := range testCases {
			tc.name = fmt.Sprintf(tc.name, tto.TxType)
			t.Run(tc.name, func(t *testing.T) {
				if tc.bypass {
					return
				}

				s.BeforeEach(t)

				expQueuedTxHashes, expPendingTxHashes := tc.malleate(s, tto.TransferFunc)
				tc.verify(s, expQueuedTxHashes, expPendingTxHashes, tto.TransferFunc)
			})
		}
	}
}

func TestTransactionReplacement(t *testing.T) {
	testCases := []struct {
		name     string
		malleate func(s *SystemTestSuite, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash)
		verify   func(s *SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc)
		bypass   bool
	}{
		{
			name: "Replacement of single pending tx (EVM-only, %s)",
			malleate: func(s *SystemTestSuite, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				nonces, err := FutureNonces(s.EthClient, "node0", "acc0", 0)
				require.NoError(t, err)

				lowFeeEVMTxHash, err := transferFunc(s.EthClient, "node0", "acc0", nonces[0], s.BaseFee, nil)
				require.NoError(t, err)

				highGasEVMTxHash, err := transferFunc(s.EthClient, "node0", "acc0", nonces[0], s.BaseFeeX2, big.NewInt(1))
				require.NoError(t, err)

				expQueuedTxHashes = []common.Hash{highGasEVMTxHash, lowFeeEVMTxHash}
				expPendingTxHashes = []common.Hash{}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(s *SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc) {
				for _, expSuccessTxHash := range expPendingTxHashes {
					receipt, err := s.EthClient.WaitForTransaction("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
					require.True(t, receipt.Status == uint64(1))
				}
			},
			bypass: true,
		},
		{
			name: "Replacement of multiple pending txs (EVM-only %s)",
			malleate: func(s *SystemTestSuite, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				nonces, err := FutureNonces(s.EthClient, "node0", "acc0", 1)
				require.NoError(t, err)

				_, err = transferFunc(s.EthClient, "node0", "acc0", nonces[0], s.BaseFee, nil)
				require.NoError(t, err)

				_, err = transferFunc(s.EthClient, "node0", "acc0", nonces[0], s.BaseFee, nil)
				require.NoError(t, err)

				tx3, err := transferFunc(s.EthClient, "node0", "acc0", nonces[1], s.BaseFeeX2, big.NewInt(1))
				require.NoError(t, err)

				tx4, err := transferFunc(s.EthClient, "node0", "acc0", nonces[1], s.BaseFeeX2, big.NewInt(1))
				require.NoError(t, err)

				expQueuedTxHashes = []common.Hash{}
				expPendingTxHashes = []common.Hash{tx3, tx4}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(s *SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc) {
				for _, expSuccessTxHash := range expPendingTxHashes {
					receipt, err := s.EthClient.WaitForTransaction("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
					require.True(t, receipt.Status == uint64(1))
				}
			},
			bypass: true,
		},
		{
			name: "Replacement of single queued tx (EVM-only %s)",
			malleate: func(s *SystemTestSuite, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				nonces, err := FutureNonces(s.EthClient, "node0", "acc0", 1)
				require.NoError(t, err)

				_, err = transferFunc(s.EthClient, "node0", "acc0", nonces[1], s.BaseFee, nil)
				require.NoError(t, err)

				tx2, err := transferFunc(s.EthClient, "node0", "acc0", nonces[1], s.BaseFeeX2, big.NewInt(1))
				require.NoError(t, err)

				expQueuedTxHashes = []common.Hash{tx2}
				expPendingTxHashes = []common.Hash{}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(s *SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc) {
				// send nonce-gap-filling tx
				nonces, err := FutureNonces(s.EthClient, "node0", "acc0", 0)
				require.NoError(t, err)

				txHash, err := transferFunc(s.EthClient, "node0", "acc0", nonces[0], s.BaseFee, nil)
				require.NoError(t, err)

				expPendingTxHashes = []common.Hash{txHash, expQueuedTxHashes[0]}

				for _, expSuccessTxHash := range expPendingTxHashes {
					receipt, err := s.EthClient.WaitForTransaction("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
					require.True(t, receipt.Status == uint64(1))
				}
			},
		},
		{
			name: "Replacement of multiple queued txs (EVM-only %s)",
			malleate: func(s *SystemTestSuite, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				nonces, err := FutureNonces(s.EthClient, "node0", "acc0", 2)
				require.NoError(t, err)

				_, err = transferFunc(s.EthClient, "node0", "acc0", nonces[1], s.BaseFee, nil)
				require.NoError(t, err)

				tx2, err := transferFunc(s.EthClient, "node0", "acc0", nonces[1], s.BaseFeeX2, big.NewInt(1))
				require.NoError(t, err)

				_, err = transferFunc(s.EthClient, "node0", "acc0", nonces[2], s.BaseFee, nil)
				require.NoError(t, err)

				tx4, err := transferFunc(s.EthClient, "node0", "acc0", nonces[2], s.BaseFeeX2, big.NewInt(1))
				require.NoError(t, err)

				expQueuedTxHashes = []common.Hash{tx2, tx4}
				expPendingTxHashes = []common.Hash{}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(s *SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc) {
				// send nonce-gap-filling tx
				nonces, err := FutureNonces(s.EthClient, "node0", "acc0", 0)
				require.NoError(t, err)

				txHash, err := transferFunc(s.EthClient, "node0", "acc0", nonces[0], s.BaseFee, nil)
				require.NoError(t, err)

				expPendingTxHashes = []common.Hash{txHash, expQueuedTxHashes[0], expQueuedTxHashes[1]}

				for _, expSuccessTxHash := range expPendingTxHashes {
					receipt, err := s.EthClient.WaitForTransaction("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
					require.True(t, receipt.Status == uint64(1))
				}
			},
		},
	}

	txTypeOptions := []TestOptions{
		{
			TxType:       "LegacyTx",
			TransferFunc: TransferLegacyTx,
		},
		{
			TxType:       "DynamicFeeTx",
			TransferFunc: TransferDynamicFeeTx,
		},
	}

	s := NewSystemTestSuite(t)
	s.SetupTest(t)

	for _, tto := range txTypeOptions {
		for _, tc := range testCases {
			tc.name = fmt.Sprintf(tc.name, tto.TxType)
			t.Run(tc.name, func(t *testing.T) {
				if tc.bypass {
					return
				}

				s.BeforeEach(t)

				expQueuedTxHashes, expPendingTxHashes := tc.malleate(s, tto.TransferFunc)
				tc.verify(s, expQueuedTxHashes, expPendingTxHashes, tto.TransferFunc)
			})
		}
	}
}

func TestNonceGappedTransaction(t *testing.T) {
	testCases := []struct {
		name     string
		malleate func(s *SystemTestSuite, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash)
		verify   func(s *SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc)
		bypass   bool
	}{

		{
			name: "Multiple nonce gap fill (EVM-only %s)",
			malleate: func(s *SystemTestSuite, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				nonces, err := FutureNonces(s.EthClient, "node0", "acc0", 1)
				require.NoError(t, err)

				lowFeeEVMTxHash, err := transferFunc(s.EthClient, "node0", "acc0", nonces[1], s.BaseFee, nil)
				require.NoError(t, err)

				highGasEVMTxHash, err := transferFunc(s.EthClient, "node0", "acc0", nonces[1], s.BaseFeeX2, big.NewInt(1))
				require.NoError(t, err)

				expQueuedTxHashes = []common.Hash{highGasEVMTxHash, lowFeeEVMTxHash}
				expPendingTxHashes = []common.Hash{}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(s *SystemTestSuite, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc) {
				// send nonce-gap-filling tx
				nonces, err := FutureNonces(s.EthClient, "node0", "acc0", 0)
				require.NoError(t, err)

				txHash, err := transferFunc(s.EthClient, "node0", "acc0", nonces[0], s.BaseFee, nil)
				require.NoError(t, err)

				expPendingTxHashes = []common.Hash{txHash, expQueuedTxHashes[0]}

				for _, expSuccessTxHash := range expPendingTxHashes {
					receipt, err := s.EthClient.WaitForTransaction("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
					require.True(t, receipt.Status == uint64(1))
				}
			},
		},
	}

	txTypeOptions := []TestOptions{
		{
			TxType:       "LegacyTx",
			TransferFunc: TransferLegacyTx,
		},
		{
			TxType:       "DynamicFeeTx",
			TransferFunc: TransferDynamicFeeTx,
		},
	}

	s := NewSystemTestSuite(t)
	s.SetupTest(t)

	for _, tto := range txTypeOptions {
		for _, tc := range testCases {
			tc.name = fmt.Sprintf(tc.name, tto.TxType)
			t.Run(tc.name, func(t *testing.T) {
				if tc.bypass {
					return
				}

				s.BeforeEach(t)

				expQueuedTxHashes, expPendingTxHashes := tc.malleate(s, tto.TransferFunc)
				tc.verify(s, expQueuedTxHashes, expPendingTxHashes, tto.TransferFunc)
			})
		}
	}
}
