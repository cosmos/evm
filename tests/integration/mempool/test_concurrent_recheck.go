package mempool

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"

	evmmempool "github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/testutil/keyring"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// createEVMValueTransferToAddrTx creates an EVM value transfer transaction
// to the specified recipient address.
func (s *IntegrationTestSuite) createEVMValueTransferToAddrTx(key keyring.Key, nonce int, to common.Address, gasPrice *big.Int) sdk.Tx {
	ethTxArgs := evmtypes.EvmTxArgs{
		Nonce:    uint64(nonce),
		To:       &to,
		Amount:   big.NewInt(1000),
		GasLimit: TxGas,
		GasPrice: gasPrice,
	}
	tx, err := s.factory.GenerateSignedEthTx(key.Priv, ethTxArgs)
	s.Require().NoError(err)

	return tx
}

// randomAddress generates a random Ethereum address.
func randomAddress() common.Address {
	var addr common.Address
	_, _ = rand.Read(addr[:])
	return addr
}

// TestConcurrentBlockExecutionMempoolPanic reproduces a production panic where
// the legacypool's background reorg goroutine races with block execution/commit.
//
// The panic manifests as:
//
//	"collections: conflict: index uniqueness constraint violation"
//
// The race path in production:
//  1. NotifyNewBlock → scheduleReorgLoop → runReorg → promoteExecutables →
//     Rechecker.Recheck → VerifyAccountBalance → SetAccount (for new accounts)
//  2. FinalizeBlock + Commit also creates/modifies accounts via SetAccount
//  3. These run concurrently because beginCommitRead is a no-op in non-race builds
//
// With iavlx (IAVL v2), the root cause is that CommitTree.Get() has no
// synchronization with CommitTree.Set(). GetImmutable(latestVersion) returns
// the current working root (c.root) which is being modified by the
// deliverState flush. On ARM64 (Apple Silicon), without memory barriers
// between the writer's Set() and the reader's Get(), the reader can observe
// partially-initialized MemNode structures (e.g., reading a new root pointer
// but seeing stale key/value data in newly-created nodes). This can cause
// Get() to return nil for existing accounts or return corrupted data,
// leading to account number collisions in the collections unique index.
//
// Additionally, the deliverState flush writes keys to each module's
// CommitTree in sorted order via CacheTree.Write(). Between individual
// Set() calls, GetImmutable() can capture an intermediate c.root that
// reflects a partially-flushed block state.
//
// This test reproduces the race by:
//  1. Running background goroutines that continuously call
//     AccountKeeper.NewAccountWithAddress + SetAccount on query contexts
//     (same code path as the rechecker's VerifyAccountBalance)
//  2. Advancing blocks with multiple txs (creating new accounts) WITHOUT
//     the commit lock, widening the flush race window
//  3. Triggering legacypool reorgs via NotifyNewBlock for the production path
func (s *IntegrationTestSuite) TestConcurrentBlockExecutionMempoolPanic() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	baseFee := s.network.App.GetEVMKeeper().GetBaseFee(s.network.GetContext())
	gasPrice := new(big.Int).Mul(baseFee, big.NewInt(10))

	evmMempool := s.network.App.GetMempool().(*evmmempool.ExperimentalEVMMempool)
	blockchain := evmMempool.GetBlockchain()
	ak := s.network.App.GetAccountKeeper()

	// Background goroutines: continuously create new accounts via SetAccount
	// on a CacheContext backed by the root multi-store. This simulates the
	// rechecker's ante handler path (VerifyAccountBalance lines 43-46) which
	// calls NewAccountWithAddress + SetAccount when GetAccount returns nil
	// due to concurrent IAVL modification.
	var (
		done    = make(chan struct{})
		panicCh = make(chan string, 1)
		wg      sync.WaitGroup
		started atomic.Int32
	)

	numWorkers := 8
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					msg := fmt.Sprintf("%v", r)
					select {
					case panicCh <- msg:
					default:
					}
				}
			}()
			started.Add(1)
			for {
				select {
				case <-done:
					return
				default:
				}

				// Get a query context backed by the root IAVL stores.
				// For iavlx, GetImmutable(latestVersion) returns c.root which
				// is the WORKING root being modified by the deliverState flush.
				queryCtx, err := s.network.App.GetBaseApp().CreateQueryContext(0, false)
				if err != nil {
					runtime.Gosched()
					continue
				}
				cacheCtx, _ := queryCtx.CacheContext()

				// Simulate VerifyAccountBalance for a new account:
				//   acc := accountKeeper.NewAccountWithAddress(ctx, from.Bytes())
				//   accountKeeper.SetAccount(ctx, acc)
				// NewAccountWithAddress → NextAccountNumber (reads sequence from IAVL)
				// SetAccount → Accounts.Set → IndexedMap.Set → Unique.Reference
				//   → Has(refKey) reads index from IAVL
				// On ARM64, without memory barriers, the reader can see a new c.root
				// but stale MemNode data, causing Get() to return incorrect results.
				addr := sdk.AccAddress(randomAddress().Bytes())
				acc := ak.NewAccountWithAddress(cacheCtx, addr)
				ak.SetAccount(cacheCtx, acc)
			}
		}()
	}

	// Wait for all workers to start
	for started.Load() < int32(numWorkers) {
		runtime.Gosched()
	}

	// Also trigger legacypool reorgs for realism (the production race path).
	// The reorg goroutine runs the full rechecker flow concurrently.
	mpool := s.network.App.GetMempool()
	for i := range 10 {
		key := s.keyring.GetKey(i)
		for nonce := range 5 {
			newAddr := randomAddress()
			tx := s.createEVMValueTransferToAddrTx(key, nonce, newAddr, gasPrice)
			_ = mpool.Insert(s.network.GetContext(), tx)
		}
	}

	// Advance blocks with multiple txs per block for wider race window.
	// Each tx is a value transfer to a new random address. The EVM execution
	// creates new recipient accounts, causing more Set() calls to the auth
	// module's CommitTree during the deliverState flush. More writes = longer
	// flush duration = wider window for concurrent readers to capture an
	// intermediate c.root.
	nonces := make(map[int]int)
	const txsPerBlock = 5
	for block := range 500 {
		var txBytes [][]byte
		for i := range txsPerBlock {
			senderIdx := 10 + ((block*txsPerBlock + i) % 10)
			nonce := nonces[senderIdx]
			nonces[senderIdx]++

			key := s.keyring.GetKey(senderIdx)
			tx := s.createEVMValueTransferToAddrTx(key, nonce, randomAddress(), gasPrice)
			txBz, err := s.network.App.GetTxConfig().TxEncoder()(tx)
			s.Require().NoError(err)
			txBytes = append(txBytes, txBz)
		}

		// Trigger background reorg (production race path)
		blockchain.NotifyNewBlock()

		// Advance block WITHOUT commit lock.
		// During FinalizeBlock, deliverState.ms.Write() flushes to root stores.
		// Each Set() on the CommitTree updates c.root via COW.
		// During Commit, commitTraverse() modifies MemNode metadata in-place.
		// Both race with background goroutines reading via GetImmutable().
		_, err := s.network.NextBlockWithTxsNoLock(txBytes...)
		if err != nil {
			s.T().Logf("Block %d error (expected during race): %v", block, err)
			break
		}

		// Check if any background goroutine caught a panic
		select {
		case msg := <-panicCh:
			close(done)
			wg.Wait()
			s.T().Logf("Reproduced panic at block %d: %s", block, msg)
			return
		default:
		}
	}

	close(done)
	wg.Wait()

	// Check one final time
	select {
	case msg := <-panicCh:
		s.T().Logf("Reproduced panic: %s", msg)
	default:
		s.T().Log("IAVL data race exists (detectable with -race) but collections panic " +
			"did not manifest in this run. The race is non-deterministic; " +
			"run with -race to reliably detect the underlying data race.")
	}
}
