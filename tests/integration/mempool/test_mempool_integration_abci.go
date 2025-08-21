package mempool

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// TestTransactionOrdering tests transaction ordering based on fees
func (s *IntegrationTestSuite) TestTransactionOrderingWithCheckTx() {
	fmt.Printf("DEBUG: Starting TestTransactionOrdering\n")
	testCases := []struct {
		name       string
		setupTxs   func() []sdk.Tx
		verifyFunc func(iterator mempool.Iterator)
	}{
		{
			name: "mixed EVM and cosmos transaction ordering",
			setupTxs: func() []sdk.Tx {
				// Create EVM transaction with high gas price
				highGasPriceEVMTx, err := s.createEVMTransaction(big.NewInt(5000000000))
				s.Require().NoError(err)

				// Create Cosmos transactions with different fee amounts
				highFeeCosmosTx := s.createCosmosSendTransactionWithKey(s.keyring.GetKey(6), big.NewInt(5000000000))
				mediumFeeCosmosTx := s.createCosmosSendTransactionWithKey(s.keyring.GetKey(7), big.NewInt(3000000000))
				lowFeeCosmosTx := s.createCosmosSendTransactionWithKey(s.keyring.GetKey(8), big.NewInt(1000000000))

				// Insert in non-priority order
				return []sdk.Tx{lowFeeCosmosTx, highGasPriceEVMTx, mediumFeeCosmosTx, highFeeCosmosTx}
			},
			verifyFunc: func(iterator mempool.Iterator) {
				// First transaction should be EVM with highest gas price (5 gaatom = 5000000000 aatom)
				tx1 := iterator.Tx()
				s.Require().NotNil(tx1)

				ethMsg, ok := tx1.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
				s.Require().True(ok)
				ethTx := ethMsg.AsTransaction()
				s.Require().Equal(big.NewInt(5000000000), ethTx.GasPrice(), "First transaction should be EVM with highest gas price")

				// Second transaction should be Cosmos with high fee (25000 aatom gas price)
				iterator = iterator.Next()
				s.Require().NotNil(iterator)
				tx2 := iterator.Tx()
				s.Require().NotNil(tx2)

				// Should be Cosmos transaction with high fee
				feeTx := tx2.(sdk.FeeTx)
				cosmosGasPrice := s.calculateCosmosGasPrice(feeTx.GetFee().AmountOf("aatom").BigInt().Int64(), feeTx.GetGas())
				s.Require().Equal(big.NewInt(5000000000), cosmosGasPrice, "Second transaction should be Cosmos with 25000 aatom gas price")
			},
		},
		{
			name: "EVM-only transaction replacement",
			setupTxs: func() []sdk.Tx {
				// Create first EVM transaction with low fee
				lowFeeEVMTx, err := s.createEVMTransaction(big.NewInt(1000000000)) // 1 gaatom
				s.Require().NoError(err)

				// Create second EVM transaction with high fee
				highFeeEVMTx, err := s.createEVMTransaction(big.NewInt(5000000000)) // 5 gaatom
				s.Require().NoError(err)

				// Insert low fee transaction first
				return []sdk.Tx{lowFeeEVMTx, highFeeEVMTx}
			},
			verifyFunc: func(iterator mempool.Iterator) {
				// First transaction should be high fee
				tx1 := iterator.Tx()
				s.Require().NotNil(tx1)
				ethMsg, ok := tx1.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
				s.Require().True(ok)
				ethTx := ethMsg.AsTransaction()
				s.Require().Equal(big.NewInt(5000000000), ethTx.GasPrice())
				iterator = iterator.Next()
				s.Require().Nil(iterator) // transaction with same nonce got replaced by higher fee
			},
		},
		{
			name: "EVM-only transaction replacement",
			setupTxs: func() []sdk.Tx {
				key := s.keyring.GetKey(0)
				// Create first EVM transaction with low fee
				lowFeeEVMTx, err := s.createEVMTransactionWithNonce(key, big.NewInt(1000000000), 1) // 1 gaatom
				s.Require().NoError(err)

				// Create second EVM transaction with high fee
				highFeeEVMTx, err := s.createEVMTransactionWithNonce(key, big.NewInt(5000000000), 0) // 5 gaatom
				s.Require().NoError(err)

				// Insert low fee transaction first
				return []sdk.Tx{lowFeeEVMTx, highFeeEVMTx}
			},
			verifyFunc: func(iterator mempool.Iterator) {
				// First transaction should be high fee
				tx1 := iterator.Tx()
				s.Require().NotNil(tx1)
				ethMsg, ok := tx1.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
				s.Require().True(ok)
				ethTx := ethMsg.AsTransaction()
				s.Require().Equal(big.NewInt(5000000000), ethTx.GasPrice())
				iterator = iterator.Next()
				s.Require().NotNil(iterator)
				tx2 := iterator.Tx()
				s.Require().NotNil(tx2)
				ethMsg, ok = tx2.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
				s.Require().True(ok)
				ethTx = ethMsg.AsTransaction()
				s.Require().Equal(big.NewInt(1000000000), ethTx.GasPrice())
				iterator = iterator.Next()
				s.Require().Nil(iterator)
			},
		},
		{
			name: "cosmos-only transaction replacement",
			setupTxs: func() []sdk.Tx {
				highFeeTx := s.createCosmosSendTransactionWithKey(s.keyring.GetKey(0), big.NewInt(5000000000))   // 5 gaatom
				lowFeeTx := s.createCosmosSendTransactionWithKey(s.keyring.GetKey(0), big.NewInt(1000000000))    // 1 gaatom
				mediumFeeTx := s.createCosmosSendTransactionWithKey(s.keyring.GetKey(0), big.NewInt(3000000000)) // 3 gaatom

				// Insert in random order
				return []sdk.Tx{mediumFeeTx, lowFeeTx, highFeeTx}
			},
			verifyFunc: func(iterator mempool.Iterator) {
				// Should get first transaction from cosmos pool
				tx1 := iterator.Tx()
				s.Require().NotNil(tx1)
				// Calculate gas price: fee_amount / gas_limit = 5000000000 / 200000 = 25000
				expectedGasPrice := big.NewInt(5000000000)
				feeTx := tx1.(sdk.FeeTx)
				actualGasPrice := s.calculateCosmosGasPrice(feeTx.GetFee().AmountOf("aatom").Int64(), feeTx.GetGas())
				s.Require().Equal(expectedGasPrice, actualGasPrice, "Expected gas price should match fee_amount/gas_limit")
				iterator = iterator.Next()
				s.Require().Nil(iterator)
			},
		},
		{
			name: "mixed EVM and Cosmos transactions with equal effective tips",
			setupTxs: func() []sdk.Tx {
				// Create transactions with equal effective tips (assuming base fee = 0)
				// EVM: 1000 aatom/gas effective tip
				evmTx, err := s.createEVMTransaction(big.NewInt(1000000000)) // 1 gaatom/gas
				s.Require().NoError(err)

				// Cosmos with same effective tip: 1000 * 200000 = 200000000 aatom total fee
				cosmosTx := s.createCosmosSendTransaction(big.NewInt(1000000000)) // 1 gaatom/gas effective tip

				// Insert Cosmos first, then EVM
				return []sdk.Tx{cosmosTx, evmTx}
			},
			verifyFunc: func(iterator mempool.Iterator) {
				// Both transactions have equal effective tip, so either could be first
				// But EVM should be preferred when effective tips are equal
				tx1 := iterator.Tx()
				s.Require().NotNil(tx1)

				// Check if first transaction is EVM (preferred when effective tips are equal)
				ethMsg, ok := tx1.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
				s.Require().True(ok)
				ethTx := ethMsg.AsTransaction()
				// For EVM, effective tip = gas_price - base_fee (assuming base fee = 0)
				effectiveTip := ethTx.GasPrice() // effective_tip = gas_price - 0
				s.Require().Equal(big.NewInt(1000000000), effectiveTip, "First transaction should be EVM with 1 gaatom effective tip")

				// Second transaction should be the other type
				iterator = iterator.Next()
				s.Require().NotNil(iterator)
				tx2 := iterator.Tx()
				s.Require().NotNil(tx2)

				feeTx := tx2.(sdk.FeeTx)
				effectiveTip = s.calculateCosmosEffectiveTip(feeTx.GetFee().AmountOf("aatom").Int64(), feeTx.GetGas(), big.NewInt(0)) // base fee = 0
				s.Require().Equal(big.NewInt(1000000000), effectiveTip, "Second transaction should be Cosmos with 1000 aatom effective tip")
			},
		},
		{
			name: "mixed transactions with EVM having higher effective tip",
			setupTxs: func() []sdk.Tx {
				// Create EVM transaction with higher gas price
				evmTx, err := s.createEVMTransaction(big.NewInt(5000000000)) // 5 gaatom/gas
				s.Require().NoError(err)

				// Create Cosmos transaction with lower gas price
				cosmosTx := s.createCosmosSendTransaction(big.NewInt(2000000000)) // 2 gaatom/gas

				// Insert Cosmos first, then EVM
				return []sdk.Tx{cosmosTx, evmTx}
			},
			verifyFunc: func(iterator mempool.Iterator) {
				// EVM should be first due to higher effective tip
				tx1 := iterator.Tx()
				s.Require().NotNil(tx1)

				ethMsg, ok := tx1.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
				s.Require().True(ok, "First transaction should be EVM due to higher effective tip")
				ethTx := ethMsg.AsTransaction()
				effectiveTip := ethTx.GasPrice() // effective_tip = gas_price - 0
				s.Require().Equal(big.NewInt(5000000000), effectiveTip, "First transaction should be EVM with 5000 aatom effective tip")

				// Second transaction should be Cosmos
				iterator = iterator.Next()
				s.Require().NotNil(iterator)
				tx2 := iterator.Tx()
				s.Require().NotNil(tx2)

				feeTx := tx2.(sdk.FeeTx)
				effectiveTip2 := s.calculateCosmosEffectiveTip(feeTx.GetFee().AmountOf("aatom").Int64(), feeTx.GetGas(), big.NewInt(0)) // base fee = 0
				s.Require().Equal(big.NewInt(2000000000), effectiveTip2, "Second transaction should be Cosmos with 2000 aatom effective tip")
			},
		},
		{
			name: "mixed transactions with Cosmos having higher effective tip",
			setupTxs: func() []sdk.Tx {
				// Create EVM transaction with lower gas price
				evmTx, err := s.createEVMTransaction(big.NewInt(2000000000)) // 2000 aatom/gas
				s.Require().NoError(err)

				// Create Cosmos transaction with higher gas price
				cosmosTx := s.createCosmosSendTransaction(big.NewInt(5000000000)) // 5000 aatom/gas

				// Insert EVM first, then Cosmos
				return []sdk.Tx{evmTx, cosmosTx}

			},
			verifyFunc: func(iterator mempool.Iterator) {
				// Cosmos should be first due to higher effective tip
				tx1 := iterator.Tx()
				s.Require().NotNil(tx1)

				feeTx := tx1.(sdk.FeeTx)
				effectiveTip := s.calculateCosmosEffectiveTip(feeTx.GetFee().AmountOf("aatom").Int64(), feeTx.GetGas(), big.NewInt(0)) // base fee = 0
				s.Require().Equal(big.NewInt(5000000000), effectiveTip, "First transaction should be Cosmos with 5000 aatom effective tip")

				// Second transaction should be EVM
				iterator = iterator.Next()
				s.Require().NotNil(iterator)
				tx2 := iterator.Tx()
				s.Require().NotNil(tx2)

				ethMsg, ok := tx2.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
				s.Require().True(ok, "Second transaction should be EVM")
				ethTx := ethMsg.AsTransaction()
				effectiveTip2 := ethTx.GasPrice() // effective_tip = gas_price - 0
				s.Require().Equal(big.NewInt(2000000000), effectiveTip2, "Second transaction should be EVM with 2000 aatom effective tip")
			},
		},
		{
			name: "mixed transaction ordering with multiple effective tips",
			setupTxs: func() []sdk.Tx {
				// Create multiple transactions with different gas prices
				// EVM: 8000, 4000, 2000 aatom/gas
				// Cosmos: 6000, 3000, 1000 aatom/gas

				evmHigh, err := s.createEVMTransaction(big.NewInt(8000000000))
				s.Require().NoError(err)
				evmMedium, err := s.createEVMTransactionWithKey(s.keyring.GetKey(1), big.NewInt(4000000000))
				s.Require().NoError(err)
				evmLow, err := s.createEVMTransactionWithKey(s.keyring.GetKey(2), big.NewInt(2000000000))
				s.Require().NoError(err)

				cosmosHigh := s.createCosmosSendTransactionWithKey(s.keyring.GetKey(3), big.NewInt(6000000000))
				cosmosMedium := s.createCosmosSendTransactionWithKey(s.keyring.GetKey(4), big.NewInt(3000000000))
				cosmosLow := s.createCosmosSendTransactionWithKey(s.keyring.GetKey(5), big.NewInt(1000000000))

				return []sdk.Tx{evmHigh, evmMedium, evmLow, cosmosHigh, cosmosMedium, cosmosLow}
			},
			verifyFunc: func(iterator mempool.Iterator) {
				// Expected order by gas price (highest first):
				// 1. EVM 8 gaatom/gas
				// 2. Cosmos 6 gaatom/gas
				// 3. EVM 4 gaatom/gas
				// 4. Cosmos 3 gaatom/gas
				// 5. EVM 2 gaatom/gas
				// 6. Cosmos 1 gaatom/gas

				// First: EVM 8
				tx1 := iterator.Tx()
				s.Require().NotNil(tx1)
				ethMsg, ok := tx1.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
				s.Require().True(ok, "First transaction should be EVM with highest gas price")
				ethTx := ethMsg.AsTransaction()
				s.Require().Equal(big.NewInt(8000000000), ethTx.GasPrice(), "First transaction should be EVM with 8000 aatom/gas")

				// Second: Cosmos 6
				iterator = iterator.Next()
				s.Require().NotNil(iterator)
				tx2 := iterator.Tx()
				s.Require().NotNil(tx2)
				feeTx2 := tx2.(sdk.FeeTx)
				cosmosGasPrice2 := s.calculateCosmosGasPrice(feeTx2.GetFee().AmountOf("aatom").Int64(), feeTx2.GetGas())
				s.Require().Equal(big.NewInt(6000000000), cosmosGasPrice2, "Second transaction should be Cosmos with 6000 aatom/gas")

				// Third: EVM 4
				iterator = iterator.Next()
				s.Require().NotNil(iterator)
				tx3 := iterator.Tx()
				s.Require().NotNil(tx3)
				ethMsg3, ok := tx3.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
				s.Require().True(ok, "Third transaction should be EVM")
				ethTx3 := ethMsg3.AsTransaction()
				s.Require().Equal(big.NewInt(4000000000), ethTx3.GasPrice(), "Third transaction should be EVM with 4000 aatom/gas")

				// Fourth: Cosmos 3
				iterator = iterator.Next()
				s.Require().NotNil(iterator)
				tx4 := iterator.Tx()
				s.Require().NotNil(tx4)
				feeTx4 := tx4.(sdk.FeeTx)
				cosmosGasPrice4 := s.calculateCosmosGasPrice(feeTx4.GetFee().AmountOf("aatom").Int64(), feeTx4.GetGas())
				s.Require().Equal(big.NewInt(3000000000), cosmosGasPrice4, "Fourth transaction should be Cosmos with 3000 aatom/gas")

				// Fifth: EVM 2
				iterator = iterator.Next()
				s.Require().NotNil(iterator)
				tx5 := iterator.Tx()
				s.Require().NotNil(tx5)
				ethMsg5, ok := tx5.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
				s.Require().True(ok, "Fifth transaction should be EVM")
				ethTx5 := ethMsg5.AsTransaction()
				s.Require().Equal(big.NewInt(2000000000), ethTx5.GasPrice(), "Fifth transaction should be EVM with 2000 aatom/gas")

				// Sixth: Cosmos 1
				iterator = iterator.Next()
				s.Require().NotNil(iterator)
				tx6 := iterator.Tx()
				s.Require().NotNil(tx6)
				feeTx6 := tx6.(sdk.FeeTx)
				cosmosGasPrice6 := s.calculateCosmosGasPrice(feeTx6.GetFee().AmountOf("aatom").Int64(), feeTx6.GetGas())
				s.Require().Equal(big.NewInt(1000000000), cosmosGasPrice6, "Sixth transaction should be Cosmos with 1000 aatom/gas")

				// No more transactions
				iterator = iterator.Next()
				s.Require().Nil(iterator)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Reset test setup to ensure clean state
			s.SetupTest()

			txs := tc.setupTxs()

			_, err := s.checkTxs(txs)
			s.Require().NoError(err)

			mpool := s.network.App.GetMempool()
			iterator := mpool.Select(s.network.GetContext(), nil)

			tc.verifyFunc(iterator)
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
					tx, err := s.createEVMTransactionWithNonce(key, big.NewInt(1000000000), i)
					s.Require().NoError(err)
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
					tx, err := s.createEVMTransactionWithNonce(key, big.NewInt(1000000000), i)
					s.Require().NoError(err)
					txs = append(txs, tx)
					nonces = append(nonces, i)
				}

				// Then fill the gap by inserting nonce 1
				tx, err := s.createEVMTransactionWithNonce(key, big.NewInt(1000000000), 1)
				s.Require().NoError(err)
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
					tx, err := s.createEVMTransactionWithNonce(key, big.NewInt(1000000000), i)
					s.Require().NoError(err)
					txs = append(txs, tx)
					nonces = append(nonces, i)
				}

				// Fill gaps by inserting nonces 1, 2, 4, 5, 7, 8
				for i := 1; i <= 8; i++ {
					if i%3 != 0 { // Skip nonces that are already inserted
						tx, err := s.createEVMTransactionWithNonce(key, big.NewInt(1000000000), i)
						s.Require().NoError(err)
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
					tx, err := s.createEVMTransactionWithNonce(key1, big.NewInt(1000000000), i)
					s.Require().NoError(err)
					txs = append(txs, tx)
					nonces = append(nonces, i)
				}

				// Account 2: nonces 0, 3 (gaps at 1, 2)
				for i := 0; i <= 3; i += 3 {
					tx, err := s.createEVMTransactionWithNonce(key2, big.NewInt(1000000000), i)
					s.Require().NoError(err)
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
				tx1, err := s.createEVMTransactionWithNonce(key, big.NewInt(1000000000), 0)
				s.Require().NoError(err)
				txs = append(txs, tx1)
				nonces = append(nonces, 0)

				// Insert transaction with nonce 1
				tx2, err := s.createEVMTransactionWithNonce(key, big.NewInt(1000000000), 1)
				s.Require().NoError(err)
				txs = append(txs, tx2)
				nonces = append(nonces, 1)

				// Replace nonce 0 transaction with higher gas price
				tx3, err := s.createEVMTransactionWithNonce(key, big.NewInt(2000000000), 0)
				s.Require().NoError(err)
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
					tx, err := s.createEVMTransactionWithNonce(key, big.NewInt(1000000000), i)
					s.Require().NoError(err)
					txs = append(txs, tx)
					nonces = append(nonces, i)
				}

				// Fill gaps by inserting nonces 1, 2, 4, 5, 7, 8
				for i := 1; i <= 8; i++ {
					if i%3 != 0 { // Skip nonces that are already inserted
						tx, err := s.createEVMTransactionWithNonce(key, big.NewInt(1000000000), i)
						s.Require().NoError(err)
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

				// Insert transactions with gaps: nonces 0, 1, 3, 4, 6, 7
				for i := 0; i <= 7; i++ {
					if i != 1 { // Skip nonce 1 to create a gap
						tx, err := s.createEVMTransactionWithNonce(key, big.NewInt(1000000000), i)
						s.Require().NoError(err)
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
				tx1, err := s.createEVMTransactionWithNonce(key, big.NewInt(1000000000), 1)
				s.Require().NoError(err)
				err = mpool.Insert(s.network.GetContext(), tx1)
				s.Require().NoError(err)

				// After filling gap: all nonce transactions should be in pending
				countAfterFilling := mpool.CountTx()
				s.Require().Equal(8, countAfterFilling, "After filling gap, only nonce 0 should be pending due to gap at nonce 1")

				// Remove nonce 1 transaction, dropping the rest (except for 0) into queued
				tx1, err = s.createEVMTransactionWithNonce(key, big.NewInt(1000000000), 1)
				s.Require().NoError(err)
				err = mpool.Remove(tx1)
				s.Require().NoError(err)

				// After removal: only nonce 0 should be pending, the rest get dropped to queued
				countAfterRemoval := mpool.CountTx()
				s.Require().Equal(1, countAfterRemoval, "After removing nonce 1, only nonce 0 should be pending")

				// Fill gap by inserting nonce 1
				tx1, err = s.createEVMTransactionWithNonce(key, big.NewInt(1000000000), 1)
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
