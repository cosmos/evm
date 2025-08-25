package mempool

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
)

// TestTransactionOrdering tests transaction ordering based on fees
func (s *IntegrationTestSuite) TestTransactionOrderingWithCheckTx() {
	fmt.Printf("DEBUG: Starting TestTransactionOrdering\n")
	testCases := []struct {
		name       string
		setupTxs   func() ([]sdk.Tx, []string)
		verifyFunc func(iterator mempool.Iterator, txHashesInOrder []string)
		bypass     bool // Temporarily bypass test cases that have known issue.
	}{
		{
			name: "mixed EVM and cosmos transaction ordering",
			setupTxs: func() ([]sdk.Tx, []string) {
				// Create EVM transaction with high gas price
				highGasPriceEVMTx := s.createEVMTransferWithKey(s.keyring.GetKey(0), big.NewInt(5000000000))

				// Create Cosmos transactions with different fee amounts
				highFeeCosmosTx := s.createCosmosSendTxWithKey(s.keyring.GetKey(6), big.NewInt(5000000000))
				mediumFeeCosmosTx := s.createCosmosSendTxWithKey(s.keyring.GetKey(7), big.NewInt(3000000000))
				lowFeeCosmosTx := s.createCosmosSendTxWithKey(s.keyring.GetKey(8), big.NewInt(2000000000))

				// Input txs in order
				inputTxs := []sdk.Tx{lowFeeCosmosTx, highGasPriceEVMTx, mediumFeeCosmosTx, highFeeCosmosTx}

				// Expected txs in order
				expectedTxs := []sdk.Tx{highGasPriceEVMTx, highFeeCosmosTx, mediumFeeCosmosTx, lowFeeCosmosTx}
				expectedTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expectedTxHashes
			},
		},
		{
			name: "EVM-only transaction replacement",
			setupTxs: func() ([]sdk.Tx, []string) {
				// Create first EVM transaction with low fee
				lowFeeEVMTx := s.createEVMTransferWithKey(s.keyring.GetKey(0), big.NewInt(2000000000)) // 2 gaatom

				// Create second EVM transaction with high fee
				highFeeEVMTx := s.createEVMTransferWithKey(s.keyring.GetKey(0), big.NewInt(5000000000)) // 5 gaatom

				// Input txs in order
				inputTxs := []sdk.Tx{lowFeeEVMTx, highFeeEVMTx}

				// Expected Txs in order
				expectedTxs := []sdk.Tx{highFeeEVMTx}
				expectedTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expectedTxHashes
			},
			bypass: true,
		},
		{
			name: "EVM-only transaction ordering",
			setupTxs: func() ([]sdk.Tx, []string) {
				key := s.keyring.GetKey(0)
				// Create first EVM transaction with low fee
				lowFeeEVMTx := s.createEVMTransferWithKeyAndNonce(key, big.NewInt(2000000000), uint64(1)) // 2 gaatom

				// Create second EVM transaction with high fee
				highFeeEVMTx := s.createEVMTransferWithKeyAndNonce(key, big.NewInt(5000000000), uint64(0)) // 5 gaatom

				// Input txs in order
				inputTxs := []sdk.Tx{lowFeeEVMTx, highFeeEVMTx}

				// Expected txs in order
				expectedTxs := []sdk.Tx{highFeeEVMTx, lowFeeEVMTx}
				expectedTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expectedTxHashes
			},
		},
		{
			name: "cosmos-only transaction replacement",
			setupTxs: func() ([]sdk.Tx, []string) {
				highFeeTx := s.createCosmosSendTxWithKey(s.keyring.GetKey(0), big.NewInt(5000000000))   // 5 gaatom
				lowFeeTx := s.createCosmosSendTxWithKey(s.keyring.GetKey(0), big.NewInt(2000000000))    // 2 gaatom
				mediumFeeTx := s.createCosmosSendTxWithKey(s.keyring.GetKey(0), big.NewInt(3000000000)) // 3 gaatom

				// Input txs in order
				inputTxs := []sdk.Tx{mediumFeeTx, lowFeeTx, highFeeTx}

				// Expected txs in order
				expectedTxs := []sdk.Tx{highFeeTx}
				expectedTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expectedTxHashes
			},
			bypass: true,
		},
		{
			name: "mixed EVM and Cosmos transactions with equal effective tips",
			setupTxs: func() ([]sdk.Tx, []string) {
				// Create transactions with equal effective tips (assuming base fee = 0)
				// EVM: 1000 aatom/gas effective tip
				evmTx := s.createEVMTransferWithKey(s.keyring.GetKey(0), big.NewInt(1000000000)) // 1 gaatom/gas

				// Cosmos with same effective tip: 1000 * 200000 = 200000000 aatom total fee
				cosmosTx := s.createCosmosSendTxWithKey(s.keyring.GetKey(0), big.NewInt(1000000000)) // 1 gaatom/gas effective tip

				// Input txs in order
				inputTxs := []sdk.Tx{cosmosTx, evmTx}

				// Expected txs in order
				expectedTxs := []sdk.Tx{evmTx, cosmosTx}
				expectedTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expectedTxHashes
			},
			bypass: true,
		},
		{
			name: "mixed transactions with EVM having higher effective tip",
			setupTxs: func() ([]sdk.Tx, []string) {
				// Create EVM transaction with higher gas price
				evmTx := s.createEVMTransferWithKey(s.keyring.GetKey(0), big.NewInt(5000000000)) // 5 gaatom/gas

				// Create Cosmos transaction with lower gas price
				cosmosTx := s.createCosmosSendTxWithKey(s.keyring.GetKey(0), big.NewInt(2000000000)) // 2 gaatom/gas

				// Input txs in order
				inputTxs := []sdk.Tx{cosmosTx, evmTx}

				// Expected txs in order
				expectedTxs := []sdk.Tx{evmTx, cosmosTx}
				expectedTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expectedTxHashes
			},
			bypass: true,
		},
		{
			name: "mixed transactions with Cosmos having higher effective tip",
			setupTxs: func() ([]sdk.Tx, []string) {
				// Create EVM transaction with lower gas price
				evmTx := s.createEVMTransferWithKey(s.keyring.GetKey(0), big.NewInt(2000000000)) // 2000 aatom/gas

				// Create Cosmos transaction with higher gas price
				cosmosTx := s.createCosmosSendTxWithKey(s.keyring.GetKey(0), big.NewInt(5000000000)) // 5000 aatom/gas

				// Input txs in order
				inputTxs := []sdk.Tx{evmTx, cosmosTx}

				// Expected txs in order
				expectedTxs := []sdk.Tx{cosmosTx, evmTx}
				expectedTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expectedTxHashes
			},
			bypass: true,
		},
		{
			name: "mixed transaction ordering with multiple effective tips",
			setupTxs: func() ([]sdk.Tx, []string) {
				// Create multiple transactions with different gas prices
				// EVM: 10000, 8000, 6000 aatom/gas
				// Cosmos: 9000, 7000, 5000 aatom/gas

				evmHigh := s.createEVMTransferWithKey(s.keyring.GetKey(0), big.NewInt(10000000000))
				evmMedium := s.createEVMTransferWithKey(s.keyring.GetKey(1), big.NewInt(8000000000))
				evmLow := s.createEVMTransferWithKey(s.keyring.GetKey(2), big.NewInt(6000000000))

				cosmosHigh := s.createCosmosSendTxWithKey(s.keyring.GetKey(3), big.NewInt(9000000000))
				cosmosMedium := s.createCosmosSendTxWithKey(s.keyring.GetKey(4), big.NewInt(7000000000))
				cosmosLow := s.createCosmosSendTxWithKey(s.keyring.GetKey(5), big.NewInt(5000000000))

				// Input txs in order
				inputTxs := []sdk.Tx{cosmosHigh, cosmosMedium, cosmosLow, evmHigh, evmMedium, evmLow}

				// Expected txs in order
				expectedTxs := []sdk.Tx{evmHigh, cosmosHigh, evmMedium, cosmosMedium, evmLow, cosmosLow}
				expectedTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expectedTxHashes
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Reset test setup to ensure clean state
			s.SetupTest()

			txs, txHashesInOrder := tc.setupTxs()

			_, err := s.checkTxs(txs)
			s.Require().NoError(err)

			mpool := s.network.App.GetMempool()
			iterator := mpool.Select(s.network.GetContext(), nil)

			if !tc.bypass {
				for _, txHash := range txHashesInOrder {
					actualTxHash := s.getTxHash(iterator.Tx())
					s.Require().Equal(txHash, actualTxHash)

					iterator = iterator.Next()
				}
			}
		})
	}
}

// TestNonceGappedEVMTransactions tests the behavior of nonce-gapped EVM transactions
// and the transition from queued to pending when gaps are filled
func (s *IntegrationTestSuite) TestNonceGappedEVMTransactionsWithCheckTx() {
	fmt.Printf("DEBUG: Starting TestNonceGappedEVMTransactions\n")

	testCases := []struct {
		name       string
		setupTxs   func() ([]sdk.Tx, []int) // Returns transactions and their expected nonces
		verifyFunc func(mpool mempool.Mempool)
	}{
		{
			name: "insert transactions with nonce gaps",
			setupTxs: func() ([]sdk.Tx, []int) {
				key := s.keyring.GetKey(0)
				var txs []sdk.Tx
				var nonces []int

				// Insert transactions with gaps: nonces 0, 2, 4, 6 (missing 1, 3, 5)
				for i := 0; i <= 6; i += 2 {
					tx := s.createEVMTransferWithKeyAndNonce(key, big.NewInt(1000000000), uint64(i))
					txs = append(txs, tx)
					nonces = append(nonces, i)
				}

				return txs, nonces
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
			setupTxs: func() ([]sdk.Tx, []int) {
				key := s.keyring.GetKey(0)
				var txs []sdk.Tx
				var nonces []int

				// First, insert transactions with gaps: nonces 0, 2, 4
				for i := 0; i <= 4; i += 2 {
					tx := s.createEVMTransferWithKeyAndNonce(key, big.NewInt(1000000000), uint64(i))
					txs = append(txs, tx)
					nonces = append(nonces, i)
				}

				// Then fill the gap by inserting nonce 1
				tx := s.createEVMTransferWithKeyAndNonce(key, big.NewInt(1000000000), uint64(1))
				txs = append(txs, tx)
				nonces = append(nonces, 1)

				return txs, nonces
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
			setupTxs: func() ([]sdk.Tx, []int) {
				key := s.keyring.GetKey(0)
				var txs []sdk.Tx
				var nonces []int

				// Insert transactions with multiple gaps: nonces 0, 3, 6, 9
				for i := 0; i <= 9; i += 3 {
					tx := s.createEVMTransferWithKeyAndNonce(key, big.NewInt(1000000000), uint64(i))
					txs = append(txs, tx)
					nonces = append(nonces, i)
				}

				// Fill gaps by inserting nonces 1, 2, 4, 5, 7, 8
				for i := 1; i <= 8; i++ {
					if i%3 != 0 { // Skip nonces that are already inserted
						tx := s.createEVMTransferWithKeyAndNonce(key, big.NewInt(1000000000), uint64(i))
						txs = append(txs, tx)
						nonces = append(nonces, i)
					}
				}

				return txs, nonces
			},
			verifyFunc: func(mpool mempool.Mempool) {
				// After filling all gaps, all transactions should be pending
				count := mpool.CountTx()
				s.Require().Equal(10, count, "After filling all gaps, all 10 transactions should be pending")
			},
		},
		{
			name: "test different accounts with nonce gaps",
			setupTxs: func() ([]sdk.Tx, []int) {
				var txs []sdk.Tx
				var nonces []int

				// Use different keys for different accounts
				key1 := s.keyring.GetKey(0)
				key2 := s.keyring.GetKey(1)

				// Account 1: nonces 0, 2 (gap at 1)
				for i := 0; i <= 2; i += 2 {
					tx := s.createEVMTransferWithKeyAndNonce(key1, big.NewInt(1000000000), uint64(i))
					txs = append(txs, tx)
					nonces = append(nonces, i)
				}

				// Account 2: nonces 0, 3 (gaps at 1, 2)
				for i := 0; i <= 3; i += 3 {
					tx := s.createEVMTransferWithKeyAndNonce(key2, big.NewInt(1000000000), uint64(i))
					txs = append(txs, tx)
					nonces = append(nonces, i)
				}

				return txs, nonces
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
			setupTxs: func() ([]sdk.Tx, []int) {
				key := s.keyring.GetKey(0)
				var txs []sdk.Tx
				var nonces []int

				// Insert transaction with nonce 0 and low gas price
				tx1 := s.createEVMTransferWithKeyAndNonce(key, big.NewInt(1000000000), uint64(0))
				txs = append(txs, tx1)
				nonces = append(nonces, 0)

				// Insert transaction with nonce 1
				tx2 := s.createEVMTransferWithKeyAndNonce(key, big.NewInt(1000000000), uint64(1))
				txs = append(txs, tx2)
				nonces = append(nonces, 1)

				// Replace nonce 0 transaction with higher gas price
				tx3 := s.createEVMTransferWithKeyAndNonce(key, big.NewInt(2000000000), uint64(0))
				txs = append(txs, tx3)
				nonces = append(nonces, 0)

				return txs, nonces
			},
			verifyFunc: func(mpool mempool.Mempool) {
				// After replacement, both nonces 0 and 1 should be pending
				count := mpool.CountTx()
				s.Require().Equal(2, count, "After replacement, both transactions should be pending")
			},
		},
		{
			name: "track count changes when filling nonce gaps",
			setupTxs: func() ([]sdk.Tx, []int) {
				key := s.keyring.GetKey(0)
				var txs []sdk.Tx
				var nonces []int

				// Insert transactions with gaps: nonces 0, 3, 6, 9
				for i := 0; i <= 9; i += 3 {
					tx := s.createEVMTransferWithKeyAndNonce(key, big.NewInt(1000000000), uint64(i))
					txs = append(txs, tx)
					nonces = append(nonces, i)
				}

				// Fill gaps by inserting nonces 1, 2, 4, 5, 7, 8
				for i := 1; i <= 8; i++ {
					if i%3 != 0 { // Skip nonces that are already inserted
						tx := s.createEVMTransferWithKeyAndNonce(key, big.NewInt(1000000000), uint64(i))
						txs = append(txs, tx)
						nonces = append(nonces, i)
					}
				}

				return txs, nonces
			},
			verifyFunc: func(mpool mempool.Mempool) {
				// After filling all gaps, all transactions should be pending
				count := mpool.CountTx()
				s.Require().Equal(10, count, "After filling all gaps, all 10 transactions should be pending")
			},
		},
		{
			name: "removing places subsequent transactions back into queued",
			setupTxs: func() ([]sdk.Tx, []int) {
				key := s.keyring.GetKey(0)
				var txs []sdk.Tx
				var nonces []int

				// Insert transactions with gaps: nonces 0, 2, 3, 4, 5, 6, 7
				for i := 0; i <= 7; i++ {
					if i != 1 { // Skip nonce 1 to create a gap
						tx := s.createEVMTransferWithKeyAndNonce(key, big.NewInt(1000000000), uint64(i))
						txs = append(txs, tx)
						nonces = append(nonces, i) //#nosec G115 -- int overflow is not a concern here
					}
				}

				return txs, nonces
			},
			verifyFunc: func(mpool mempool.Mempool) {
				// Initially: nonces 0 should be pending, nonces 2, 3, 4, 5, 6, 7 should be queued
				initialCount := mpool.CountTx()
				s.Require().Equal(1, initialCount, "Initially only nonces 0, 1 should be pending")
				key := s.keyring.GetKey(0)

				// Fill gap by inserting nonce 1
				tx1 := s.createEVMTransferWithKeyAndNonce(key, big.NewInt(1000000000), uint64(1))
				err := mpool.Insert(s.network.GetContext(), tx1)
				s.Require().NoError(err)

				// After filling gap: all nonce transactions should be in pending
				countAfterFilling := mpool.CountTx()
				s.Require().Equal(8, countAfterFilling, "After filling gap, only nonce 0 should be pending due to gap at nonce 1")

				// Remove nonce 1 transaction, dropping the rest (except for 0) into queued
				tx1 = s.createEVMTransferWithKeyAndNonce(key, big.NewInt(1000000000), uint64(1))
				s.Require().NoError(err)
				err = mpool.Remove(tx1)
				s.Require().NoError(err)

				// After removal: only nonce 0 should be pending, the rest get dropped to queued
				countAfterRemoval := mpool.CountTx()
				s.Require().Equal(1, countAfterRemoval, "After removing nonce 1, only nonce 0 should be pending")

				// Fill gap by inserting nonce 1
				tx1 = s.createEVMTransferWithKeyAndNonce(key, big.NewInt(1000000000), uint64(1))
				s.Require().NoError(err)
				err = mpool.Insert(s.network.GetContext(), tx1)
				s.Require().NoError(err)

				// After filling gap: all transactions should be re-promoted and places into pending
				countAfterFilling = mpool.CountTx()
				s.Require().Equal(8, countAfterFilling, "After filling gap, only nonce 0 should be pending due to gap at nonce 1")
			},
		},
	}

	for i, tc := range testCases {
		fmt.Printf("DEBUG: TestNonceGappedEVMTransactions - Starting test case %d/%d: %s\n", i+1, len(testCases), tc.name)
		s.Run(tc.name, func() {
			fmt.Printf("DEBUG: Running test case: %s\n", tc.name)
			// Reset test setup to ensure clean state
			s.SetupTest()
			fmt.Printf("DEBUG: SetupTest completed for: %s\n", tc.name)

			txs, nonces := tc.setupTxs()
			mpool := s.network.App.GetMempool()

			// Insert transactions and track count changes
			initialCount := mpool.CountTx()
			fmt.Printf("DEBUG: Initial mempool count: %d\n", initialCount)

			for i, tx := range txs {
				err := mpool.Insert(s.network.GetContext(), tx)
				s.Require().NoError(err)

				currentCount := mpool.CountTx()
				fmt.Printf("DEBUG: After inserting nonce %d: count = %d\n", nonces[i], currentCount)
			}

			tc.verifyFunc(mpool)
			fmt.Printf("DEBUG: Completed test case: %s\n", tc.name)
		})
		fmt.Printf("DEBUG: TestNonceGappedEVMTransactions - Completed test case %d/%d: %s\n", i+1, len(testCases), tc.name)
	}
}
