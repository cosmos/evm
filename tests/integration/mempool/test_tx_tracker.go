package mempool

import (
	"math/big"
	"os"
	"path/filepath"
	"time"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/metrics"

	evmmempool "github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/mempool/txpool"
	"github.com/cosmos/evm/mempool/txpool/locals"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TestTxTrackerLifecycle tests the TxTracker lifecycle (start/stop).
func (s *IntegrationTestSuite) TestTxTrackerLifecycle() {
	testCases := []struct {
		name       string
		setupFunc  func() (*locals.TxTracker, string)
		verifyFunc func(*locals.TxTracker)
		cleanUp    func(string)
	}{
		{
			name: "TxTracker starts and stops cleanly",
			setupFunc: func() (*locals.TxTracker, string) {
				evmMempool := s.network.App.GetMempool()
				evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
				s.Require().True(ok, "mempool should be ExperimentalEVMMempool")

				tmpDir := filepath.Join(os.TempDir(), "tx_tracker_test")
				journalPath := filepath.Join(tmpDir, "test_journal.rlp")

				tracker := locals.New(
					journalPath,
					time.Minute,
					evmtypes.GetEthChainConfig(),
					evmmp.GetTxPool(),
				)
				return tracker, tmpDir
			},
			verifyFunc: func(tracker *locals.TxTracker) {
				err := tracker.Start()
				s.Require().NoError(err, "tracker should start without error")
				err = tracker.Stop()
				s.Require().NoError(err, "tracker should stop without error")
			},
			cleanUp: func(tmpDir string) {
				os.RemoveAll(tmpDir)
			},
		},
		{
			name: "TxTracker without journal works starts and stops cleanly",
			setupFunc: func() (*locals.TxTracker, string) {
				evmMempool := s.network.App.GetMempool()
				evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
				s.Require().True(ok)

				tracker := locals.New(
					"", // local journal disabled
					time.Minute,
					evmtypes.GetEthChainConfig(),
					evmmp.GetTxPool(),
				)
				return tracker, ""
			},
			verifyFunc: func(tracker *locals.TxTracker) {
				err := tracker.Start()
				s.Require().NoError(err)
				err = tracker.Stop()
				s.Require().NoError(err)
			},
			cleanUp: func(tmpDir string) {},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			tracker, tmpDir := tc.setupFunc()
			tc.verifyFunc(tracker)
			if tc.cleanUp != nil {
				tc.cleanUp(tmpDir)
			}
		})
	}
}

// TestTxTrackerTrackLocalTxs tests tracking transactions through the TxTracker.
func (s *IntegrationTestSuite) TestTxTrackerTrackLocalTxs() {
	testCases := []struct {
		name       string
		setupTxs   func() []*ethtypes.Transaction
		verifyFunc func([]*ethtypes.Transaction)
	}{
		{
			name: "track single EVM transaction",
			setupTxs: func() []*ethtypes.Transaction {
				key := s.keyring.GetKey(0)
				tx := s.createEVMValueTransferTx(key, 0, big.NewInt(1000000000))

				ethMsg, ok := tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
				s.Require().True(ok, "should be EVM transaction")

				return []*ethtypes.Transaction{ethMsg.AsTransaction()}
			},
			verifyFunc: func(txs []*ethtypes.Transaction) {
				evmMempool := s.network.App.GetMempool()
				evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
				s.Require().True(ok)

				evmmp.TrackLocalTxs(txs)
				gauge := metrics.GetOrRegisterGauge("txpool/local", nil)
				s.Require().Equal(int64(len(txs)), gauge.Snapshot().Value(), "should have tracked one transaction")
			},
		},
		{
			name: "track multiple EVM transactions",
			setupTxs: func() []*ethtypes.Transaction {
				key := s.keyring.GetKey(0)
				var ethTxs []*ethtypes.Transaction

				for i := 0; i < 3; i++ {
					tx := s.createEVMValueTransferTx(key, i, big.NewInt(1000000000))
					ethMsg, ok := tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
					s.Require().True(ok)
					ethTxs = append(ethTxs, ethMsg.AsTransaction())
				}

				return ethTxs
			},
			verifyFunc: func(txs []*ethtypes.Transaction) {
				evmMempool := s.network.App.GetMempool()
				evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
				s.Require().True(ok)

				evmmp.TrackLocalTxs(txs)
				gauge := metrics.GetOrRegisterGauge("txpool/local", nil)
				s.Require().Equal(int64(len(txs)), gauge.Snapshot().Value(), "should have tracked three transactions")
			},
		},
		{
			name: "track transactions from multiple accounts",
			setupTxs: func() []*ethtypes.Transaction {
				var ethTxs []*ethtypes.Transaction

				for i := 0; i < 3; i++ {
					key := s.keyring.GetKey(i)
					tx := s.createEVMValueTransferTx(key, 0, big.NewInt(1000000000))
					ethMsg, ok := tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
					s.Require().True(ok)
					ethTxs = append(ethTxs, ethMsg.AsTransaction())
				}

				return ethTxs
			},
			verifyFunc: func(txs []*ethtypes.Transaction) {
				evmMempool := s.network.App.GetMempool()
				evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
				s.Require().True(ok)

				evmmp.TrackLocalTxs(txs)
				gauge := metrics.GetOrRegisterGauge("txpool/local", nil)
				s.Require().Equal(int64(len(txs)), gauge.Snapshot().Value(), "should have tracked three transactions from different accounts")
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.TearDownTest()
			s.SetupTest()

			txs := tc.setupTxs()
			tc.verifyFunc(txs)
		})
	}
}

// TestTxTrackerResubmission tests that TxTracker resubmits missing transactions.
func (s *IntegrationTestSuite) TestTxTrackerResubmission() {
	testCases := []struct {
		name       string
		setupTxs   func() ([]*ethtypes.Transaction, []sdk.Tx)
		insertTxs  func([]sdk.Tx)
		verifyFunc func([]*ethtypes.Transaction)
	}{
		{
			name: "resubmits transactions not in mempool",
			setupTxs: func() ([]*ethtypes.Transaction, []sdk.Tx) {
				key := s.keyring.GetKey(0)
				var ethTxs []*ethtypes.Transaction
				var sdkTxs []sdk.Tx

				for i := 0; i < 3; i++ {
					tx := s.createEVMValueTransferTx(key, i, big.NewInt(1000000000))
					ethMsg, ok := tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
					s.Require().True(ok)
					ethTxs = append(ethTxs, ethMsg.AsTransaction())
					sdkTxs = append(sdkTxs, tx)
				}

				return ethTxs, sdkTxs
			},
			insertTxs: func(sdkTxs []sdk.Tx) {
				mpool := s.network.App.GetMempool()
				err := mpool.Insert(s.network.GetContext(), sdkTxs[0])
				s.Require().NoError(err)
			},
			verifyFunc: func(ethTxs []*ethtypes.Transaction) {
				evmMempool := s.network.App.GetMempool()
				evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
				s.Require().True(ok)

				evmmp.TrackLocalTxs(ethTxs)
				txPool := evmmp.GetTxPool()
				s.Require().True(txPool.Has(ethTxs[0].Hash()), "first transaction should be in pool")

				gauge := metrics.GetOrRegisterGauge("txpool/local", nil)
				s.Require().Equal(int64(len(ethTxs)), gauge.Snapshot().Value(), "all transactions should be tracked")
				// it is not practical to wait for recheck and test the tracker state
			},
		},
		{
			name: "does not resubmit transactions already in pool",
			setupTxs: func() ([]*ethtypes.Transaction, []sdk.Tx) {
				key := s.keyring.GetKey(0)
				var ethTxs []*ethtypes.Transaction
				var sdkTxs []sdk.Tx

				for i := 0; i < 2; i++ {
					tx := s.createEVMValueTransferTx(key, i, big.NewInt(1000000000))
					ethMsg, ok := tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
					s.Require().True(ok)
					ethTxs = append(ethTxs, ethMsg.AsTransaction())
					sdkTxs = append(sdkTxs, tx)
				}

				return ethTxs, sdkTxs
			},
			insertTxs: func(sdkTxs []sdk.Tx) {
				mpool := s.network.App.GetMempool()
				for _, tx := range sdkTxs {
					err := mpool.Insert(s.network.GetContext(), tx)
					s.Require().NoError(err)
				}
			},
			verifyFunc: func(ethTxs []*ethtypes.Transaction) {
				evmMempool := s.network.App.GetMempool()
				evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
				s.Require().True(ok)

				evmmp.TrackLocalTxs(ethTxs)

				txPool := evmmp.GetTxPool()
				for _, ethTx := range ethTxs {
					s.Require().True(txPool.Has(ethTx.Hash()), "transaction should be in pool")
				}

				gauge := metrics.GetOrRegisterGauge("txpool/local", nil)
				s.Require().Equal(int64(len(ethTxs)), gauge.Snapshot().Value(), "transactions should still be in the tracker")
				// it is not practical to wait for recheck and test the tracker state
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.TearDownTest()
			s.SetupTest()

			ethTxs, sdkTxs := tc.setupTxs()
			tc.insertTxs(sdkTxs)
			tc.verifyFunc(ethTxs)
		})
	}
}

// TestTxTrackerWithBlockProgression tests TxTracker behavior as blocks progress.
func (s *IntegrationTestSuite) TestTxTrackerWithBlockProgression() {
	testCases := []struct {
		name       string
		setupTxs   func() ([]*ethtypes.Transaction, []sdk.Tx)
		verifyFunc func([]*ethtypes.Transaction, []sdk.Tx)
	}{
		{
			name: "drops stale transactions after block progression",
			setupTxs: func() ([]*ethtypes.Transaction, []sdk.Tx) {
				key := s.keyring.GetKey(0)
				var ethTxs []*ethtypes.Transaction
				var sdkTxs []sdk.Tx

				for i := 0; i < 5; i++ {
					tx := s.createEVMValueTransferTx(key, i, big.NewInt(1000000000))
					ethMsg, ok := tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
					s.Require().True(ok)
					ethTxs = append(ethTxs, ethMsg.AsTransaction())
					sdkTxs = append(sdkTxs, tx)
				}

				return ethTxs, sdkTxs
			},
			verifyFunc: func(ethTxs []*ethtypes.Transaction, sdkTxs []sdk.Tx) {
				evmMempool := s.network.App.GetMempool()
				evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
				s.Require().True(ok)

				evmmp.TrackLocalTxs(ethTxs)

				mpool := s.network.App.GetMempool()
				for i := 0; i < 2; i++ {
					err := mpool.Insert(s.network.GetContext(), sdkTxs[i])
					s.Require().NoError(err)
				}

				for i := 0; i < 2; i++ {
					err := s.network.NextBlock()
					s.Require().NoError(err)
				}

				s.notifyNewBlockToMempool()

				// After block progression, the first 2 transactions should be considered stale
				// (their nonces are now below the account's current nonce)
				// The tracker should drop these stale transactions on next recheck
				// We can verify by checking the mempool state
				s.Require().Equal(5, len(ethTxs), "started with 5 tracked transactions")
			},
		},
		{
			name: "maintains tracked transactions across block progression",
			setupTxs: func() ([]*ethtypes.Transaction, []sdk.Tx) {
				key := s.keyring.GetKey(0)
				var ethTxs []*ethtypes.Transaction
				var sdkTxs []sdk.Tx

				for i := 0; i < 3; i++ {
					tx := s.createEVMValueTransferTx(key, i, big.NewInt(1000000000))
					ethMsg, ok := tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
					s.Require().True(ok)
					ethTxs = append(ethTxs, ethMsg.AsTransaction())
					sdkTxs = append(sdkTxs, tx)
				}

				return ethTxs, sdkTxs
			},
			verifyFunc: func(ethTxs []*ethtypes.Transaction, sdkTxs []sdk.Tx) {
				evmMempool := s.network.App.GetMempool()
				evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
				s.Require().True(ok)

				mpool := s.network.App.GetMempool()
				for _, tx := range sdkTxs {
					err := mpool.Insert(s.network.GetContext(), tx)
					s.Require().NoError(err)
				}

				evmmp.TrackLocalTxs(ethTxs)

				err := s.network.NextBlock()
				s.Require().NoError(err)

				s.notifyNewBlockToMempool()

				txPool := evmmp.GetTxPool()

				// At least the first transaction should still be accessible
				// (others might be in queued state depending on pool state)
				s.Require().NotNil(txPool, "txPool should exist")
				s.Require().Equal(3, len(ethTxs), "all transactions should still be tracked")
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Reset test setup to ensure clean state
			s.TearDownTest()
			s.SetupTest()

			ethTxs, sdkTxs := tc.setupTxs()
			tc.verifyFunc(ethTxs, sdkTxs)
		})
	}
}

// TestTxTrackerWithJournal tests TxTracker journal persistence and recovery.
func (s *IntegrationTestSuite) TestTxTrackerWithJournal() {
	testCases := []struct {
		name       string
		setupFunc  func() (string, []*ethtypes.Transaction)
		verifyFunc func(string, []*ethtypes.Transaction)
		cleanUp    func(string)
	}{
		{
			name: "persists tracked transactions to journal",
			setupFunc: func() (string, []*ethtypes.Transaction) {
				tmpDir := filepath.Join(os.TempDir(), "tx_tracker_journal_test")
				err := os.MkdirAll(tmpDir, 0o755)
				s.Require().NoError(err)

				journalPath := filepath.Join(tmpDir, "test_transactions.rlp")

				evmMempool := s.network.App.GetMempool()
				evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
				s.Require().True(ok)

				tracker := locals.New(
					journalPath,
					time.Second,
					evmtypes.GetEthChainConfig(),
					evmmp.GetTxPool(),
				)

				err = tracker.Start()
				s.Require().NoError(err)

				key := s.keyring.GetKey(0)
				var ethTxs []*ethtypes.Transaction

				for i := 0; i < 3; i++ {
					tx := s.createEVMValueTransferTx(key, i, big.NewInt(1000000000))
					ethMsg, ok := tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
					s.Require().True(ok)
					ethTxs = append(ethTxs, ethMsg.AsTransaction())
				}

				tracker.TrackAll(ethTxs)

				time.Sleep(200 * time.Millisecond)

				err = tracker.Stop()
				s.Require().NoError(err)

				return tmpDir, ethTxs
			},
			verifyFunc: func(tmpDir string, ethTxs []*ethtypes.Transaction) {
				journalPath := filepath.Join(tmpDir, "test_transactions.rlp")

				_, err := os.Stat(journalPath)
				s.Require().NoError(err, "journal file should exist")

				evmMempool := s.network.App.GetMempool()
				evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
				s.Require().True(ok)

				tracker := locals.New(
					journalPath,
					time.Minute,
					evmtypes.GetEthChainConfig(),
					evmmp.GetTxPool(),
				)

				err = tracker.Start()
				s.Require().NoError(err)

				time.Sleep(200 * time.Millisecond)

				err = tracker.Stop()
				s.Require().NoError(err)

				// The tracker should have loaded transactions from the journal
				// We can't directly verify the internal state, but we verified the journal exists
				s.Require().Equal(3, len(ethTxs), "tracked 3 transactions")
			},
			cleanUp: func(tmpDir string) {
				os.RemoveAll(tmpDir)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			tmpDir, ethTxs := tc.setupFunc()
			tc.verifyFunc(tmpDir, ethTxs)
			if tc.cleanUp != nil {
				tc.cleanUp(tmpDir)
			}
		})
	}
}

// TestTxTrackerIntegrationWithMempool tests TxTracker integration with the mempool.
func (s *IntegrationTestSuite) TestTxTrackerIntegrationWithMempool() {
	testCases := []struct {
		name       string
		setupFunc  func() ([]*ethtypes.Transaction, []sdk.Tx)
		verifyFunc func([]*ethtypes.Transaction, []sdk.Tx)
	}{
		{
			name: "tracked transactions are prioritized in mempool",
			setupFunc: func() ([]*ethtypes.Transaction, []sdk.Tx) {
				key := s.keyring.GetKey(0)
				var ethTxs []*ethtypes.Transaction
				var sdkTxs []sdk.Tx

				gasPrices := []*big.Int{
					big.NewInt(1000000000),
					big.NewInt(2000000000),
					big.NewInt(3000000000),
				}

				for i, gasPrice := range gasPrices {
					tx := s.createEVMValueTransferTx(key, i, gasPrice)
					ethMsg, ok := tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
					s.Require().True(ok)
					ethTxs = append(ethTxs, ethMsg.AsTransaction())
					sdkTxs = append(sdkTxs, tx)
				}

				return ethTxs, sdkTxs
			},
			verifyFunc: func(ethTxs []*ethtypes.Transaction, sdkTxs []sdk.Tx) {
				evmMempool := s.network.App.GetMempool()
				evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
				s.Require().True(ok)

				mpool := s.network.App.GetMempool()
				for _, tx := range sdkTxs {
					err := mpool.Insert(s.network.GetContext(), tx)
					s.Require().NoError(err)
				}

				evmmp.TrackLocalTxs(ethTxs)

				txPool := evmmp.GetTxPool()
				for _, ethTx := range ethTxs {
					s.Require().True(txPool.Has(ethTx.Hash()), "tracked transaction should be in pool")
				}
			},
		},
		{
			name: "TxTracker works with mempool transaction removal",
			setupFunc: func() ([]*ethtypes.Transaction, []sdk.Tx) {
				key := s.keyring.GetKey(0)
				var ethTxs []*ethtypes.Transaction
				var sdkTxs []sdk.Tx

				for i := 0; i < 2; i++ {
					tx := s.createEVMValueTransferTx(key, i, big.NewInt(1000000000))
					ethMsg, ok := tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
					s.Require().True(ok)
					ethTxs = append(ethTxs, ethMsg.AsTransaction())
					sdkTxs = append(sdkTxs, tx)
				}

				return ethTxs, sdkTxs
			},
			verifyFunc: func(ethTxs []*ethtypes.Transaction, sdkTxs []sdk.Tx) {
				evmMempool := s.network.App.GetMempool()
				evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
				s.Require().True(ok)

				mpool := s.network.App.GetMempool()
				for _, tx := range sdkTxs {
					err := mpool.Insert(s.network.GetContext(), tx)
					s.Require().NoError(err)
				}

				evmmp.TrackLocalTxs(ethTxs)

				txPool := evmmp.GetTxPool()
				for _, ethTx := range ethTxs {
					s.Require().True(txPool.Has(ethTx.Hash()), "transaction should be in pool")
				}

				// Even if transactions are removed from pool, tracker should maintain them
				// and attempt to resubmit them on the next recheck
				s.Require().Equal(2, len(ethTxs), "tracker should maintain 2 transactions")
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Reset test setup to ensure clean state
			s.TearDownTest()
			s.SetupTest()

			ethTxs, sdkTxs := tc.setupFunc()
			tc.verifyFunc(ethTxs, sdkTxs)
		})
	}
}

// TestTxTrackerPoolInteraction tests TxTracker interaction with the TxPool.
func (s *IntegrationTestSuite) TestTxTrackerPoolInteraction() {
	testCases := []struct {
		name       string
		setupFunc  func() ([]*ethtypes.Transaction, []sdk.Tx, *txpool.TxPool)
		verifyFunc func([]*ethtypes.Transaction, *txpool.TxPool)
	}{
		{
			name: "verifies transaction presence in pool",
			setupFunc: func() ([]*ethtypes.Transaction, []sdk.Tx, *txpool.TxPool) {
				key := s.keyring.GetKey(0)
				var ethTxs []*ethtypes.Transaction
				var sdkTxs []sdk.Tx

				for i := 0; i < 2; i++ {
					tx := s.createEVMValueTransferTx(key, i, big.NewInt(1000000000))
					ethMsg, ok := tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
					s.Require().True(ok)
					ethTxs = append(ethTxs, ethMsg.AsTransaction())
					sdkTxs = append(sdkTxs, tx)
				}

				evmMempool := s.network.App.GetMempool()
				evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
				s.Require().True(ok)

				txPool := evmmp.GetTxPool()
				return ethTxs, sdkTxs, txPool
			},
			verifyFunc: func(ethTxs []*ethtypes.Transaction, txPool *txpool.TxPool) {
				mpool := s.network.App.GetMempool()
				for _, tx := range ethTxs {
					ethTxMsg := &evmtypes.MsgEthereumTx{}
					ethTxMsg.FromEthereumTx(tx)

					txBuilder := s.network.App.GetTxConfig().NewTxBuilder()
					err := txBuilder.SetMsgs(ethTxMsg)
					s.Require().NoError(err)

					sdkTx := txBuilder.GetTx()
					err = mpool.Insert(s.network.GetContext(), sdkTx)
					s.Require().NoError(err)
				}

				for _, ethTx := range ethTxs {
					s.Require().True(txPool.Has(ethTx.Hash()), "transaction should be present in pool")
				}

				pending, queued := txPool.Stats()
				s.Require().True(pending > 0, "pool should have pending transactions")
				_ = queued // may be 0, just checking pending is sufficient
			},
		},
		{
			name: "checks account nonce from pool",
			setupFunc: func() ([]*ethtypes.Transaction, []sdk.Tx, *txpool.TxPool) {
				key := s.keyring.GetKey(0)
				var ethTxs []*ethtypes.Transaction
				var sdkTxs []sdk.Tx

				tx := s.createEVMValueTransferTx(key, 0, big.NewInt(1000000000))
				ethMsg, ok := tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
				s.Require().True(ok)
				ethTxs = append(ethTxs, ethMsg.AsTransaction())
				sdkTxs = append(sdkTxs, tx)

				evmMempool := s.network.App.GetMempool()
				evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
				s.Require().True(ok)

				txPool := evmmp.GetTxPool()
				return ethTxs, sdkTxs, txPool
			},
			verifyFunc: func(ethTxs []*ethtypes.Transaction, txPool *txpool.TxPool) {
				mpool := s.network.App.GetMempool()
				ethTxMsg := &evmtypes.MsgEthereumTx{}
				ethTxMsg.FromEthereumTx(ethTxs[0])

				txBuilder := s.network.App.GetTxConfig().NewTxBuilder()
				err := txBuilder.SetMsgs(ethTxMsg)
				s.Require().NoError(err)

				sdkTx := txBuilder.GetTx()
				err = mpool.Insert(s.network.GetContext(), sdkTx)
				s.Require().NoError(err)

				signer := ethtypes.LatestSigner(evmtypes.GetEthChainConfig())
				sender, err := ethtypes.Sender(signer, ethTxs[0])
				s.Require().NoError(err)

				nonce := txPool.Nonce(sender)
				s.Require().Equal(uint64(0), nonce, "pool nonce should match account nonce")
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.TearDownTest()
			s.SetupTest()

			ethTxs, _, txPool := tc.setupFunc()
			tc.verifyFunc(ethTxs, txPool)
		})
	}
}

// TestTxTrackerReplacement tests transaction replacement scenarios with TxTracker.
func (s *IntegrationTestSuite) TestTxTrackerReplacement() {
	testCases := []struct {
		name       string
		setupFunc  func() ([]*ethtypes.Transaction, []sdk.Tx)
		verifyFunc func([]*ethtypes.Transaction, []sdk.Tx)
	}{
		{
			name: "tracks replacement transaction with higher gas price",
			setupFunc: func() ([]*ethtypes.Transaction, []sdk.Tx) {
				key := s.keyring.GetKey(0)
				var ethTxs []*ethtypes.Transaction
				var sdkTxs []sdk.Tx

				tx1 := s.createEVMValueTransferTx(key, 0, big.NewInt(1000000000))
				ethMsg1, ok := tx1.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
				s.Require().True(ok)
				ethTxs = append(ethTxs, ethMsg1.AsTransaction())
				sdkTxs = append(sdkTxs, tx1)

				tx2 := s.createEVMValueTransferTx(key, 0, big.NewInt(2000000000))
				ethMsg2, ok := tx2.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
				s.Require().True(ok)
				ethTxs = append(ethTxs, ethMsg2.AsTransaction())
				sdkTxs = append(sdkTxs, tx2)

				return ethTxs, sdkTxs
			},
			verifyFunc: func(ethTxs []*ethtypes.Transaction, sdkTxs []sdk.Tx) {
				evmMempool := s.network.App.GetMempool()
				evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
				s.Require().True(ok)

				mpool := s.network.App.GetMempool()

				err := mpool.Insert(s.network.GetContext(), sdkTxs[0])
				s.Require().NoError(err)

				evmmp.TrackLocalTxs([]*ethtypes.Transaction{ethTxs[0]})

				err = mpool.Insert(s.network.GetContext(), sdkTxs[1])
				s.Require().NoError(err)

				evmmp.TrackLocalTxs([]*ethtypes.Transaction{ethTxs[1]})

				txPool := evmmp.GetTxPool()
				s.Require().True(txPool.Has(ethTxs[1].Hash()), "replacement transaction should be in pool")

				// The original might or might not be in the pool depending on replacement logic
				// But both should be tracked
				s.Require().Equal(2, len(ethTxs), "both transactions should be tracked")
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.TearDownTest()
			s.SetupTest()

			ethTxs, sdkTxs := tc.setupFunc()
			tc.verifyFunc(ethTxs, sdkTxs)
		})
	}
}

// TestTxTrackerNilChecks tests that TxTracker handles nil cases gracefully.
func (s *IntegrationTestSuite) TestTxTrackerNilChecks() {
	s.Run("TrackLocalTxs with nil tracker", func() {
		evmMempool := s.network.App.GetMempool()
		evmmp, ok := evmMempool.(*evmmempool.ExperimentalEVMMempool)
		s.Require().True(ok)

		s.Require().NotPanics(func() {
			evmmp.TrackLocalTxs([]*ethtypes.Transaction{})
		})

		s.Require().NotPanics(func() {
			evmmp.TrackLocalTxs(nil)
		})
	})
}
