package systemtests

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"cosmossdk.io/systemtests"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/tests/systemtests/clients"
	"github.com/evmos/tests/systemtests/config"
	"github.com/stretchr/testify/require"
)

type TransferFunc func(
	ethClient *clients.EthClient,
	nodeID string,
	accID string,
	nonce uint64,
	gasPrice *big.Int,
) (common.Hash, error)

type TxTypeOption struct {
	TxType       string
	TransferFunc TransferFunc
}

func TestTransactionOrdering(t *testing.T) {
	testCases := []struct {
		name     string
		malleate func(ethClient *clients.EthClient, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash)
		verify   func(ethClient *clients.EthClient, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc)
		bypass   bool
	}{
		{
			name: "Basic ordering of pending txs (EVM-only %s)",
			malleate: func(ethClient *clients.EthClient, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				// txHashes = make([]common.Hash, 0)
				expPendingTxHashes = make([]common.Hash, 5)
				for i := 0; i < 5; i++ {
					// nonce order of submitted txs: 3,4,0,1,2
					nonce := (i + 3) % 5
					txHash, err := transferFunc(ethClient, "node0", "acc0", uint64(nonce), big.NewInt(2000000000))
					require.NoError(t, err)

					// nonce order of committed txs: 0,1,2,3,4
					expPendingTxHashes[i] = txHash
				}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(ethClient *clients.EthClient, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc) {
				for _, expSuccessTxHash := range expPendingTxHashes {
					receipt, err := ethClient.WaitForTransaction("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
					require.True(t, receipt.Status == uint64(1))
				}
			},
		},
	}

	txTypeOptions := []TxTypeOption{
		{
			TxType:       "LegacyTx",
			TransferFunc: TransferLegacyTx,
		},
		{
			TxType:       "DynamicFeeTx",
			TransferFunc: TransferDynamicFeeTx,
		},
	}

	for _, tto := range txTypeOptions {
		for _, tc := range testCases {
			tc.name = fmt.Sprintf(tc.name, tto.TxType)
			t.Run(tc.name, func(t *testing.T) {
				if tc.bypass {
					return
				}

				sut := systemtests.Sut
				sut.ResetChain(t)

				StartChain(t, sut)
				sut.AwaitNBlocks(t, 10)

				config, err := config.NewConfig()
				require.NoError(t, err)

				ethClient, err := clients.NewEthClient(config)
				require.NoError(t, err)

				expQueuedTxHashes, expPendingTxHashes := tc.malleate(ethClient, tto.TransferFunc)
				tc.verify(ethClient, expQueuedTxHashes, expPendingTxHashes, tto.TransferFunc)
			})
		}
	}
}

func TestTransactionReplacement(t *testing.T) {
	testCases := []struct {
		name     string
		malleate func(ethClient *clients.EthClient, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash)
		verify   func(ethClient *clients.EthClient, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc)
		bypass   bool
	}{
		{
			name: "Replacement of single pending tx (EVM-only, %s)",
			malleate: func(ethClient *clients.EthClient, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				lowFeeEVMTxHash, err := transferFunc(ethClient, "node0", "acc0", 0, big.NewInt(2000000000))
				require.NoError(t, err)

				highGasEVMTxHash, err := transferFunc(ethClient, "node0", "acc0", 0, big.NewInt(5000000000))
				require.NoError(t, err)

				expQueuedTxHashes = []common.Hash{highGasEVMTxHash, lowFeeEVMTxHash}
				expPendingTxHashes = []common.Hash{}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(ethClient *clients.EthClient, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc) {
				for _, expSuccessTxHash := range expPendingTxHashes {
					receipt, err := ethClient.WaitForTransaction("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
					require.True(t, receipt.Status == uint64(1))
				}
			},
			bypass: true,
		},
		{
			name: "Replacement of multiple pending txs (EVM-only %s)",
			malleate: func(ethClient *clients.EthClient, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				_, err := transferFunc(ethClient, "node0", "acc0", 0, big.NewInt(2000000000))
				require.NoError(t, err)

				_, err = transferFunc(ethClient, "node0", "acc0", 0, big.NewInt(2000000000))
				require.NoError(t, err)

				tx3, err := transferFunc(ethClient, "node0", "acc0", 0, big.NewInt(5000000000))
				require.NoError(t, err)

				tx4, err := transferFunc(ethClient, "node0", "acc0", 0, big.NewInt(5000000000))
				require.NoError(t, err)

				expQueuedTxHashes = []common.Hash{}
				expPendingTxHashes = []common.Hash{tx3, tx4}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(ethClient *clients.EthClient, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc) {
				for _, expSuccessTxHash := range expPendingTxHashes {
					receipt, err := ethClient.WaitForTransaction("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
					require.True(t, receipt.Status == uint64(1))
				}
			},
			bypass: true,
		},
		{
			name: "Replacement of single queued tx (EVM-only %s)",
			malleate: func(ethClient *clients.EthClient, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				_, err := transferFunc(ethClient, "node0", "acc0", 1, big.NewInt(2000000000))
				require.NoError(t, err)

				tx2, err := transferFunc(ethClient, "node0", "acc0", 1, big.NewInt(5000000000))
				require.NoError(t, err)

				expQueuedTxHashes = []common.Hash{tx2}
				expPendingTxHashes = []common.Hash{}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(ethClient *clients.EthClient, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc) {
				// send nonce-gap-filling tx
				txHash, err := transferFunc(ethClient, "node0", "acc0", 0, big.NewInt(2000000000))
				require.NoError(t, err)

				expPendingTxHashes = []common.Hash{txHash, expQueuedTxHashes[0]}

				for _, expSuccessTxHash := range expPendingTxHashes {
					receipt, err := ethClient.WaitForTransaction("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
					require.True(t, receipt.Status == uint64(1))
				}
			},
		},
		{
			name: "Replacement of multiple queued txs (EVM-only %s)",
			malleate: func(ethClient *clients.EthClient, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				_, err := transferFunc(ethClient, "node0", "acc0", 1, big.NewInt(2000000000))
				require.NoError(t, err)

				tx2, err := transferFunc(ethClient, "node0", "acc0", 1, big.NewInt(5000000000))
				require.NoError(t, err)

				_, err = transferFunc(ethClient, "node0", "acc0", 2, big.NewInt(2000000000))
				require.NoError(t, err)

				tx4, err := transferFunc(ethClient, "node0", "acc0", 2, big.NewInt(5000000000))
				require.NoError(t, err)

				expQueuedTxHashes = []common.Hash{tx2, tx4}
				expPendingTxHashes = []common.Hash{}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(ethClient *clients.EthClient, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc) {
				// send nonce-gap-filling tx
				txHash, err := transferFunc(ethClient, "node0", "acc0", 0, big.NewInt(2000000000))
				require.NoError(t, err)

				expPendingTxHashes = []common.Hash{txHash, expQueuedTxHashes[0], expQueuedTxHashes[1]}

				for _, expSuccessTxHash := range expPendingTxHashes {
					receipt, err := ethClient.WaitForTransaction("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
					require.True(t, receipt.Status == uint64(1))
				}
			},
		},
	}

	txTypeOptions := []TxTypeOption{
		{
			TxType:       "LegacyTx",
			TransferFunc: TransferLegacyTx,
		},
		{
			TxType:       "DynamicFeeTx",
			TransferFunc: TransferDynamicFeeTx,
		},
	}

	for _, tto := range txTypeOptions {
		for _, tc := range testCases {
			tc.name = fmt.Sprintf(tc.name, tto.TxType)
			t.Run(tc.name, func(t *testing.T) {
				if tc.bypass {
					return
				}

				sut := systemtests.Sut
				sut.ResetChain(t)

				StartChain(t, sut)
				sut.AwaitNBlocks(t, 10)

				config, err := config.NewConfig()
				require.NoError(t, err)

				ethClient, err := clients.NewEthClient(config)
				require.NoError(t, err)

				expQueuedTxHashes, expPendingTxHashes := tc.malleate(ethClient, tto.TransferFunc)
				tc.verify(ethClient, expQueuedTxHashes, expPendingTxHashes, tto.TransferFunc)
			})
		}
	}
}

func TestNonceGappedTransaction(t *testing.T) {
	testCases := []struct {
		name     string
		malleate func(ethClient *clients.EthClient, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash)
		verify   func(ethClient *clients.EthClient, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc)
		bypass   bool
	}{

		{
			name: "Multiple nonce gap fill (EVM-only %s)",
			malleate: func(ethClient *clients.EthClient, transferFunc TransferFunc) (expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				lowFeeEVMTxHash, err := transferFunc(ethClient, "node0", "acc0", 1, big.NewInt(2000000000))
				require.NoError(t, err)

				highGasEVMTxHash, err := transferFunc(ethClient, "node0", "acc0", 1, big.NewInt(5000000000))
				require.NoError(t, err)

				expQueuedTxHashes = []common.Hash{highGasEVMTxHash, lowFeeEVMTxHash}
				expPendingTxHashes = []common.Hash{}

				return expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(ethClient *clients.EthClient, expQueuedTxHashes, expPendingTxHashes []common.Hash, transferFunc TransferFunc) {
				// send nonce-gap-filling tx
				txHash, err := transferFunc(ethClient, "node0", "acc0", 0, big.NewInt(2000000000))
				require.NoError(t, err)

				expPendingTxHashes = []common.Hash{txHash, expQueuedTxHashes[0]}

				for _, expSuccessTxHash := range expPendingTxHashes {
					receipt, err := ethClient.WaitForTransaction("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
					require.True(t, receipt.Status == uint64(1))
				}
			},
		},
	}

	txTypeOptions := []TxTypeOption{
		{
			TxType:       "LegacyTx",
			TransferFunc: TransferLegacyTx,
		},
		{
			TxType:       "DynamicFeeTx",
			TransferFunc: TransferDynamicFeeTx,
		},
	}

	for _, tto := range txTypeOptions {
		for _, tc := range testCases {
			tc.name = fmt.Sprintf(tc.name, tto.TxType)
			t.Run(tc.name, func(t *testing.T) {
				if tc.bypass {
					return
				}

				sut := systemtests.Sut
				sut.ResetChain(t)

				StartChain(t, sut)
				sut.AwaitNBlocks(t, 10)

				config, err := config.NewConfig()
				require.NoError(t, err)

				ethClient, err := clients.NewEthClient(config)
				require.NoError(t, err)

				expQueuedTxHashes, expPendingTxHashes := tc.malleate(ethClient, tto.TransferFunc)
				tc.verify(ethClient, expQueuedTxHashes, expPendingTxHashes, tto.TransferFunc)
			})
		}
	}
}
