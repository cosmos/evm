package systemtests

import (
	"math/big"
	"testing"
	"time"

	"cosmossdk.io/systemtests"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/tests/systemtests/clients"
	"github.com/evmos/tests/systemtests/config"
	"github.com/stretchr/testify/require"
)

type TxArgs struct {
	pk       string
	nonce    string
	gasPrice string
}

func TestTransactionOrdering(t *testing.T) {
	testCases := []struct {
		name    string
		getArgs func(ethClient *clients.EthClient) (txHashes, expQueuedTxHashes, expPendingTxHashes []common.Hash)
		verify  func(ethClient *clients.EthClient, expQueuedTxHashes, expPendingTxHashes []common.Hash)
	}{
		{
			name: "Conversion of queued to pending by nonce gap fill (EVM-only, LegacyTx)",
			getArgs: func(ethClient *clients.EthClient) (txHashes, expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				lowFeeEVMTxHash, err := ValueTransferLegacyTx(ethClient, "node0", "acc0", 1, big.NewInt(2000000000))
				require.NoError(t, err)

				highGasEVMTxHash, err := ValueTransferLegacyTx(ethClient, "node0", "acc0", 1, big.NewInt(5000000000))
				require.NoError(t, err)

				txHashes = []common.Hash{lowFeeEVMTxHash, highGasEVMTxHash}
				expQueuedTxHashes = []common.Hash{highGasEVMTxHash, lowFeeEVMTxHash}
				expPendingTxHashes = []common.Hash{}

				return txHashes, expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(ethClient *clients.EthClient, expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				txHash, err := ValueTransferDynamicFeeTx(ethClient, "node0", "acc0", 0, big.NewInt(0), big.NewInt(2000000000))
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
			name: "Conversion of queued to pending by nonce gap fill (EVM-only, DynamicFeeTx)",
			getArgs: func(ethClient *clients.EthClient) (txHashes, expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				lowFeeEVMTxHash, err := ValueTransferDynamicFeeTx(ethClient, "node0", "acc0", 1, big.NewInt(0), big.NewInt(2000000000))
				require.NoError(t, err)

				highGasEVMTxHash, err := ValueTransferDynamicFeeTx(ethClient, "node0", "acc0", 1, big.NewInt(100), big.NewInt(5000000000))
				require.NoError(t, err)

				txHashes = []common.Hash{lowFeeEVMTxHash, highGasEVMTxHash}
				expQueuedTxHashes = []common.Hash{highGasEVMTxHash}
				expPendingTxHashes = []common.Hash{highGasEVMTxHash}

				return txHashes, expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(ethClient *clients.EthClient, expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				txHash, err := ValueTransferDynamicFeeTx(ethClient, "node0", "acc0", 0, big.NewInt(0), big.NewInt(2000000000))
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
			name: "Replacement of pending txs (EVM-only, LegacyTx)",
			getArgs: func(ethClient *clients.EthClient) (txHashes, expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				lowFeeEVMTxHash, err := ValueTransferLegacyTx(ethClient, "node0", "acc0", 0, big.NewInt(2000000000))
				require.NoError(t, err)

				highGasEVMTxHash, err := ValueTransferLegacyTx(ethClient, "node0", "acc0", 0, big.NewInt(5000000000))
				require.NoError(t, err)

				txHashes = []common.Hash{lowFeeEVMTxHash, highGasEVMTxHash}
				expQueuedTxHashes = []common.Hash{highGasEVMTxHash, lowFeeEVMTxHash}
				expPendingTxHashes = []common.Hash{}

				return txHashes, expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(ethClient *clients.EthClient, expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				for _, expSuccessTxHash := range expPendingTxHashes {
					receipt, err := ethClient.WaitForTransaction("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
					require.True(t, receipt.Status == uint64(1))
				}
			},
		},
		{
			name: "Replacement of pending txs (EVM-only, DynamicFeeTx)",
			getArgs: func(ethClient *clients.EthClient) (txHashes, expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				lowFeeEVMTxHash, err := ValueTransferDynamicFeeTx(ethClient, "node0", "acc0", 0, big.NewInt(0), big.NewInt(2000000000))
				require.NoError(t, err)

				highGasEVMTxHash, err := ValueTransferDynamicFeeTx(ethClient, "node0", "acc0", 0, big.NewInt(100), big.NewInt(5000000000))
				require.NoError(t, err)

				txHashes = []common.Hash{lowFeeEVMTxHash, highGasEVMTxHash}
				expQueuedTxHashes = []common.Hash{highGasEVMTxHash}
				expPendingTxHashes = []common.Hash{highGasEVMTxHash}

				return txHashes, expQueuedTxHashes, expPendingTxHashes
			},
			verify: func(ethClient *clients.EthClient, expQueuedTxHashes, expPendingTxHashes []common.Hash) {
				for _, expSuccessTxHash := range expPendingTxHashes {
					receipt, err := ethClient.WaitForTransaction("node0", expSuccessTxHash, time.Second*10)
					require.NoError(t, err)
					require.True(t, receipt.Status == uint64(1))
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sut := systemtests.Sut
			sut.ResetChain(t)

			StartChain(t, sut)
			sut.AwaitNBlocks(t, 10)

			config, err := config.NewConfig()
			require.NoError(t, err)

			ethClient, err := clients.NewEthClient(config)
			require.NoError(t, err)

			_, expQueuedTxHashes, expPendingTxHashes := tc.getArgs(ethClient)
			tc.verify(ethClient, expQueuedTxHashes, expPendingTxHashes)
		})
	}
}
