package mempool

import (
	"encoding/hex"
	"math/big"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/tmhash"

	evmmempool "github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	"github.com/cosmos/evm/testutil/integration/evm/network"

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
				// Cosmos with same effective tip (use different key to avoid address reservation conflict)
				cosmosTx := s.createCosmosSendTx(s.keyring.GetKey(9), big.NewInt(1000000000)) // 1 gaatom/gas effective tip

				// Create transactions with equal effective tips (assuming base fee = 0)
				// EVM: 1000 aatom/gas effective tip
				evmTx := s.createEVMValueTransferDynamicFeeTx(s.keyring.GetKey(0), 0, big.NewInt(1000000000), big.NewInt(1000000000)) // 1 gaatom/gas

				// Input txs in order
				inputTxs := []sdk.Tx{cosmosTx, evmTx}

				// Expected txs in order
				expectedTxs := []sdk.Tx{evmTx, cosmosTx}
				expTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expTxHashes
			},
		},
		{
			name: "mixed transactions with EVM having higher effective tip",
			setupTxs: func() ([]sdk.Tx, []string) {
				// Create Cosmos transaction with lower gas price (use different key to avoid address reservation conflict)
				cosmosTx := s.createCosmosSendTx(s.keyring.GetKey(9), big.NewInt(2000000000)) // 2 gaatom/gas

				// Create EVM transaction with higher gas price
				evmTx := s.createEVMValueTransferDynamicFeeTx(s.keyring.GetKey(0), 0, big.NewInt(5000000000), big.NewInt(5000000000)) // 5 gaatom/gas

				// Input txs in order
				inputTxs := []sdk.Tx{cosmosTx, evmTx}

				// Expected txs in order
				expectedTxs := []sdk.Tx{evmTx, cosmosTx}
				expTxHashes := s.getTxHashes(expectedTxs)

				return inputTxs, expTxHashes
			},
		},
		{
			name: "mixed transactions with Cosmos having higher effective tip",
			setupTxs: func() ([]sdk.Tx, []string) {
				// Create EVM transaction with lower gas price
				evmTx := s.createEVMValueTransferTx(s.keyring.GetKey(0), 0, big.NewInt(2000000000)) // 2000 aatom/gas

				// Create Cosmos transaction with higher gas price (use different key to avoid address reservation conflict)
				cosmosTx := s.createCosmosSendTx(s.keyring.GetKey(9), big.NewInt(5000000000)) // 5000 aatom/gas

				// Input txs in order
				inputTxs := []sdk.Tx{evmTx, cosmosTx}

				// Expected txs in order
				expectedTxs := []sdk.Tx{cosmosTx, evmTx}
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

			// insert txs given by setup
			err := s.insertTxs(txs)
			s.Require().NoError(err)

			// Refresh the cached latestCtx and trigger cosmos recheck so
			// cosmos txs are available via Select/PrepareProposal.
			mpool := s.network.App.GetMempool()
			if kMp, ok := mpool.(*evmmempool.Mempool); ok {
				head := kMp.GetBlockchain().CurrentBlock()
				kMp.RecheckEVMTxs(head)
				kMp.RecheckCosmosTxs(head)
			}

			// Call FinalizeBlock to make finalizeState before calling PrepareProposal
			_, err = s.network.FinalizeBlock()
			s.Require().NoError(err)

			// Call PrepareProposal for the next block (H+1) after recheck at height H.
			// This mirrors production where PrepareProposal is for the next block.
			prepareProposalRes, err := s.network.App.PrepareProposal(&abci.RequestPrepareProposal{
				MaxTxBytes: 1_000_000,
				Height:     s.network.GetContext().BlockHeight() + 1,
			})
			s.Require().NoError(err)

			// Check whether expected transactions are included and returned as pending state in mempool
			ctx := s.network.GetContext()
			if kMp, ok := mpool.(*evmmempool.Mempool); ok {
				head := kMp.GetBlockchain().CurrentBlock()
				kMp.RecheckEVMTxs(head)
				kMp.RecheckCosmosTxs(head)
			}
			iterator := mpool.Select(ctx.WithBlockHeight(ctx.BlockHeight()+1), nil)
			for _, txHash := range expTxHashes {
				actualTxHash := s.getTxHash(iterator.Tx())
				s.Require().Equal(txHash, actualTxHash)

				iterator = iterator.Next()
			}

			// Check whether expected transactions are selected by PrepareProposal
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
		{
			name: "test queue txs do not propagate state changes after failed recheck",
			setupTxs: func() ([]sdk.Tx, []string) {
				key := s.keyring.GetKey(0)
				var txs []sdk.Tx

				balance := network.PrefundedAccountInitialBalance.BigInt()
				gasPrice := new(big.Int).Quo(balance, new(big.Int).SetUint64(2*TxGas))

				// nonce gapped tx that will spend over half of the sender
				// balance
				txs = append(txs, s.createEVMValueTransferTx(key, 1, gasPrice))

				// nonce gapped tx that will spend over half of the sender
				// balance
				txs = append(txs, s.createEVMValueTransferTx(key, 2, gasPrice))

				return txs, s.getTxHashes(nil)
			},
			verifyFunc: func(mpool mempool.Mempool) {
				evmMp := mpool.(*evmmempool.Mempool)
				legacyPool := evmMp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
				pending, queued := legacyPool.Stats()
				s.Require().Equal(0, pending, "expected no txs to be pending")

				// ensure that we do evict either of the txs, they are both
				// nonce gapped which means that they should simply sit in
				// queued until the nonce gap is filled and they are ready to
				// be validated. if we incorrectly propagate the ante writes from
				// rechecking tx1 to tx2, then tx2 will fail recheck with a non
				// tolerated error (using > signers balance) and it will be
				// improperly evicted.
				s.Require().Equal(2, queued, "expected both txs to be queued")
			},
		},
		{
			name: "test queued rechecks happen on pending state",
			setupTxs: func() ([]sdk.Tx, []string) {
				key := s.keyring.GetKey(0)
				balance := network.PrefundedAccountInitialBalance.BigInt()

				var txs []sdk.Tx

				// these txs will go to queued
				txs = append(txs, s.createEVMValueTransferTx(key, 10, big.NewInt(1000000000)))
				txs = append(txs, s.createEVMValueTransferTx(key, 11, big.NewInt(1000000000)))
				txs = append(txs, s.createEVMValueTransferTx(key, 12, big.NewInt(1000000000)))

				// this tx will use the senders entire balance
				gasPrice := new(big.Int).Quo(balance, new(big.Int).SetUint64(TxGas))
				txs = append(txs, s.createEVMValueTransferTxWithValue(key, 0, big.NewInt(0), gasPrice))

				return txs, s.getTxHashes(txs[len(txs)-1:])
			},
			verifyFunc: func(mpool mempool.Mempool) {
				evmMp := mpool.(*evmmempool.Mempool)
				legacyPool := evmMp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
				pending, queued := legacyPool.Stats()
				s.Require().Equal(1, pending, "expected 1 pending tx")

				// not expecting any txs to be in queued since the queued txs
				// are validated on top of pending state. if we take pending
				// state into account, the sender should have no balance when
				// it tries to validate the queued tx, thus it is evicted.
				s.Require().Equal(0, queued, "expected no queued txs")
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Clean up previous test's resources before resetting
			s.TearDownTest()
			s.SetupTest()

			txs, expTxHashes := tc.setupTxs()

			// insert txs given by setup
			err := s.insertTxs(txs)
			s.Require().NoError(err)

			// Refresh the cached latestCtx and trigger cosmos recheck so
			// HeightSync is at the correct height for Select/PrepareProposal.
			mpool := s.network.App.GetMempool()
			if kMp, ok := mpool.(*evmmempool.Mempool); ok {
				head := kMp.GetBlockchain().CurrentBlock()
				kMp.RecheckEVMTxs(head)
				kMp.RecheckCosmosTxs(head)
			}

			// Call FinalizeBlock to make finalizeState before calling PrepareProposal
			_, err = s.network.FinalizeBlock()
			s.Require().NoError(err)

			// Call PrepareProposal for the next block (H+1) after recheck at height H.
			prepareProposalRes, err := s.network.App.PrepareProposal(&abci.RequestPrepareProposal{
				MaxTxBytes: 1_000_000,
				Height:     s.network.GetContext().BlockHeight() + 1,
			})
			s.Require().NoError(err)

			ctx := s.network.GetContext()
			if kMp, ok := mpool.(*evmmempool.Mempool); ok {
				head := kMp.GetBlockchain().CurrentBlock()
				kMp.RecheckEVMTxs(head)
				kMp.RecheckCosmosTxs(head)
			}
			iterator := mpool.Select(ctx.WithBlockHeight(ctx.BlockHeight()+1), nil)

			// Check whether expected transactions are included and returned as pending state in mempool
			for _, txHash := range expTxHashes {
				actualTxHash := s.getTxHash(iterator.Tx())
				s.Require().Equal(txHash, actualTxHash)

				iterator = iterator.Next()
			}
			tc.verifyFunc(mpool)

			// Check whether expected transactions are selected by PrepareProposal
			txHashes := make([]string, 0)
			for _, txBytes := range prepareProposalRes.Txs {
				txHash := hex.EncodeToString(tmhash.Sum(txBytes))
				txHashes = append(txHashes, txHash)
			}
			s.Require().Equal(expTxHashes, txHashes)
		})
	}
}

func (s *IntegrationTestSuite) TestRechecking() {
	testCases := []struct {
		name       string
		setupTxs   func() ([]sdk.Tx, []string)
		networkTxs func() []sdk.Tx
		verifyFunc func(mpool mempool.Mempool)
	}{
		{
			name: "recheck drops evm tx after attempted spend is no longer valid",
			setupTxs: func() ([]sdk.Tx, []string) {
				key := s.keyring.GetKey(0)
				var txs []sdk.Tx

				// create two txs that will each spend half our balance
				balance := network.PrefundedAccountInitialBalance.BigInt()
				gasPrice := big.NewInt(1000000000)
				gasCost := new(big.Int).Mul(big.NewInt(TxGas), gasPrice)
				value := new(big.Int).Sub(new(big.Int).Quo(balance, big.NewInt(2)), gasCost)
				txs = append(txs, s.createEVMValueTransferTxWithValue(key, 0, value, gasPrice))
				txs = append(txs, s.createEVMValueTransferTxWithValue(key, 1, value, gasPrice))

				return txs, s.getTxHashes(nil)
			},
			networkTxs: func() []sdk.Tx {
				// create a new network tx spending just over half balance
				balance := network.PrefundedAccountInitialBalance.BigInt()
				value := new(big.Int).Quo(balance, big.NewInt(2))
				return []sdk.Tx{
					s.createEVMValueTransferTxWithValue(s.keyring.GetKey(0), 0, value, big.NewInt(1000000000)),
				}
			},
			verifyFunc: func(mpool mempool.Mempool) {
				mp := mpool.(*evmmempool.Mempool)
				legacyPool := mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
				pending, queued := legacyPool.Stats()

				// expecting our tx to have been dropped
				s.Require().Equal(0, pending, "expected no pending txs")
				s.Require().Equal(0, queued, "expected no queued txs")
			},
		},
		{
			name: "recheck drops cosmos tx after attempted spend is no longer valid",
			setupTxs: func() ([]sdk.Tx, []string) {
				key := s.keyring.GetKey(0)

				// cosmos tx that spends ~half the balance; becomes invalid
				// once the network tx below also spends half the balance.
				balance := network.PrefundedAccountInitialBalance.BigInt()
				gasPrice := big.NewInt(1000000000)
				gasCost := new(big.Int).Mul(big.NewInt(TxGas), gasPrice)
				value := new(big.Int).Sub(new(big.Int).Quo(balance, big.NewInt(2)), gasCost)

				return []sdk.Tx{s.createCosmosSendTxWithAmount(key, value, gasPrice)}, s.getTxHashes(nil)
			},
			networkTxs: func() []sdk.Tx {
				// create a new network tx spending just over half balance
				balance := network.PrefundedAccountInitialBalance.BigInt()
				value := new(big.Int).Quo(balance, big.NewInt(2))
				return []sdk.Tx{
					s.createEVMValueTransferTxWithValue(s.keyring.GetKey(0), 0, value, big.NewInt(1000000000)),
				}
			},
			verifyFunc: func(mpool mempool.Mempool) {
				mp := mpool.(*evmmempool.Mempool)
				legacyPool := mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
				pending, queued := legacyPool.Stats()

				// the cosmos tx lives in the cosmos pool, not the EVM legacy
				// pool. CountTx returns the sum, so subtracting EVM pending
				// gives the cosmos pool count.
				s.Require().Equal(0, pending, "expected no pending txs")
				s.Require().Equal(0, queued, "expected no queued txs")
				s.Require().Equal(0, mp.CountTx()-pending, "expected no cosmos txs")
			},
		},
		{
			name: "recheck demotes future evm txs to queued after predecessor failure",
			setupTxs: func() ([]sdk.Tx, []string) {
				key := s.keyring.GetKey(0)
				var txs []sdk.Tx

				gasPrice := big.NewInt(1000000000)
				// this is a placeholder tx that will be removed and its
				// changes not included in state since another tx0 will be
				// included in state
				txs = append(txs, s.createEVMValueTransferTx(key, 0, gasPrice))

				// create a tx that will fail recheck due to balance spend when
				// the network tx is included
				balance := network.PrefundedAccountInitialBalance.BigInt()
				gasCost := new(big.Int).Mul(big.NewInt(TxGas), gasPrice)
				value := new(big.Int).Sub(new(big.Int).Quo(balance, big.NewInt(2)), gasCost)
				txs = append(txs, s.createEVMValueTransferTxWithValue(key, 1, value, gasPrice))

				// create a final tx that will spend a minimal amount. this is
				// a valid tx even after the inclusion of the network tx.
				txs = append(txs, s.createEVMValueTransferTx(key, 2, gasPrice))

				return txs, s.getTxHashes(nil)
			},
			networkTxs: func() []sdk.Tx {
				// create a new network tx spending just over half balance
				balance := network.PrefundedAccountInitialBalance.BigInt()
				value := new(big.Int).Quo(balance, big.NewInt(2))
				return []sdk.Tx{
					s.createEVMValueTransferTxWithValue(s.keyring.GetKey(0), 0, value, big.NewInt(1000000000)),
				}
			},
			verifyFunc: func(mpool mempool.Mempool) {
				mp := mpool.(*evmmempool.Mempool)
				legacyPool := mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
				pending, queued := legacyPool.Stats()

				// expecting txs 0 and 1 to have been dropped (tx0 is too old,
				// tx1 spends too much). However tx2 should be demoted to
				// queued, since its parent tx was dropped from pending and its
				// now nonce gapped.
				s.Require().Equal(0, pending, "expected no pending txs")
				s.Require().Equal(1, queued, "expected pending tx to be demoted to queued")
			},
		},
		{
			name: "invalid queue tx stays queue while gapped and dropped once filled",
			setupTxs: func() ([]sdk.Tx, []string) {
				key := s.keyring.GetKey(0)
				var txs []sdk.Tx

				gasPrice := big.NewInt(1000000000)

				// queued tx that will be promoted once the nonce gap is filled
				txs = append(txs, s.createEVMValueTransferTx(key, 1, gasPrice))

				// queued tx that is invalid when taking into account the above
				// tx, and will be dropped only once the above tx is promoted
				balance := network.PrefundedAccountInitialBalance.BigInt()
				gasCost := new(big.Int).Mul(big.NewInt(TxGas), gasPrice)
				value := new(big.Int).Sub(new(big.Int).Quo(balance, big.NewInt(2)), gasCost)
				txs = append(txs, s.createEVMValueTransferTxWithValue(key, 2, value, gasPrice))

				// this tx is valid if we have removed the above tx, this
				// should not be promoted (it will be gapped again once its
				// parent is dropped), but it should stay in queued
				txs = append(txs, s.createEVMValueTransferTx(key, 3, gasPrice))

				return txs, s.getTxHashes([]sdk.Tx{txs[0]})
			},
			networkTxs: func() []sdk.Tx {
				// create a new network tx spending just over half balance
				balance := network.PrefundedAccountInitialBalance.BigInt()
				value := new(big.Int).Quo(balance, big.NewInt(2))
				return []sdk.Tx{
					s.createEVMValueTransferTxWithValue(s.keyring.GetKey(0), 0, value, big.NewInt(1000000000)),
				}
			},
			verifyFunc: func(mpool mempool.Mempool) {
				mp := mpool.(*evmmempool.Mempool)
				legacyPool := mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)

				pending, queued := legacyPool.ContentFrom(s.keyring.GetKey(0).Addr)
				s.Require().Len(pending, 1, "expected only 1 tx to have been promoted to pending")
				s.Require().Len(queued, 1, "expected only 1 tx to have stayed in queued")

				s.Require().Equal(uint64(0x1), pending[0].Nonce(), "expected nonce 1 tx to be in pending")
				s.Require().Equal(uint64(0x3), queued[0].Nonce(), "expected nonce 3 tx to be in queued")
			},
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// clean up previous test's resources before resetting
			s.TearDownTest()
			s.SetupTest()

			txs, expTxHashes := tc.setupTxs()
			s.T().Logf("inserting %d txs into local mempool", len(txs))

			// insert txs given by setup
			err := s.insertTxs(txs)
			s.Require().NoError(err)

			// commit a block with network txs
			networkTxs := tc.networkTxs()
			s.T().Logf("including %d txs from the network", len(networkTxs))

			var toFinalize [][]byte
			for _, tx := range networkTxs {
				encoded, err := s.factory.EncodeTx(tx)
				s.Require().NoError(err)
				toFinalize = append(toFinalize, encoded)
			}
			res, err := s.network.NextBlockWithTxs(toFinalize...)
			s.Require().NoError(err)
			for _, result := range res.GetTxResults() {
				s.Require().Equal(uint32(0x0), result.GetCode())
			}

			// call PrepareProposal for the next block (H+1) after recheck at height H.
			prepareProposalRes, err := s.network.App.PrepareProposal(&abci.RequestPrepareProposal{
				MaxTxBytes: 1_000_000,
				Height:     s.network.GetContext().BlockHeight() + 1,
			})
			s.Require().NoError(err)

			// run custom verify func
			tc.verifyFunc(s.network.App.GetMempool())

			// check whether expected transactions are selected by PrepareProposal
			txHashes := make([]string, 0)
			for _, txBytes := range prepareProposalRes.Txs {
				txHash := hex.EncodeToString(tmhash.Sum(txBytes))
				txHashes = append(txHashes, txHash)
			}
			s.Require().Equal(expTxHashes, txHashes)
		})
	}
}
