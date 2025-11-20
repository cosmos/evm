package mempool

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/core"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/tmhash"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

// TestTransactionOrderingWithABCIMethodCalls tests transaction ordering based on fees
func (s *IntegrationTestSuite) TestTransactionOrderingWithABCIMethodCalls() {
	testCases := []struct {
		name     string
		setupTxs func() ([]sdk.Tx, []string)
	}{
		{
			name: "mixed EVM and cosmos transaction ordering",
			setupTxs: func() ([]sdk.Tx, []string) {
				// Create EVM transaction with high gas price
				highGasPriceEVMTx := s.createEVMValueTransferTx(s.keyring.GetKey(0), 0, big.NewInt(5000000000))

				// Create Cosmos transactions with different fee amounts
				highFeeCosmosTx := s.createCosmosSendTx(s.keyring.GetKey(6), big.NewInt(5000000000))
				mediumFeeCosmosTx := s.createCosmosSendTx(s.keyring.GetKey(7), big.NewInt(3000000000))
				lowFeeCosmosTx := s.createCosmosSendTx(s.keyring.GetKey(8), big.NewInt(2000000000))

				// Input txs in order
				inputTxs := []sdk.Tx{lowFeeCosmosTx, highGasPriceEVMTx, mediumFeeCosmosTx, highFeeCosmosTx}

				// Expected txs in order
				expectedTxs := []sdk.Tx{highGasPriceEVMTx, highFeeCosmosTx, mediumFeeCosmosTx, lowFeeCosmosTx}
				expTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expTxHashes
			},
		},
		{
			name: "EVM-only transaction replacement",
			setupTxs: func() ([]sdk.Tx, []string) {
				// Create first EVM transaction with low fee
				lowFeeEVMTx := s.createEVMValueTransferTx(s.keyring.GetKey(0), 0, big.NewInt(2000000000)) // 2 gaatom

				// Create second EVM transaction with high fee
				highFeeEVMTx := s.createEVMValueTransferDynamicFeeTx(s.keyring.GetKey(0), 0, big.NewInt(5000000000), big.NewInt(5000000000)) // 5 gaatom

				// Input txs in order
				inputTxs := []sdk.Tx{lowFeeEVMTx, highFeeEVMTx}

				// Expected Txs in order
				expectedTxs := []sdk.Tx{highFeeEVMTx}
				expTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expTxHashes
			},
		},
		{
			name: "EVM-only transaction ordering",
			setupTxs: func() ([]sdk.Tx, []string) {
				key := s.keyring.GetKey(0)
				// Create first EVM transaction with low fee
				lowFeeEVMTx := s.createEVMValueTransferTx(key, 1, big.NewInt(2000000000)) // 2 gaatom

				// Create second EVM transaction with high fee
				highFeeEVMTx := s.createEVMValueTransferTx(key, 0, big.NewInt(5000000000)) // 5 gaatom

				// Input txs in order
				inputTxs := []sdk.Tx{lowFeeEVMTx, highFeeEVMTx}

				// Expected txs in order
				expectedTxs := []sdk.Tx{highFeeEVMTx, lowFeeEVMTx}
				expTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expTxHashes
			},
		},
		{
			name: "mixed EVM and Cosmos transactions with equal effective tips",
			setupTxs: func() ([]sdk.Tx, []string) {
				// Cosmos with same effective tip: 1000 * 200000 = 200000000 aatom total fee
				cosmosTx := s.createCosmosSendTx(s.keyring.GetKey(0), big.NewInt(1000000000)) // 1 gaatom/gas effective tip

				// Create transactions with equal effective tips (assuming base fee = 0)
				// EVM: 1000 aatom/gas effective tip
				evmTx := s.createEVMValueTransferDynamicFeeTx(s.keyring.GetKey(0), 0, big.NewInt(1000000000), big.NewInt(1000000000)) // 1 gaatom/gas

				// Input txs in order
				inputTxs := []sdk.Tx{cosmosTx, evmTx}

				// Expected txs in order
				expectedTxs := []sdk.Tx{evmTx}
				expTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expTxHashes
			},
		},
		{
			name: "mixed transactions with EVM having higher effective tip",
			setupTxs: func() ([]sdk.Tx, []string) {
				// Create Cosmos transaction with lower gas price
				cosmosTx := s.createCosmosSendTx(s.keyring.GetKey(0), big.NewInt(2000000000)) // 2 gaatom/gas

				// Create EVM transaction with higher gas price
				evmTx := s.createEVMValueTransferDynamicFeeTx(s.keyring.GetKey(0), 0, big.NewInt(5000000000), big.NewInt(5000000000)) // 5 gaatom/gas

				// Input txs in order
				inputTxs := []sdk.Tx{cosmosTx, evmTx}

				// Expected txs in order
				expectedTxs := []sdk.Tx{evmTx}
				expTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expTxHashes
			},
		},
		{
			name: "mixed transactions with Cosmos having higher effective tip",
			setupTxs: func() ([]sdk.Tx, []string) {
				// Create EVM transaction with lower gas price
				evmTx := s.createEVMValueTransferTx(s.keyring.GetKey(0), 0, big.NewInt(2000000000)) // 2000 aatom/gas

				// Create Cosmos transaction with higher gas price
				cosmosTx := s.createCosmosSendTx(s.keyring.GetKey(0), big.NewInt(5000000000)) // 5000 aatom/gas

				// Input txs in order
				inputTxs := []sdk.Tx{evmTx, cosmosTx}

				// Expected txs in order
				expectedTxs := []sdk.Tx{evmTx}
				expTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expTxHashes
			},
		},
		{
			name: "mixed transaction ordering with multiple effective tips",
			setupTxs: func() ([]sdk.Tx, []string) {
				// Create multiple transactions with different gas prices
				// EVM: 10000, 8000, 6000 aatom/gas
				// Cosmos: 9000, 7000, 5000 aatom/gas

				evmHigh := s.createEVMValueTransferTx(s.keyring.GetKey(0), 0, big.NewInt(10000000000))
				evmMedium := s.createEVMValueTransferTx(s.keyring.GetKey(1), 0, big.NewInt(8000000000))
				evmLow := s.createEVMValueTransferTx(s.keyring.GetKey(2), 0, big.NewInt(6000000000))

				cosmosHigh := s.createCosmosSendTx(s.keyring.GetKey(3), big.NewInt(9000000000))
				cosmosMedium := s.createCosmosSendTx(s.keyring.GetKey(4), big.NewInt(7000000000))
				cosmosLow := s.createCosmosSendTx(s.keyring.GetKey(5), big.NewInt(5000000000))

				// Input txs in order
				inputTxs := []sdk.Tx{cosmosHigh, cosmosMedium, cosmosLow, evmHigh, evmMedium, evmLow}

				// Expected txs in order
				expectedTxs := []sdk.Tx{evmHigh, cosmosHigh, evmMedium, cosmosMedium, evmLow, cosmosLow}
				expTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expTxHashes
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Clean up previous test's resources before resetting
			s.TearDownTest()
			// Reset test setup to ensure clean state
			s.SetupTest()

			txs, expTxHashes := tc.setupTxs()

			// Call CheckTx for transactions
			err := s.checkTxs(txs)
			s.Require().NoError(err)

			// Call FinalizeBlock to make finalizeState before calling PrepareProposal
			_, err = s.network.FinalizeBlock()
			s.Require().NoError(err)

			// Call PrepareProposal to selcet transactions from mempool and make proposal
			prepareProposalRes, err := s.network.App.PrepareProposal(&abci.RequestPrepareProposal{
				MaxTxBytes: 1_000_000,
				Height:     1,
			})
			s.Require().NoError(err)

			// Check whether expected transactions are included and returned as pending state in mempool
			mpool := s.network.App.GetMempool()
			iterator := mpool.Select(s.network.GetContext(), nil)
			for _, txHash := range expTxHashes {
				actualTxHash := s.getTxHash(iterator.Tx())
				s.Require().Equal(txHash, actualTxHash)

				iterator = iterator.Next()
			}

			// Check whether expected transactions are selcted by PrepareProposal
			txHashes := make([]string, 0)
			for _, txBytes := range prepareProposalRes.Txs {
				txHash := hex.EncodeToString(tmhash.Sum(txBytes))
				txHashes = append(txHashes, txHash)
			}
			s.Require().Equal(expTxHashes, txHashes)
		})
	}
}

// TestNonceGappedEVMTransactionsWithABCIMethodCalls tests the behavior of nonce-gapped EVM transactions
// and the transition from queued to pending when gaps are filled
func (s *IntegrationTestSuite) TestNonceGappedEVMTransactionsWithABCIMethodCalls() {
	testCases := []struct {
		name       string
		setupTxs   func() ([]sdk.Tx, []string) // Returns transactions and their expected nonces
		verifyFunc func(mpool mempool.Mempool)
	}{
		{
			name: "insert transactions with nonce gaps",
			setupTxs: func() ([]sdk.Tx, []string) {
				key := s.keyring.GetKey(0)
				var txs []sdk.Tx

				// Insert transactions with gaps: nonces 0, 2, 4, 6 (missing 1, 3, 5)
				for i := 0; i <= 6; i += 2 {
					tx := s.createEVMValueTransferTx(key, i, big.NewInt(2000000000))
					txs = append(txs, tx)
				}

				// Expected txs in order
				expectedTxs := txs[:1]
				expTxHashes := s.getTxHashes(expectedTxs)

				return txs, expTxHashes
			},
			verifyFunc: func(mpool mempool.Mempool) {
				// Only nonce 0 should be pending (the first consecutive transaction)
				// nonces 2, 4, 6 should be queued
				count := mpool.CountTx()
				s.Require().Equal(1, count, "Only nonce 0 should be pending, others should be queued")
			},
		},
		{
			name: "fill nonce gap and verify pending count increases",
			setupTxs: func() ([]sdk.Tx, []string) {
				key := s.keyring.GetKey(0)
				var txs []sdk.Tx

				// First, insert transactions with gaps: nonces 0, 2, 4
				for i := 0; i <= 4; i += 2 {
					tx := s.createEVMValueTransferTx(key, i, big.NewInt(1000000000))
					txs = append(txs, tx)
				}

				// Then fill the gap by inserting nonce 1
				tx := s.createEVMValueTransferTx(key, 1, big.NewInt(1000000000))
				txs = append(txs, tx)

				// Expected txs in order
				expectedTxs := []sdk.Tx{txs[0], txs[3], txs[1]}
				expTxHashes := s.getTxHashes(expectedTxs)

				return txs, expTxHashes
			},
			verifyFunc: func(mpool mempool.Mempool) {
				// After filling nonce 1, transactions 0, 1, 2 should be pending
				// nonce 4 should still be queued
				count := mpool.CountTx()
				s.Require().Equal(3, count, "After filling gap, nonces 0, 1, 2 should be pending")
			},
		},
		{
			name: "fill multiple nonce gaps",
			setupTxs: func() ([]sdk.Tx, []string) {
				key := s.keyring.GetKey(0)
				var txs []sdk.Tx

				// Insert transactions with multiple gaps: nonces 0, 3, 6, 9
				for i := 0; i <= 9; i += 3 {
					tx := s.createEVMValueTransferTx(key, i, big.NewInt(1000000000))
					txs = append(txs, tx)

				}

				// Fill gaps by inserting nonces 1, 2, 4, 5, 7, 8
				for i := 1; i <= 8; i++ {
					if i%3 != 0 { // Skip nonces that are already inserted
						tx := s.createEVMValueTransferTx(key, i, big.NewInt(1000000000))
						txs = append(txs, tx)

					}
				}

				// Expected txs in order
				expectedTxs := []sdk.Tx{txs[0], txs[4], txs[5], txs[1], txs[6], txs[7], txs[2], txs[8], txs[9], txs[3]}
				expTxHashes := s.getTxHashes(expectedTxs)

				return txs, expTxHashes
			},
			verifyFunc: func(mpool mempool.Mempool) {
				// After filling all gaps, all transactions should be pending
				count := mpool.CountTx()
				s.Require().Equal(10, count, "After filling all gaps, all 10 transactions should be pending")
			},
		},
		{
			name: "test different accounts with nonce gaps",
			setupTxs: func() ([]sdk.Tx, []string) {
				var txs []sdk.Tx

				// Use different keys for different accounts
				key1 := s.keyring.GetKey(0)
				key2 := s.keyring.GetKey(1)

				// Account 1: nonces 0, 2 (gap at 1)
				for i := 0; i <= 2; i += 2 {
					tx := s.createEVMValueTransferTx(key1, i, big.NewInt(1000000000))
					txs = append(txs, tx)
				}

				// Account 2: nonces 0, 3 (gaps at 1, 2)
				for i := 0; i <= 3; i += 3 {
					tx := s.createEVMValueTransferTx(key2, i, big.NewInt(1000000000))
					txs = append(txs, tx)
				}

				// Expected txs in order
				expectedTxs := []sdk.Tx{txs[0], txs[2]}
				expTxHashes := s.getTxHashes(expectedTxs)

				return txs, expTxHashes
			},
			verifyFunc: func(mpool mempool.Mempool) {
				// Account 1: nonce 0 pending, nonce 2 queued
				// Account 2: nonce 0 pending, nonce 3 queued
				// Total: 2 pending transactions
				count := mpool.CountTx()
				s.Require().Equal(2, count, "Only nonce 0 from each account should be pending")
			},
		},
		{
			name: "test replacement transactions with higher gas price",
			setupTxs: func() ([]sdk.Tx, []string) {
				key := s.keyring.GetKey(0)
				var txs []sdk.Tx

				// Insert transaction with nonce 0 and low gas price
				tx1 := s.createEVMValueTransferTx(key, 0, big.NewInt(1000000000))
				txs = append(txs, tx1)

				// Insert transaction with nonce 1
				tx2 := s.createEVMValueTransferTx(key, 1, big.NewInt(1000000000))
				txs = append(txs, tx2)

				// Replace nonce 0 transaction with higher gas price
				tx3 := s.createEVMValueTransferTx(key, 0, big.NewInt(2000000000))
				txs = append(txs, tx3)

				// Expected txs in order
				expectedTxs := []sdk.Tx{txs[2], txs[1]}
				expTxHashes := s.getTxHashes(expectedTxs)

				return txs, expTxHashes
			},
			verifyFunc: func(mpool mempool.Mempool) {
				// After replacement, both nonces 0 and 1 should be pending
				count := mpool.CountTx()
				s.Require().Equal(2, count, "After replacement, both transactions should be pending")
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Clean up previous test's resources before resetting
			s.TearDownTest()
			s.SetupTest()

			txs, expTxHashes := tc.setupTxs()

			// Call CheckTx for transactions
			err := s.checkTxs(txs)
			s.Require().NoError(err)

			// Call FinalizeBlock to make finalizeState before calling PrepareProposal
			_, err = s.network.FinalizeBlock()
			s.Require().NoError(err)

			// Call PrepareProposal to selcet transactions from mempool and make proposal
			prepareProposalRes, err := s.network.App.PrepareProposal(&abci.RequestPrepareProposal{
				MaxTxBytes: 1_000_000,
				Height:     1,
			})
			s.Require().NoError(err)

			mpool := s.network.App.GetMempool()
			iterator := mpool.Select(s.network.GetContext(), nil)

			// Check whether expected transactions are included and returned as pending state in mempool
			for _, txHash := range expTxHashes {
				actualTxHash := s.getTxHash(iterator.Tx())
				s.Require().Equal(txHash, actualTxHash)

				iterator = iterator.Next()
			}
			tc.verifyFunc(mpool)

			// Check whether expected transactions are selcted by PrepareProposal
			txHashes := make([]string, 0)
			for _, txBytes := range prepareProposalRes.Txs {
				txHash := hex.EncodeToString(tmhash.Sum(txBytes))
				txHashes = append(txHashes, txHash)
			}
			s.Require().Equal(expTxHashes, txHashes)
		})
	}
}

// TestCheckTxHandlerForCommittedAndLowerNonceTxs tests that:
// 1. Committed transactions are not in the mempool after block finalization
// 2. New transactions with nonces lower than current nonce fail at mempool level
func (s *IntegrationTestSuite) TestCheckTxHandlerForCommittedAndLowerNonceTxs() {
	testCases := []struct {
		name       string
		setupTxs   func() []sdk.Tx
		verifyFunc func()
	}{
		{
			name: "EVM transactions: committed txs removed from mempool and lower nonce txs fail",
			setupTxs: func() []sdk.Tx {
				key := s.keyring.GetKey(0)

				// Create transactions with sequential nonces (0, 1, 2)
				tx0 := s.createEVMValueTransferTx(key, 0, big.NewInt(2000000000))
				tx1 := s.createEVMValueTransferTx(key, 1, big.NewInt(2000000000))
				tx2 := s.createEVMValueTransferTx(key, 2, big.NewInt(2000000000))

				return []sdk.Tx{tx0, tx1, tx2}
			},
			verifyFunc: func() {
				// 1. Verify the correct nonce transaction is in mempool
				mpool := s.network.App.GetMempool()
				s.Require().Equal(0, mpool.CountTx(), "Only the correct nonce transaction should be in mempool")

				// 2. Check current sequence
				acc := s.network.App.GetAccountKeeper().GetAccount(s.network.GetContext(), s.keyring.GetAccAddr(0))
				sequence := acc.GetSequence()
				s.Require().Equal(uint64(3), sequence)

				// 3. Check new transactions with nonces lower than current nonce fails
				// Current nonce should be 3 after committing nonces 1, 2
				//
				// NOTE: The reason we don't try tx with nonce 0 is
				// because txFactory replace nonce 0 with curreent nonce.
				// So we just test for nonce 1 and 2.
				key := s.keyring.GetKey(0)

				// Try to add transaction with nonce 1 (lower than current nonce 3) - should fail
				dupTx1 := s.createEVMValueTransferTx(key, 1, big.NewInt(2000000000))
				res, err := s.checkTx(dupTx1)
				s.Require().NoError(err, "Transaction with nonce 1 should fail when current nonce is 3")
				s.Require().Contains(res.GetLog(), core.ErrNonceTooLow.Error())
				s.Require().Equal(0, mpool.CountTx(), "Only the correct nonce transaction should be in mempool")

				// Try to add transaction with nonce 2 (lower than current nonce 3) - should fail
				dupTx2 := s.createEVMValueTransferTx(key, 2, big.NewInt(2000000000))
				res, err = s.checkTx(dupTx2)
				s.Require().NoError(err, "Transaction with nonce 2 should fail when current nonce is 3")
				s.Require().Contains(res.GetLog(), core.ErrNonceTooLow.Error())
				s.Require().Equal(0, mpool.CountTx(), "Only the correct nonce transaction should be in mempool")

				// Verify transaction with correct nonce (3) still works
				tx3 := s.createEVMValueTransferTx(key, 3, big.NewInt(2000000000))
				res, err = s.checkTx(tx3)
				s.Require().NoError(err, "Transaction with correct nonce 3 should succeed")
				s.Require().Equal(abci.CodeTypeOK, res.Code)
				s.Require().Equal(1, mpool.CountTx(), "Only the correct nonce transaction should be in mempool")
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Clean up previous test's resources before resetting
			s.TearDownTest()
			// Reset test setup to ensure clean state
			s.SetupTest()

			txs := tc.setupTxs()

			// Call CheckTx for transactions
			err := s.checkTxs(txs)
			s.Require().NoError(err)

			// Finalize block with txs and Commit state
			txBytes, err := s.getTxBytes(txs)
			s.Require().NoError(err)

			_, err = s.network.NextBlockWithTxs(txBytes...)
			s.Require().NoError(err)

			// Manually trigger chain head event to notify mempool about the new block
			// This simulates the natural block notification that occurs in production
			s.notifyNewBlockToMempool()

			// Run verification function
			tc.verifyFunc()
		})
	}
}
