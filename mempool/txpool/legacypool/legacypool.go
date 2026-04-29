// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package legacypool implements the normal EVM execution transaction pool.
package legacypool

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math/big"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	cosmoslog "cosmossdk.io/log/v2"
	"github.com/cosmos/evm/mempool/internal/heightsync"
	"github.com/cosmos/evm/mempool/internal/reaplist"
	"github.com/cosmos/evm/mempool/internal/txtracker"
	"github.com/cosmos/evm/mempool/reserver"
	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/prque"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/mempool/txpool"
)

var meter = otel.Meter("github.com/cosmos/evm/mempool/txpool/legacypool")

const (
	// txSlotSize is used to calculate how many data slots a single transaction
	// takes up based on its size. The slots are used as DoS protection, ensuring
	// that validating a new transaction remains a constant operation (in reality
	// O(maxslots), where max slots are 4 currently).
	txSlotSize = 32 * 1024

	// txMaxSize is the maximum size a single transaction can have. This field has
	// non-trivial consequences: larger transactions are significantly harder and
	// more expensive to propagate; larger transactions also take more resources
	// to validate whether they fit into the pool or not.
	txMaxSize = 4 * txSlotSize // 128KB
)

var (
	// ErrTxPoolOverflow is returned if the transaction pool is full and can't accept
	// another remote transaction.
	ErrTxPoolOverflow = errors.New("txpool is full")

	// ErrOutOfOrderTxFromDelegated is returned when the transaction with gapped
	// nonce received from the accounts with delegation or pending delegation.
	ErrOutOfOrderTxFromDelegated = errors.New("gapped-nonce tx from delegated accounts")

	// ErrAuthorityReserved is returned if a transaction has an authorization
	// signed by an address which already has in-flight transactions known to the
	// pool.
	ErrAuthorityReserved = errors.New("authority already reserved")

	// ErrFutureReplacePending is returned if a future transaction replaces a pending
	// one. Future transactions should only be able to replace other future transactions.
	ErrFutureReplacePending = errors.New("future transaction tries to replace pending")
)

var (
	evictionInterval    = time.Minute     // Time interval to check for evictable transactions
	statsReportInterval = 8 * time.Second // Time interval to report transaction pool stats
)

const (
	RemovalReasonLifetime               txpool.RemovalReason = "lifetime"           // Tx has been in queued for too long
	RemovalReasonBelowTip               txpool.RemovalReason = "belowtip"           // Min gas tip changed and these txs are too low
	RemovalReasonTruncatedOverflow      txpool.RemovalReason = "truncated_overflow" // We have to truncate a pool and this account has too many txs
	RemovalReasonTruncatedLast          txpool.RemovalReason = "truncated_last"     // We have to truncate a pool and these txs are the last ones in so they are the first out
	RemovalReasonUnderpricedFull        txpool.RemovalReason = "underpriced_full"   // New tx came in that has a better price. The pool is also full so we kicked a tx out to make room.
	RemovalReasonCapExceeded            txpool.RemovalReason = "capped"             // Too many txs for this account
	RemovalReasonRunTxRecheck           txpool.RemovalReason = "runtx_recheck"
	RemovalReasonRunTxFinalize          txpool.RemovalReason = "runtx_finalize"
	RemovalReasonPrepareProposalInvalid txpool.RemovalReason = "prepare_proposal_invalid"
	RemovalReasonReapEvicted            txpool.RemovalReason = "reap_evicted" // Tx was evicted from the reap list (permanently oversized for any block)
)

var (
	// Specific removal metrics
	// Queue pool
	queuedRemovedLifetime          metric.Int64Counter
	queuedRemovedBelowTip          metric.Int64Counter
	queuedRemovedTruncatedOverflow metric.Int64Counter
	queuedRemovedTruncatedLast     metric.Int64Counter
	queuedRemovedUnderpricedFull   metric.Int64Counter
	queuedRemovedOld               metric.Int64Counter
	queuedRemovedCapped            metric.Int64Counter
	queuedRemovedRunTxRecheck      metric.Int64Counter
	queuedRemovedRunTxFinalize     metric.Int64Counter
	queuedRemovedPrepareProposal   metric.Int64Counter
	queuedRemovedUnknown           metric.Int64Counter
	// Pending pool
	pendingRemovedLifetime          metric.Int64Counter
	pendingRemovedBelowTip          metric.Int64Counter
	pendingRemovedTruncatedOverflow metric.Int64Counter
	pendingRemovedTruncatedLast     metric.Int64Counter
	pendingRemovedUnderpricedFull   metric.Int64Counter
	pendingRemovedOld               metric.Int64Counter
	pendingRemovedCostly            metric.Int64Counter
	pendingRemovedCapped            metric.Int64Counter
	pendingRemovedRunTxRecheck      metric.Int64Counter
	pendingRemovedRunTxFinalize     metric.Int64Counter
	pendingRemovedPrepareProposal   metric.Int64Counter
	pendingRemovedUnknown           metric.Int64Counter

	// Metrics for the pending pool
	pendingDiscardMeter         metric.Int64Counter
	pendingReplaceMeter         metric.Int64Counter
	pendingRateLimitMeter       metric.Int64Counter     // Dropped due to rate limiting
	pendingRecheckDropMeter     metric.Int64Counter     // Dropped due to recheck failing
	pendingRecheckDurationTimer metric.Float64Histogram // How long rechecking txs in the pending pool takes (demoteUnexecutables)
	pendingTruncateTimer        metric.Float64Histogram // How long truncating the pending pool takes
	pendingDemotedRecheck       metric.Int64Counter     // Demoted due to parent tx failing recheck
	pendingDemotedRemoved       metric.Int64Counter     // Demoted due to parent tx being explicitly removed
	pendingDemotedCancelled     metric.Int64Counter     // Demote loop cancelled due to a new block arriving

	// Metrics for queued promotions
	queuedPromotedCancelled metric.Int64Counter // Promote loop cancelled due to a new block arriving

	// Metrics for the queued pool
	queuedDiscardMeter         metric.Int64Counter
	queuedReplaceMeter         metric.Int64Counter
	queuedRateLimitMeter       metric.Int64Counter     // Dropped due to rate limiting
	queuedNofundsMeter         metric.Int64Counter     // Dropped due to out-of-funds
	queuedEvictionMeter        metric.Int64Counter     // Dropped due to lifetime
	queuedRecheckDropMeter     metric.Int64Counter     // Dropped due to antehandler failing
	queuedRecheckDurationTimer metric.Float64Histogram // How long rechecking txs in the queued pool takes (promoteExecutables)
	queuedNonReadies           metric.Int64Counter     // Number of txs that were not ready to be promoted due to a nonce gap during promote loop

	// General tx metrics
	knownTxMeter       metric.Int64Counter
	validTxMeter       metric.Int64Counter
	invalidTxMeter     metric.Int64Counter
	underpricedTxMeter metric.Int64Counter
	overflowedTxMeter  metric.Int64Counter

	// throttleTxMeter counts how many transactions are rejected due to too-many-changes between
	// txpool reorgs.
	throttleTxMeter metric.Int64Counter

	// reorgDurationTimer measures how long time a txpool reorg takes.
	reorgDurationTimer metric.Float64Histogram
	// reorgResetTimer measures how long a txpool reorg takes when it is resetting.
	reorgResetTimer metric.Float64Histogram
	// demoteTimer measures how long demoting transactions in the pending pool takes
	demoteTimer metric.Float64Histogram

	// dropBetweenReorgHistogram counts how many drops we experience between two reorg runs. It is expected
	// that this number is pretty low, since txpool reorgs happen very frequently.
	dropBetweenReorgHistogram metric.Int64Histogram

	pendingGauge metric.Int64UpDownCounter
	queuedGauge  metric.Int64UpDownCounter
	slotsGauge   metric.Int64Gauge

	reheapTimer metric.Float64Histogram
)

func init() {
	counter := func(name, desc string) metric.Int64Counter {
		m, err := meter.Int64Counter(name, metric.WithDescription(desc))
		if err != nil {
			panic(err)
		}
		return m
	}
	timer := func(name, desc string) metric.Float64Histogram {
		m, err := meter.Float64Histogram(name, metric.WithDescription(desc), metric.WithUnit("ms"))
		if err != nil {
			panic(err)
		}
		return m
	}

	queuedRemovedLifetime = counter("txpool.queued.removed.lifetime", "Queued txs removed due to lifetime")
	queuedRemovedBelowTip = counter("txpool.queued.removed.belowtip", "Queued txs removed due to min gas tip change")
	queuedRemovedTruncatedOverflow = counter("txpool.queued.removed.truncated_overflow", "Queued txs removed due to truncation overflow")
	queuedRemovedTruncatedLast = counter("txpool.queued.removed.truncated_last", "Queued txs removed as last-in during truncation")
	queuedRemovedUnderpricedFull = counter("txpool.queued.removed.underpriced_full", "Queued txs removed to make room for better priced tx")
	queuedRemovedOld = counter("txpool.queued.removed.old", "Queued txs removed due to nonce being too old")
	queuedRemovedCapped = counter("txpool.queued.removed.capped", "Queued txs removed due to per-account cap")
	queuedRemovedRunTxRecheck = counter("txpool.queued.removed.runtx_recheck", "Queued txs removed due to RunTx recheck failure")
	queuedRemovedRunTxFinalize = counter("txpool.queued.removed.runtx_finalize", "Queued txs removed due to RunTx finalize")
	queuedRemovedPrepareProposal = counter("txpool.queued.removed.prepare_proposal_invalid", "Queued txs removed as invalid during PrepareProposal")
	queuedRemovedUnknown = counter("txpool.queued.removed.unknown", "Queued txs removed for an unknown reason")

	pendingRemovedLifetime = counter("txpool.pending.removed.lifetime", "Pending txs removed due to lifetime")
	pendingRemovedBelowTip = counter("txpool.pending.removed.belowtip", "Pending txs removed due to min gas tip change")
	pendingRemovedTruncatedOverflow = counter("txpool.pending.removed.truncated_overflow", "Pending txs removed due to truncation overflow")
	pendingRemovedTruncatedLast = counter("txpool.pending.removed.truncated_last", "Pending txs removed as last-in during truncation")
	pendingRemovedUnderpricedFull = counter("txpool.pending.removed.underpriced_full", "Pending txs removed to make room for better priced tx")
	pendingRemovedOld = counter("txpool.pending.removed.old", "Pending txs removed due to nonce being too old")
	pendingRemovedCostly = counter("txpool.pending.removed.costly", "Pending txs removed as too costly")
	pendingRemovedCapped = counter("txpool.pending.removed.capped", "Pending txs removed due to per-account cap")
	pendingRemovedRunTxRecheck = counter("txpool.pending.removed.runtx_recheck", "Pending txs removed due to RunTx recheck failure")
	pendingRemovedRunTxFinalize = counter("txpool.pending.removed.runtx_finalize", "Pending txs removed due to RunTx finalize")
	pendingRemovedPrepareProposal = counter("txpool.pending.removed.prepare_proposal_invalid", "Pending txs removed as invalid during PrepareProposal")
	pendingRemovedUnknown = counter("txpool.pending.removed.unknown", "Pending txs removed for an unknown reason")

	pendingDiscardMeter = counter("txpool.pending.discard", "Pending txs discarded")
	pendingReplaceMeter = counter("txpool.pending.replace", "Pending txs replaced")
	pendingRateLimitMeter = counter("txpool.pending.ratelimit", "Pending txs dropped due to rate limiting")
	pendingRecheckDropMeter = counter("txpool.pending.recheckdrop", "Pending txs dropped due to recheck failing")
	pendingRecheckDurationTimer = timer("txpool.pending.rechecktime", "Time to recheck txs in the pending pool (demoteUnexecutables)")
	pendingTruncateTimer = timer("txpool.pending.truncate", "Time to truncate the pending pool")
	pendingDemotedRecheck = counter("txpool.pending.demoted.recheck", "Pending txs demoted due to parent tx failing recheck")
	pendingDemotedRemoved = counter("txpool.pending.demoted.removed", "Pending txs demoted due to parent tx being explicitly removed")
	pendingDemotedCancelled = counter("txpool.pending.demoted.cancelled", "Demote loop cancelled due to new block arrival")

	queuedPromotedCancelled = counter("txpool.queued.promoted.cancelled", "Promote loop cancelled due to new block arrival")

	queuedDiscardMeter = counter("txpool.queued.discard", "Queued txs discarded")
	queuedReplaceMeter = counter("txpool.queued.replace", "Queued txs replaced")
	queuedRateLimitMeter = counter("txpool.queued.ratelimit", "Queued txs dropped due to rate limiting")
	queuedNofundsMeter = counter("txpool.queued.nofunds", "Queued txs dropped due to out-of-funds")
	queuedEvictionMeter = counter("txpool.queued.eviction", "Queued txs dropped due to lifetime")
	queuedRecheckDropMeter = counter("txpool.queued.recheckdrop", "Queued txs dropped due to antehandler failure")
	queuedRecheckDurationTimer = timer("txpool.queued.rechecktime", "Time to recheck txs in the queued pool (promoteExecutables)")
	queuedNonReadies = counter("txpool.queued.notready", "Queued txs not ready for promotion due to nonce gap")

	knownTxMeter = counter("txpool.known", "Txs discarded as already known")
	validTxMeter = counter("txpool.valid", "Txs deemed valid by the pool")
	invalidTxMeter = counter("txpool.invalid", "Txs discarded as invalid")
	underpricedTxMeter = counter("txpool.underpriced", "Txs discarded as underpriced")
	overflowedTxMeter = counter("txpool.overflowed", "Txs discarded due to pool overflow")

	throttleTxMeter = counter("txpool.throttle", "Txs rejected due to too-many-changes between reorgs")

	reorgDurationTimer = timer("txpool.reorgtime", "Time taken to run a txpool reorg")
	reorgResetTimer = timer("txpool.resettime", "Time taken to reset state during a txpool reorg")
	demoteTimer = timer("txpool.demotetime", "Time taken to demote txs in the pending pool")

	var err error
	dropBetweenReorgHistogram, err = meter.Int64Histogram(
		"txpool.dropbetweenreorg",
		metric.WithDescription("Number of drops observed between two reorg runs"),
	)
	if err != nil {
		panic(err)
	}

	pendingGauge, err = meter.Int64UpDownCounter(
		"txpool.pending",
		metric.WithDescription("Current number of txs in the pending pool"),
	)
	if err != nil {
		panic(err)
	}
	queuedGauge, err = meter.Int64UpDownCounter(
		"txpool.queued",
		metric.WithDescription("Current number of txs in the queued pool"),
	)
	if err != nil {
		panic(err)
	}
	slotsGauge, err = meter.Int64Gauge(
		"txpool.slots",
		metric.WithDescription("Current number of slots in use by tracked txs"),
	)
	if err != nil {
		panic(err)
	}

	reheapTimer = timer("txpool.reheap", "Time taken to reheap the priced list")
}

type PoolType string

const (
	Pending PoolType = "pending"
	Queue   PoolType = "queue"
)

// BlockChain defines the minimal set of methods needed to back a tx pool with
// a chain. Exists to allow mocking the live chain out of tests.
type BlockChain interface {
	// Config retrieves the chain's fork configuration.
	Config() *params.ChainConfig

	// CurrentBlock returns the current head of the chain.
	CurrentBlock() *types.Header

	// GetBlock retrieves a specific block, used during pool resets.
	GetBlock(hash common.Hash, number uint64) *types.Block

	// StateAt returns a state database for a given root hash (generally the head).
	StateAt(root common.Hash) (vm.StateDB, error)

	// GetLatestContext returns the latest SDK context for the chain.
	GetLatestContext() (sdk.Context, error)
}

// Rechecker defines the minimal set of methods needed to recheck transactions
// and manage the context that the transactions are rechecked against.
type Rechecker interface {
	// GetContext gets a branch of the current context that transactions should
	// be rechecked against. Changes to ctx will only be persisted back to the
	// Reckecker once the write function is invoked.
	GetContext() (ctx sdk.Context, write func())

	// RecheckEVM performs validation of an EVM tx against a context, and
	// returns an updated context.
	RecheckEVM(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error)

	// Update updates the main context returned by GetContext to be the base
	// chain context at header. The caller provides the SDK context directly.
	Update(ctx sdk.Context, header *types.Header)
}

// Config are the configuration parameters of the transaction pool.
type Config struct {
	Locals    []common.Address // Addresses that should be treated by default as local
	NoLocals  bool             // Whether local transaction handling should be disabled
	Journal   string           // Journal of local transactions to survive node restarts
	Rejournal time.Duration    // Time interval to regenerate the local transaction journal

	PriceLimit uint64 // Minimum gas price to enforce for acceptance into the pool
	PriceBump  uint64 // Minimum price bump percentage to replace an already existing transaction (nonce)

	AccountSlots uint64 // Number of executable transaction slots guaranteed per account
	GlobalSlots  uint64 // Maximum number of executable transaction slots for all accounts
	AccountQueue uint64 // Maximum number of non-executable transaction slots permitted per account
	GlobalQueue  uint64 // Maximum number of non-executable transaction slots for all accounts

	Lifetime time.Duration // Maximum amount of time non-executable transaction are queued

	IncludedNonceCacheSize int // Max entries in the included nonce LRU cache
}

// DefaultConfig contains the default configurations for the transaction pool.
var DefaultConfig = Config{
	Journal:   "transactions.rlp",
	Rejournal: time.Hour,

	PriceLimit: 1,
	PriceBump:  10,

	AccountSlots: 16,
	GlobalSlots:  4096 + 1024, // urgent + floating queue capacity with 4:1 ratio
	AccountQueue: 64,
	GlobalQueue:  1024,

	Lifetime: 3 * time.Hour,

	IncludedNonceCacheSize: 4096, // should be >= max txs expected in a block for best perf
}

// sanitize checks the provided user configurations and changes anything that's
// unreasonable or unworkable.
func (config *Config) sanitize() Config {
	conf := *config
	if conf.PriceLimit < 1 {
		log.Warn("Sanitizing invalid txpool price limit", "provided", conf.PriceLimit, "updated", DefaultConfig.PriceLimit)
		conf.PriceLimit = DefaultConfig.PriceLimit
	}
	if conf.PriceBump < 1 {
		log.Warn("Sanitizing invalid txpool price bump", "provided", conf.PriceBump, "updated", DefaultConfig.PriceBump)
		conf.PriceBump = DefaultConfig.PriceBump
	}
	if conf.AccountSlots < 1 {
		log.Warn("Sanitizing invalid txpool account slots", "provided", conf.AccountSlots, "updated", DefaultConfig.AccountSlots)
		conf.AccountSlots = DefaultConfig.AccountSlots
	}
	if conf.GlobalSlots < 1 {
		log.Warn("Sanitizing invalid txpool global slots", "provided", conf.GlobalSlots, "updated", DefaultConfig.GlobalSlots)
		conf.GlobalSlots = DefaultConfig.GlobalSlots
	}
	if conf.AccountQueue < 1 {
		log.Warn("Sanitizing invalid txpool account queue", "provided", conf.AccountQueue, "updated", DefaultConfig.AccountQueue)
		conf.AccountQueue = DefaultConfig.AccountQueue
	}
	if conf.GlobalQueue < 1 {
		log.Warn("Sanitizing invalid txpool global queue", "provided", conf.GlobalQueue, "updated", DefaultConfig.GlobalQueue)
		conf.GlobalQueue = DefaultConfig.GlobalQueue
	}
	if conf.Lifetime < 1 {
		log.Warn("Sanitizing invalid txpool lifetime", "provided", conf.Lifetime, "updated", DefaultConfig.Lifetime)
		conf.Lifetime = DefaultConfig.Lifetime
	}
	if conf.IncludedNonceCacheSize < 1 {
		log.Warn("Sanitizing invalid txpool included nonce cache size", "provided", conf.IncludedNonceCacheSize, "updated", DefaultConfig.IncludedNonceCacheSize)
		conf.IncludedNonceCacheSize = DefaultConfig.IncludedNonceCacheSize
	}
	return conf
}

// LegacyPool contains all currently known transactions. Transactions
// enter the pool when they are received from the network or submitted
// locally. They exit the pool when they are included in the blockchain.
//
// The pool separates processable transactions (which can be applied to the
// current state) and future transactions. Transactions move between those
// two states over time as they are received and processed.
//
// In addition to tracking transactions, the pool also tracks a set of pending SetCode
// authorizations (EIP7702). This helps minimize number of transactions that can be
// trivially churned in the pool. As a standard rule, any account with a deployed
// delegation or an in-flight authorization to deploy a delegation will only be allowed a
// single transaction slot instead of the standard number. This is due to the possibility
// of the account being sweeped by an unrelated account.
//
// Because SetCode transactions can have many authorizations included, we avoid explicitly
// checking their validity to save the state lookup. So long as the encompassing
// transaction is valid, the authorization will be accepted and tracked by the pool. In
// case the pool is tracking a pending / queued transaction from a specific account, it
// will reject new transactions with delegations from that account with standard in-flight
// transactions.
type LegacyPool struct {
	config      Config
	chainconfig *params.ChainConfig
	chain       BlockChain
	gasTip      atomic.Pointer[uint256.Int]
	txFeed      event.Feed
	signer      types.Signer
	mu          sync.RWMutex

	currentHead         atomic.Pointer[types.Header]       // Current head of the blockchain
	currentState        vm.StateDB                         // Current state in the blockchain head
	pendingNonces       *noncer                            // Pending state tracking virtual nonces
	latestIncludedNonce *lru.Cache[common.Address, uint64] // Cache of latest nonce seen executed on chain for accounts
	reserver            reserver.Reserver                  // Address reserver to ensure exclusivity across subpools
	rechecker           Rechecker                          // Checks a tx for validity against the current state

	validPendingTxs *heightsync.HeightSync[TxStore] // Per height store of pending txs that have been validated

	pending map[common.Address]*list     // All currently processable transactions
	queue   map[common.Address]*list     // Queued but non-processable transactions
	beats   map[common.Address]time.Time // Last heartbeat from each known account
	all     *lookup                      // All transactions to allow lookups
	priced  *pricedList                  // All transactions sorted by price

	reqResetCh       chan *txpoolResetRequest
	reqPromoteCh     chan *accountSet
	reqCancelResetCh chan struct{}
	queueTxEventCh   chan *types.Transaction
	reorgDoneCh      chan chan struct{}
	reorgShutdownCh  chan struct{}  // requests shutdown of scheduleReorgLoop
	wg               sync.WaitGroup // tracks loop, scheduleReorgLoop
	initDoneCh       chan struct{}  // is closed once the pool is initialized (for tests)

	changesSinceReorg int // A counter for how many drops we've performed in-between reorg.

	reapList *reaplist.ReapList   // Queue of txs to be reaped by comet and gossiped
	tracker  txtracker.Tracker    // Track tx lifecycle events for metrics
}

type txpoolResetRequest struct {
	oldHead, newHead *types.Header
}

// Option is a function that sets an optional parameter on the legacypool
type Option func(pool *LegacyPool)

// WithRecheck enables recheck evicting of transactions from the mempool.
func WithRecheck(rechecker Rechecker) Option {
	return func(pool *LegacyPool) {
		pool.rechecker = rechecker
	}
}

// New creates a new transaction pool to gather, sort and filter inbound
// transactions from the network. reapList and tracker are required: they
// observe every pending-pool transition and are load-bearing for both
// broadcast (reapList) and telemetry (tracker).
func New(
	config Config,
	logger cosmoslog.Logger,
	chain BlockChain,
	reapList *reaplist.ReapList,
	tracker txtracker.Tracker,
	opts ...Option,
) *LegacyPool {
	if reapList == nil {
		panic("legacypool: reapList must not be nil")
	}
	if tracker == nil {
		panic("legacypool: tracker must not be nil")
	}
	// Sanitize the input to ensure no vulnerable gas prices are set
	config = (&config).sanitize()

	// only errors if size <= 0 and we have already validated this
	nonceCache, _ := lru.New[common.Address, uint64](config.IncludedNonceCacheSize)

	// Create the transaction pool with its initial settings
	pool := &LegacyPool{
		config:              config,
		chain:               chain,
		chainconfig:         chain.Config(),
		signer:              types.LatestSigner(chain.Config()),
		pending:             make(map[common.Address]*list),
		queue:               make(map[common.Address]*list),
		beats:               make(map[common.Address]time.Time),
		all:                 newLookup(),
		rechecker:           newNopRechecker(),
		reapList:            reapList,
		tracker:             tracker,
		validPendingTxs:     heightsync.New(chain.CurrentBlock().Number, NewTxStore, logger.With("pool", "legacypool")),
		reqResetCh:          make(chan *txpoolResetRequest),
		reqPromoteCh:        make(chan *accountSet),
		reqCancelResetCh:    make(chan struct{}),
		queueTxEventCh:      make(chan *types.Transaction),
		reorgDoneCh:         make(chan chan struct{}),
		reorgShutdownCh:     make(chan struct{}),
		initDoneCh:          make(chan struct{}),
		latestIncludedNonce: nonceCache,
	}
	pool.priced = newPricedList(pool.all)

	for _, opt := range opts {
		opt(pool)
	}

	return pool
}

// SetLatestNonce records the latest on chain nonce observed for an account.
func (pool *LegacyPool) SetLatestNonce(sender common.Address, nonce uint64) {
	existing, ok := pool.latestIncludedNonce.Get(sender)
	if !ok || nonce > existing {
		pool.latestIncludedNonce.Add(sender, nonce)
	}
}

// LatestNonce returns the most recently recorded latest-included nonce for
// addr and whether an entry exists in the cache. Primarily useful for tests
// and debugging.
func (pool *LegacyPool) LatestNonce(addr common.Address) (uint64, bool) {
	return pool.latestIncludedNonce.Get(addr)
}

// removeOlds removes txs that have been scheduled for removals from
// list l for sender addr. Returns the txs successfully removed.
func (pool *LegacyPool) removeOlds(addr common.Address, l *list, poolType PoolType) types.Transactions {
	latest, ok := pool.latestIncludedNonce.Get(addr)
	if !ok {
		return nil
	}

	dropped := l.Forward(latest + 1)
	for _, tx := range dropped {
		pool.all.Remove(tx.Hash())
		pool.markTxRemoved(addr, tx, poolType)
	}

	return dropped
}

// Filter returns whether the given transaction can be consumed by the legacy
// pool, specifically, whether it is a Legacy, AccessList or Dynamic transaction.
func (pool *LegacyPool) Filter(tx *types.Transaction) bool {
	switch tx.Type() {
	case types.LegacyTxType, types.AccessListTxType, types.DynamicFeeTxType, types.SetCodeTxType:
		return true
	default:
		return false
	}
}

// Init sets the gas price needed to keep a transaction in the pool and the chain
// head to allow balance / nonce checks. The internal
// goroutines will be spun up and the pool deemed operational afterwards.
func (pool *LegacyPool) Init(gasTip uint64, head *types.Header, reserver reserver.Reserver) error {
	// Set the address reserver to request exclusive access to pooled accounts
	pool.reserver = reserver

	// Set the basic pool parameters
	pool.gasTip.Store(uint256.NewInt(gasTip))

	// Initialize the state with head block, or fallback to empty one in
	// case the head state is not available (might occur when node is not
	// fully synced).
	statedb, err := pool.chain.StateAt(head.Root)
	if err != nil {
		statedb, err = pool.chain.StateAt(types.EmptyRootHash)
	}
	if err != nil {
		return err
	}
	pool.currentHead.Store(head)
	pool.currentState = statedb
	pool.pendingNonces = newNoncer(statedb)

	pool.wg.Add(1)
	go pool.scheduleReorgLoop()

	pool.wg.Add(1)
	go pool.loop()
	return nil
}

// loop is the transaction pool's main event loop, waiting for and reacting to
// outside blockchain events as well as for various reporting and transaction
// eviction events.
func (pool *LegacyPool) loop() {
	defer pool.wg.Done()

	var (
		prevPending, prevQueued, prevStales int

		// Start the stats reporting and transaction eviction tickers
		report = time.NewTicker(statsReportInterval)
		evict  = time.NewTicker(evictionInterval)
	)
	defer report.Stop()
	defer evict.Stop()

	// Notify tests that the init phase is done
	close(pool.initDoneCh)
	for {
		select {
		// Handle pool shutdown
		case <-pool.reorgShutdownCh:
			return

		// Handle stats reporting ticks
		case <-report.C:
			pool.mu.RLock()
			pending, queued := pool.stats()
			pool.mu.RUnlock()
			stales := int(pool.priced.stales.Load())

			if pending != prevPending || queued != prevQueued || stales != prevStales {
				log.Debug("Transaction pool status report", "executable", pending, "queued", queued, "stales", stales)
				prevPending, prevQueued, prevStales = pending, queued, stales
			}

		// Handle inactive account transaction eviction
		case <-evict.C:
			pool.mu.Lock()
			for addr := range pool.queue {
				// Any old enough should be removed
				if time.Since(pool.beats[addr]) > pool.config.Lifetime {
					list := pool.queue[addr].Flatten()
					for _, tx := range list {
						pool.removeTx(tx.Hash(), true, true, RemovalReasonLifetime)
					}
					queuedEvictionMeter.Add(context.Background(), int64(len(list)))
				}
			}
			pool.mu.Unlock()
		}
	}
}

// Close terminates the transaction pool.
func (pool *LegacyPool) Close() error {
	// Terminate the pool reorger and return
	close(pool.reorgShutdownCh)
	pool.wg.Wait()

	log.Info("Transaction pool stopped")

	return nil
}

// Reset implements txpool.SubPool, allowing the legacy pool's internal state to be
// kept in sync with the main transaction pool's internal state.
func (pool *LegacyPool) Reset(oldHead, newHead *types.Header) {
	wait := pool.requestReset(oldHead, newHead)
	<-wait
}

// CancelReset implements txpool.SubPool, signals the legacypool to stop
// processing its current reset request since a new block arrived and the work
// it is doing to reset at the current height will be invalidated.
func (pool *LegacyPool) CancelReset() {
	select {
	case pool.reqCancelResetCh <- struct{}{}:
		return
	case <-pool.reorgShutdownCh:
		return
	}
}

// SubscribeTransactions registers a subscription for new transaction events,
// supporting feeding only newly seen or also resurrected transactions.
func (pool *LegacyPool) SubscribeTransactions(ch chan<- core.NewTxsEvent, reorgs bool) event.Subscription {
	// The legacy pool has a very messed up internal shuffling, so it's kind of
	// hard to separate newly discovered transaction from resurrected ones. This
	// is because the new txs are added to the queue, resurrected ones too and
	// reorgs run lazily, so separating the two would need a marker.
	return pool.txFeed.Subscribe(ch)
}

// SetGasTip updates the minimum gas tip required by the transaction pool for a
// new transaction, and drops all transactions below this threshold.
func (pool *LegacyPool) SetGasTip(tip *big.Int) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	var (
		newTip = uint256.MustFromBig(tip)
		old    = pool.gasTip.Load()
	)
	pool.gasTip.Store(newTip)
	// If the min miner fee increased, remove transactions below the new threshold
	if newTip.Cmp(old) > 0 {
		// pool.priced is sorted by GasFeeCap, so we have to iterate through pool.all instead
		drop := pool.all.TxsBelowTip(tip)
		for _, tx := range drop {
			pool.removeTx(tx.Hash(), false, true, RemovalReasonBelowTip)
		}
		pool.priced.Removed(len(drop))
	}
	log.Info("Legacy pool tip threshold updated", "tip", newTip)
}

// Nonce returns the next nonce of an account, with all transactions executable
// by the pool already applied on top.
func (pool *LegacyPool) Nonce(addr common.Address) uint64 {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.pendingNonces.get(addr)
}

// Stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) transactions.
func (pool *LegacyPool) Stats() (int, int) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.stats()
}

// stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) transactions.
func (pool *LegacyPool) stats() (int, int) {
	pending := 0
	for _, list := range pool.pending {
		pending += list.Len()
	}
	queued := 0
	for _, list := range pool.queue {
		queued += list.Len()
	}
	return pending, queued
}

// Content retrieves the data content of the transaction pool, returning all the
// pending as well as queued transactions, grouped by account and sorted by nonce.
func (pool *LegacyPool) Content() (map[common.Address][]*types.Transaction, map[common.Address][]*types.Transaction) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[common.Address][]*types.Transaction, len(pool.pending))
	for addr, list := range pool.pending {
		pending[addr] = list.Flatten()
	}
	queued := make(map[common.Address][]*types.Transaction, len(pool.queue))
	for addr, list := range pool.queue {
		queued[addr] = list.Flatten()
	}
	return pending, queued
}

// ContentFrom retrieves the data content of the transaction pool, returning the
// pending as well as queued transactions of this address, grouped by nonce.
func (pool *LegacyPool) ContentFrom(addr common.Address) ([]*types.Transaction, []*types.Transaction) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	var pending []*types.Transaction
	if list, ok := pool.pending[addr]; ok {
		pending = list.Flatten()
	}
	var queued []*types.Transaction
	if list, ok := pool.queue[addr]; ok {
		queued = list.Flatten()
	}
	return pending, queued
}

// Rechecked retrieves all currently rechecked transactions, grouped by origin
// account and sorted by nonce.
//
// The transactions can also be pre-filtered by the dynamic fee components to
// reduce allocations and load on downstream subsystems.
func (pool *LegacyPool) Rechecked(ctx context.Context, height *big.Int, filter txpool.PendingFilter) map[common.Address][]*txpool.LazyTransaction {
	txStore := pool.validPendingTxs.GetStore(ctx, height)
	if txStore == nil {
		return nil
	}
	return txStore.Txs(ctx, filter)
}

// Pending retrieves all currently processable transactions, grouped by origin
// account and sorted by nonce.
//
// The transactions can also be pre-filtered by the dynamic fee components to
// reduce allocations and load on downstream subsystems.
func (pool *LegacyPool) Pending(ctx context.Context, filter txpool.PendingFilter) map[common.Address][]*txpool.LazyTransaction {
	// If only blob transactions are requested, this pool is unsuitable as it
	// contains none, don't even bother.
	if filter.OnlyBlobTxs {
		return nil
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[common.Address][]*txpool.LazyTransaction, len(pool.pending))
	for addr, list := range pool.pending {
		if lazies := filterAndWrapTxs(list.Flatten(), filter.MinTip, filter.BaseFee); len(lazies) > 0 {
			pending[addr] = lazies
		}
	}
	return pending
}

// filterAndWrapTxs applies tip filtering to txs and wraps the survivors into
// LazyTransactions.
func filterAndWrapTxs(txs []*types.Transaction, minTip, baseFee *uint256.Int) []*txpool.LazyTransaction {
	if minTip != nil {
		for i, tx := range txs {
			if tx.EffectiveGasTipIntCmp(minTip, baseFee) < 0 {
				txs = txs[:i]
				break
			}
		}
	}
	if len(txs) == 0 {
		return nil
	}
	lazies := make([]*txpool.LazyTransaction, len(txs))
	for i, tx := range txs {
		lazies[i] = &txpool.LazyTransaction{
			Hash:      tx.Hash(),
			Tx:        tx,
			Time:      tx.Time(),
			GasFeeCap: uint256.MustFromBig(tx.GasFeeCap()),
			GasTipCap: uint256.MustFromBig(tx.GasTipCap()),
			Gas:       tx.Gas(),
			BlobGas:   tx.BlobGas(),
		}
	}
	return lazies
}

// ValidateTxBasics checks whether a transaction is valid according to the consensus
// rules, but does not check state-dependent validation such as sufficient balance.
// This check is meant as an early check which only needs to be performed once,
// and does not require the pool mutex to be held.
func (pool *LegacyPool) ValidateTxBasics(tx *types.Transaction) error {
	opts := &txpool.ValidationOptions{
		Config: pool.chainconfig,
		Accept: 0 |
			1<<types.LegacyTxType |
			1<<types.AccessListTxType |
			1<<types.DynamicFeeTxType |
			1<<types.SetCodeTxType,
		MaxSize: txMaxSize,
		MinTip:  pool.gasTip.Load().ToBig(),
	}
	return txpool.ValidateTransaction(tx, pool.currentHead.Load(), pool.signer, opts)
}

// validateTx checks whether a transaction is valid according to the consensus
// rules and adheres to some heuristic limits of the local node (price and size).
func (pool *LegacyPool) validateTx(tx *types.Transaction) error {
	opts := &txpool.ValidationOptionsWithState{
		State: pool.currentState,

		FirstNonceGap:    nil, // Pool allows arbitrary arrival order, don't invalidate nonce gaps
		UsedAndLeftSlots: nil, // Pool has own mechanism to limit the number of transactions
		ExistingExpenditure: func(addr common.Address) *big.Int {
			if list := pool.pending[addr]; list != nil {
				return list.totalcost.ToBig()
			}
			return new(big.Int)
		},
		ExistingCost: func(addr common.Address, nonce uint64) *big.Int {
			if list := pool.pending[addr]; list != nil {
				if tx := list.txs.Get(nonce); tx != nil {
					return tx.Cost()
				}
			}
			return nil
		},
	}
	if err := txpool.ValidateTransactionWithState(tx, pool.signer, opts); err != nil {
		return err
	}
	return pool.validateAuth(tx)
}

// checkDelegationLimit determines if the tx sender is delegated or has a
// pending delegation, and if so, ensures they have at most one in-flight
// **executable** transaction, e.g. disallow stacked and gapped transactions
// from the account.
func (pool *LegacyPool) checkDelegationLimit(tx *types.Transaction) error {
	from, _ := types.Sender(pool.signer, tx) // validated

	// Short circuit if the sender has neither delegation nor pending delegation.
	if pool.currentState.GetCodeHash(from) == types.EmptyCodeHash && !pool.all.hasAuth(from) {
		return nil
	}
	pending := pool.pending[from]
	if pending == nil {
		// Transaction with gapped nonce is not supported for delegated accounts
		if pool.pendingNonces.get(from) != tx.Nonce() {
			return ErrOutOfOrderTxFromDelegated
		}
		return nil
	}
	// Transaction replacement is supported
	if pending.Contains(tx.Nonce()) {
		return nil
	}
	return txpool.ErrInflightTxLimitReached
}

// validateAuth verifies that the transaction complies with code authorization
// restrictions brought by SetCode transaction type.
func (pool *LegacyPool) validateAuth(tx *types.Transaction) error {
	// Allow at most one in-flight tx for delegated accounts or those with a
	// pending authorization.
	if err := pool.checkDelegationLimit(tx); err != nil {
		return err
	}
	// For symmetry, allow at most one in-flight tx for any authority with a
	// pending transaction.
	if auths := tx.SetCodeAuthorities(); len(auths) > 0 {
		for _, auth := range auths {
			var count int
			if pending := pool.pending[auth]; pending != nil {
				count += pending.Len()
			}
			if queue := pool.queue[auth]; queue != nil {
				count += queue.Len()
			}
			if count > 1 {
				return ErrAuthorityReserved
			}
			// Because there is no exclusive lock held between different subpools
			// when processing transactions, the SetCode transaction may be accepted
			// while other transactions with the same sender address are also
			// accepted simultaneously in the other pools.
			//
			// This scenario is considered acceptable, as the rule primarily ensures
			// that attackers cannot easily stack a SetCode transaction when the sender
			// is reserved by other pools.
			if pool.reserver.Has(auth) {
				return ErrAuthorityReserved
			}
		}
	}
	return nil
}

// add validates a transaction and inserts it into the non-executable queue for later
// pending promotion and execution. If the transaction is a replacement for an already
// pending or queued one, it overwrites the previous transaction if its price is higher.
func (pool *LegacyPool) add(tx *types.Transaction) (replaced bool, err error) {
	// If the transaction is already known, discard it
	hash := tx.Hash()
	if pool.all.Get(hash) != nil {
		log.Trace("Discarding already known transaction", "hash", hash)
		knownTxMeter.Add(context.Background(), 1)
		return false, txpool.ErrAlreadyKnown
	}

	// If the transaction fails basic validation, discard it
	if err := pool.validateTx(tx); err != nil {
		log.Trace("Discarding invalid transaction", "hash", hash, "err", err)
		invalidTxMeter.Add(context.Background(), 1)
		return false, err
	}
	// already validated by this point
	from, _ := types.Sender(pool.signer, tx)

	// If the address is not yet known, request exclusivity to track the account
	// only by this subpool until all transactions are evicted
	var (
		_, hasPending = pool.pending[from]
		_, hasQueued  = pool.queue[from]
	)
	if !hasPending && !hasQueued {
		if err := pool.reserver.Hold(from); err != nil {
			return false, err
		}
		defer func() {
			// If the transaction is rejected by some post-validation check, remove
			// the lock on the reservation set.
			//
			// Note, `err` here is the named error return, which will be initialized
			// by a return statement before running deferred methods. Take care with
			// removing or subscoping err as it will break this clause.
			if err != nil {
				pool.reserver.Release(from)
			}
		}()
	}
	// If the transaction pool is full, discard underpriced transactions
	if uint64(pool.all.Slots()+numSlots(tx)) > pool.config.GlobalSlots+pool.config.GlobalQueue {
		// If the new transaction is underpriced, don't accept it
		if pool.priced.Underpriced(tx) {
			log.Trace("Discarding underpriced transaction", "hash", hash, "gasTipCap", tx.GasTipCap(), "gasFeeCap", tx.GasFeeCap())
			underpricedTxMeter.Add(context.Background(), 1)
			return false, txpool.ErrUnderpriced
		}

		// We're about to replace a transaction. The reorg does a more thorough
		// analysis of what to remove and how, but it runs async. We don't want to
		// do too many replacements between reorg-runs, so we cap the number of
		// replacements to 25% of the slots
		if pool.changesSinceReorg > int(pool.config.GlobalSlots/4) {
			throttleTxMeter.Add(context.Background(), 1)
			return false, ErrTxPoolOverflow
		}

		// New transaction is better than our worse ones, make room for it.
		// If we can't make enough room for new one, abort the operation.
		drop, success := pool.priced.Discard(pool.all.Slots() - int(pool.config.GlobalSlots+pool.config.GlobalQueue) + numSlots(tx))

		// Special case, we still can't make the room for the new remote one.
		if !success {
			log.Trace("Discarding overflown transaction", "hash", hash)
			overflowedTxMeter.Add(context.Background(), 1)
			return false, ErrTxPoolOverflow
		}

		// If the new transaction is a future transaction it should never churn pending transactions
		if pool.isGapped(from, tx) {
			var replacesPending bool
			for _, dropTx := range drop {
				dropSender, _ := types.Sender(pool.signer, dropTx)
				if list := pool.pending[dropSender]; list != nil && list.Contains(dropTx.Nonce()) {
					replacesPending = true
					break
				}
			}
			// Add all transactions back to the priced queue
			if replacesPending {
				for _, dropTx := range drop {
					pool.priced.Put(dropTx)
				}
				log.Trace("Discarding future transaction replacing pending tx", "hash", hash)
				return false, ErrFutureReplacePending
			}
		}

		// Kick out the underpriced remote transactions.
		for _, tx := range drop {
			log.Trace("Discarding freshly underpriced transaction", "hash", tx.Hash(), "gasTipCap", tx.GasTipCap(), "gasFeeCap", tx.GasFeeCap())
			underpricedTxMeter.Add(context.Background(), 1)

			sender, _ := types.Sender(pool.signer, tx)
			dropped := pool.removeTx(tx.Hash(), false, sender != from, RemovalReasonUnderpricedFull) // Don't unreserve the sender of the tx being added if last from the acc

			pool.changesSinceReorg += dropped
		}
	}

	// Try to replace an existing transaction in the pending pool
	if list := pool.pending[from]; list != nil && list.Contains(tx.Nonce()) {
		// Nonce already pending, check if required price bump is met
		inserted, old := list.Add(tx, pool.config.PriceBump)
		if !inserted {
			pendingDiscardMeter.Add(context.Background(), 1)
			return false, txpool.ErrReplaceUnderpriced
		}
		// New transaction is better, replace old one
		if old != nil {
			pool.all.Remove(old.Hash())
			pool.priced.Removed(1)
			pool.markTxRemoved(from, old, Pending)
			pendingReplaceMeter.Add(context.Background(), 1)
		}
		pool.all.Add(tx)
		pool.priced.Put(tx)
		pool.queueTxEvent(tx)

		// tx went straight to the pending queue and bypassed the queue, mark
		// it as replaced
		pool.markTxReplaced(from, tx)
		log.Trace("Pooled new executable transaction", "hash", hash, "from", from, "to", tx.To())

		// Successful promotion, bump the heartbeat
		pool.beats[from] = time.Now()
		return old != nil, nil
	}
	// New transaction isn't replacing a pending one, push into queue
	replaced, err = pool.enqueueTx(hash, tx, true)
	if err != nil {
		return false, err
	}

	log.Trace("Pooled new future transaction", "hash", hash, "from", from, "to", tx.To())
	return replaced, nil
}

// isGapped reports whether the given transaction is immediately executable.
func (pool *LegacyPool) isGapped(from common.Address, tx *types.Transaction) bool {
	// Short circuit if transaction falls within the scope of the pending list
	// or matches the next pending nonce which can be promoted as an executable
	// transaction afterwards. Note, the tx staleness is already checked in
	// 'validateTx' function previously.
	next := pool.pendingNonces.get(from)
	if tx.Nonce() <= next {
		return false
	}
	// The transaction has a nonce gap with pending list, it's only considered
	// as executable if transactions in queue can fill up the nonce gap.
	queue, ok := pool.queue[from]
	if !ok {
		return true
	}
	for nonce := next; nonce < tx.Nonce(); nonce++ {
		if !queue.Contains(nonce) {
			return true // txs in queue can't fill up the nonce gap
		}
	}
	return false
}

// enqueueTx inserts a new transaction into the non-executable transaction queue.
//
// Note, this method assumes the pool lock is held!
func (pool *LegacyPool) enqueueTx(hash common.Hash, tx *types.Transaction, addAll bool) (bool, error) {
	// Try to insert the transaction into the future queue
	from, _ := types.Sender(pool.signer, tx) // already validated
	if pool.queue[from] == nil {
		pool.queue[from] = newList(false)
	}
	inserted, old := pool.queue[from].Add(tx, pool.config.PriceBump)
	if !inserted {
		// An older transaction was better, discard this
		queuedDiscardMeter.Add(context.Background(), 1)
		return false, txpool.ErrReplaceUnderpriced
	}
	// Discard any previous transaction and mark this
	if old != nil {
		pool.all.Remove(old.Hash())
		pool.priced.Removed(1)
		queuedReplaceMeter.Add(context.Background(), 1)
		pool.markTxRemoved(from, old, Queue)
	} else {
		// Nothing was replaced, bump the queued counter
		queuedGauge.Add(context.Background(), 1)
	}
	// If the transaction isn't in lookup set but it's expected to be there,
	// show the error log.
	if pool.all.Get(hash) == nil && !addAll {
		log.Error("Missing transaction in lookup set, please report the issue", "hash", hash)
	}
	if addAll {
		pool.all.Add(tx)
		pool.priced.Put(tx)
	}
	// If we never record the heartbeat, do it right now.
	if _, exist := pool.beats[from]; !exist {
		pool.beats[from] = time.Now()
	}
	pool.markTxEnqueued(tx)
	return old != nil, nil
}

// promoteTx adds a transaction to the pending (processable) list of transactions
// and returns whether it was inserted or an older was better.
//
// Note, this method assumes the pool lock is held!
func (pool *LegacyPool) promoteTx(addr common.Address, hash common.Hash, tx *types.Transaction) bool {
	// Try to insert the transaction into the pending queue
	if pool.pending[addr] == nil {
		pool.pending[addr] = newList(true)
	}
	list := pool.pending[addr]

	inserted, old := list.Add(tx, pool.config.PriceBump)
	if !inserted {
		// An older transaction was better, discard this
		pool.all.Remove(hash)
		pool.priced.Removed(1)
		pool.markTxRemoved(addr, tx, Queue)
		pendingDiscardMeter.Add(context.Background(), 1)
		return false
	}
	// Otherwise discard any previous transaction and mark this
	if old != nil {
		// TODO: this tx that we are removing may be in the pending builder, we
		// should remove it from there
		pool.all.Remove(old.Hash())
		pool.priced.Removed(1)
		pool.markTxRemoved(addr, old, Pending)
		pendingReplaceMeter.Add(context.Background(), 1)
	} else {
		// Nothing was replaced, bump the pending counter
		pendingGauge.Add(context.Background(), 1)
	}
	// Set the potentially new pending nonce and notify any subsystems of the new tx
	pool.pendingNonces.set(addr, tx.Nonce()+1)

	// Successful promotion, bump the heartbeat
	pool.beats[addr] = time.Now()
	pool.markTxPromoted(addr, tx)
	return true
}

// addRemotes enqueues a batch of transactions into the pool if they are valid.
// Full pricing constraints will apply.
//
// This method is used to add transactions from the p2p network and does not wait for pool
// reorganization and internal event propagation.
func (pool *LegacyPool) addRemotes(txs []*types.Transaction) []error {
	return pool.Add(txs, false)
}

// addRemote enqueues a single transaction into the pool if it is valid. This is a convenience
// wrapper around addRemotes.
func (pool *LegacyPool) addRemote(tx *types.Transaction) error {
	return pool.addRemotes([]*types.Transaction{tx})[0]
}

// addRemotesSync is like addRemotes, but waits for pool reorganization. Tests use this method.
func (pool *LegacyPool) addRemotesSync(txs []*types.Transaction) []error {
	return pool.Add(txs, true)
}

// This is like addRemotes with a single transaction, but waits for pool reorganization. Tests use this method.
func (pool *LegacyPool) addRemoteSync(tx *types.Transaction) error {
	return pool.Add([]*types.Transaction{tx}, true)[0]
}

// Add enqueues a batch of transactions into the pool if they are valid.
//
// Note, if sync is set the method will block until all internal maintenance
// related to the add is finished. Only use this during tests for determinism.
func (pool *LegacyPool) Add(txs []*types.Transaction, sync bool) []error {
	// Filter out known ones without obtaining the pool lock or recovering signatures
	var (
		errs = make([]error, len(txs))
		news = make([]*types.Transaction, 0, len(txs))
	)
	for i, tx := range txs {
		// If the transaction is known, pre-set the error slot
		if pool.all.Get(tx.Hash()) != nil {
			errs[i] = txpool.ErrAlreadyKnown
			knownTxMeter.Add(context.Background(), 1)
			continue
		}
		// Exclude transactions with basic errors, e.g invalid signatures and
		// insufficient intrinsic gas as soon as possible and cache senders
		// in transactions before obtaining lock
		if err := pool.ValidateTxBasics(tx); err != nil {
			errs[i] = err
			log.Trace("Discarding invalid transaction", "hash", tx.Hash(), "err", err)
			invalidTxMeter.Add(context.Background(), 1)
			continue
		}
		// Accumulate all unknown transactions for deeper processing
		news = append(news, tx)
	}
	if len(news) == 0 {
		return errs
	}

	// Process all the new transaction and merge any errors into the original slice
	pool.mu.Lock()
	newErrs, dirtyAddrs := pool.addTxsLocked(news)
	pool.mu.Unlock()

	nilSlot := 0
	for _, err := range newErrs {
		for errs[nilSlot] != nil {
			nilSlot++
		}
		errs[nilSlot] = err
		nilSlot++
	}
	// Reorg the pool internals if needed and return
	done := pool.requestPromoteExecutables(dirtyAddrs)
	if sync {
		<-done
	}
	return errs
}

// addTxsLocked attempts to queue a batch of transactions if they are valid.
// The transaction pool lock must be held.
func (pool *LegacyPool) addTxsLocked(txs []*types.Transaction) ([]error, *accountSet) {
	dirty := newAccountSet(pool.signer)
	errs := make([]error, len(txs))
	for i, tx := range txs {
		replaced, err := pool.add(tx)
		errs[i] = err
		if err == nil && !replaced {
			dirty.addTx(tx)
		}
	}
	validTxMeter.Add(context.Background(), int64(len(dirty.accounts)))
	return errs, dirty
}

// Status returns the status (unknown/pending/queued) of a batch of transactions
// identified by their hashes.
func (pool *LegacyPool) Status(hash common.Hash) txpool.TxStatus {
	tx := pool.get(hash)
	if tx == nil {
		return txpool.TxStatusUnknown
	}
	from, _ := types.Sender(pool.signer, tx) // already validated

	pool.mu.RLock()
	defer pool.mu.RUnlock()

	if txList := pool.pending[from]; txList != nil && txList.txs.items[tx.Nonce()] != nil {
		return txpool.TxStatusPending
	} else if txList := pool.queue[from]; txList != nil && txList.txs.items[tx.Nonce()] != nil {
		return txpool.TxStatusQueued
	}
	return txpool.TxStatusUnknown
}

// Get returns a transaction if it is contained in the pool and nil otherwise.
func (pool *LegacyPool) Get(hash common.Hash) *types.Transaction {
	tx := pool.get(hash)
	if tx == nil {
		return nil
	}
	return tx
}

// get returns a transaction if it is contained in the pool and nil otherwise.
func (pool *LegacyPool) get(hash common.Hash) *types.Transaction {
	return pool.all.Get(hash)
}

// GetRLP returns a RLP-encoded transaction if it is contained in the pool.
func (pool *LegacyPool) GetRLP(hash common.Hash) []byte {
	tx := pool.all.Get(hash)
	if tx == nil {
		return nil
	}
	encoded, err := rlp.EncodeToBytes(tx)
	if err != nil {
		log.Error("Failed to encoded transaction in legacy pool", "hash", hash, "err", err)
		return nil
	}
	return encoded
}

// GetMetadata returns the transaction type and transaction size with the
// given transaction hash.
func (pool *LegacyPool) GetMetadata(hash common.Hash) *txpool.TxMetadata {
	tx := pool.all.Get(hash)
	if tx == nil {
		return nil
	}
	return &txpool.TxMetadata{
		Type: tx.Type(),
		Size: tx.Size(),
	}
}

// GetBlobs is not supported by the legacy transaction pool, it is just here to
// implement the txpool.SubPool interface.
func (pool *LegacyPool) GetBlobs(vhashes []common.Hash) ([]*kzg4844.Blob, []*kzg4844.Proof) {
	return nil, nil
}

// Has returns an indicator whether txpool has a transaction cached with the
// given hash.
func (pool *LegacyPool) Has(hash common.Hash) bool {
	return pool.all.Get(hash) != nil
}

// RemoveTx removes a single transaction from the queue, moving all subsequent
// transactions back to the future queue.
//
// In unreserve is false, the account will not be relinquished to the main txpool
// even if there are no more references to it. This is used to handle a race when
// a tx being added, and it evicts a previously scheduled tx from the same account,
// which could lead to a premature release of the lock.
//
// Returns the number of transactions removed from the pending queue.
func (pool *LegacyPool) RemoveTx(hash common.Hash, outofbound bool, unreserve bool, reason txpool.RemovalReason) int {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return pool.removeTx(hash, outofbound, unreserve, reason)
}

// removeTx removes a single transaction from the queue, moving all subsequent
// transactions back to the future queue.
//
// If unreserve is false, the account will not be relinquished to the main txpool
// even if there are no more references to it. This is used to handle a race when
// a tx being added, and it evicts a previously scheduled tx from the same account,
// which could lead to a premature release of the lock.
//
// Returns the number of transactions removed from the pending queue.
//
// The transaction pool lock must be held.
func (pool *LegacyPool) removeTx(hash common.Hash, outofbound bool, unreserve bool, reason txpool.RemovalReason) int {
	// Fetch the transaction we wish to delete
	tx := pool.all.Get(hash)
	if tx == nil {
		return 0
	}
	addr, _ := types.Sender(pool.signer, tx) // already validated during insertion

	// If after deletion there are no more transactions belonging to this account,
	// relinquish the address reservation. It's a bit convoluted do this, via a
	// defer, but it's safer vs. the many return pathways.
	if unreserve {
		defer func() {
			var (
				_, hasPending = pool.pending[addr]
				_, hasQueued  = pool.queue[addr]
			)
			if !hasPending && !hasQueued {
				pool.reserver.Release(addr)
			}
		}()
	}
	// Remove it from the list of known transactions
	pool.all.Remove(hash)
	if outofbound {
		pool.priced.Removed(1)
	}
	// Remove the transaction from the pending lists and reset the account nonce
	if pending := pool.pending[addr]; pending != nil {
		if removed, invalids := pending.Remove(tx); removed {
			pool.markTxRemoved(addr, tx, Pending)
			pendingRemovalMetric(reason).Add(context.Background(), 1)

			// If no more pending transactions are left, remove the list
			if pending.Empty() {
				delete(pool.pending, addr)
			}
			// Postpone any invalidated transactions
			for _, tx := range invalids {
				// Internal shuffle shouldn't touch the lookup set.
				pool.enqueueTx(tx.Hash(), tx, false)
			}
			// Update the account nonce if needed
			pool.pendingNonces.setIfLower(addr, tx.Nonce())
			// Reduce the pending counter
			pendingGauge.Add(context.Background(), -int64(1+len(invalids)))
			pendingDemotedRemoved.Add(context.Background(), int64(len(invalids)))
			return 1 + len(invalids)
		}
	}
	// Transaction is in the future queue
	if future := pool.queue[addr]; future != nil {
		if removed, _ := future.Remove(tx); removed {
			pool.markTxRemoved(addr, tx, Queue)
			queueRemovalMetric(reason).Add(context.Background(), 1)

			// Reduce the queued counter
			queuedGauge.Add(context.Background(), -1)
		}
		if future.Empty() {
			delete(pool.queue, addr)
			delete(pool.beats, addr)
		}
	}
	return 0
}

// requestReset requests a pool reset to the new head block.
// The returned channel is closed when the reset has occurred.
func (pool *LegacyPool) requestReset(oldHead *types.Header, newHead *types.Header) chan struct{} {
	select {
	case pool.reqResetCh <- &txpoolResetRequest{oldHead, newHead}:
		return <-pool.reorgDoneCh
	case <-pool.reorgShutdownCh:
		return pool.reorgShutdownCh
	}
}

// requestPromoteExecutables requests transaction promotion checks for the given addresses.
// The returned channel is closed when the promotion checks have occurred.
func (pool *LegacyPool) requestPromoteExecutables(set *accountSet) chan struct{} {
	select {
	case pool.reqPromoteCh <- set:
		return <-pool.reorgDoneCh
	case <-pool.reorgShutdownCh:
		return pool.reorgShutdownCh
	}
}

// queueTxEvent enqueues a transaction event to be sent in the next reorg run.
func (pool *LegacyPool) queueTxEvent(tx *types.Transaction) {
	select {
	case pool.queueTxEventCh <- tx:
	case <-pool.reorgShutdownCh:
	}
}

// scheduleReorgLoop schedules runs of reset and promoteExecutables. Code above should not
// call those methods directly, but request them being run using requestReset and
// requestPromoteExecutables instead.
func (pool *LegacyPool) scheduleReorgLoop() {
	defer pool.wg.Done()

	var (
		curDone        chan struct{} // non-nil while runReorg is active
		nextDone       = make(chan struct{})
		resetCancelled = make(chan struct{})
		launchNextRun  bool
		reset          *txpoolResetRequest
		dirtyAccounts  *accountSet
		queuedEvents   = make(map[common.Address]*SortedMap)
	)
	for {
		// Launch next background reorg if needed
		if curDone == nil && launchNextRun {
			// Run the background reorg and announcements
			input := reorgInput{
				done:          nextDone,
				cancelReset:   resetCancelled,
				reset:         reset,
				dirtyAccounts: dirtyAccounts,
				events:        queuedEvents,
			}
			go pool.runReorg(input)

			// Prepare everything for the next round of reorg
			curDone, nextDone = nextDone, make(chan struct{})
			launchNextRun = false

			reset, dirtyAccounts = nil, nil
			queuedEvents = make(map[common.Address]*SortedMap)
		}

		select {
		case req := <-pool.reqResetCh:
			// Reset request: update head if request is already pending.
			if reset == nil {
				reset = req
			} else {
				reset.newHead = req.newHead
			}
			launchNextRun = true
			pool.reorgDoneCh <- nextDone

		case <-pool.reqCancelResetCh:
			// Only process the request if the reorg loop is running (curDone
			// != nil) and a cancel request for this reorg loop has not been
			// processed already (resetCancelled != nil)
			if resetCancelled != nil && curDone != nil {
				close(resetCancelled)
				// Set to nil to dedupe any future requests to cancel reset for
				// this same reorg iteration. Once this run finishes,
				// resetCancelled will be recreated as non nil chan and the
				// next iteration will support being cancelled again.
				resetCancelled = nil
			}
		case req := <-pool.reqPromoteCh:
			// Promote request: update address set if request is already pending.
			if dirtyAccounts == nil {
				dirtyAccounts = req
			} else {
				dirtyAccounts.merge(req)
			}
			launchNextRun = true
			pool.reorgDoneCh <- nextDone

		case tx := <-pool.queueTxEventCh:
			// Queue up the event, but don't schedule a reorg. It's up to the caller to
			// request one later if they want the events sent.
			addr, _ := types.Sender(pool.signer, tx)
			if _, ok := queuedEvents[addr]; !ok {
				queuedEvents[addr] = NewSortedMap()
			}
			queuedEvents[addr].Put(tx)
		case <-curDone:
			curDone = nil
			resetCancelled = make(chan struct{})

		case <-pool.reorgShutdownCh:
			// Wait for current run to finish.
			if curDone != nil {
				<-curDone
			}
			close(nextDone)
			return
		}
	}
}

type reorgInput struct {
	done          chan struct{}
	cancelReset   chan struct{}
	reset         *txpoolResetRequest
	dirtyAccounts *accountSet
	events        map[common.Address]*SortedMap
}

// computePendingTipNonces returns a mapping from account address to the next
// expected nonce, taking the pending pool entries into account, but not the
// queued pool entries.
func (pool *LegacyPool) computePendingTipNonces() map[common.Address]uint64 {
	tips := make(map[common.Address]uint64, len(pool.pending))
	for addr, list := range pool.pending {
		// get the next nonce that should be included on chain after last
		// pending pool tx for this account lands
		nextPendingNonce := list.LastElement().Nonce() + 1

		// grab the latest nonce that we have observed on chain for this
		// account if we have it cached
		latestIncludedNonce, ok := pool.latestIncludedNonce.Get(addr)
		if ok {
			// if we have a cached on chain nonce for this account, it likely
			// had a tx land in the latest block and its last pending pool
			// element may be stale if we have not yet run demoteUnexecutables
			// yet. thus, if the latestIncludedNonce + 1 is > nextPendingNonce,
			// then that is actually what we should be using as the 'tip' of
			// the pending pool for this account
			//
			// NOTE: latestIncludedNonce is a lru cache (lru to manage its
			// growth). It is strongly recommended to the user to configure
			// this cache size > the max number of unique accounts they expect
			// to see in a block. If they do not do so, an issue could manifest
			// here where a block has more accounts than entries in the
			// latestIncludedNonce cache. If an entry gets evicted from
			// latestIncludedNonce for an account that was included in H-1,
			// then calling this function at H will use a stale
			// nextPendingNonce value (when compared to statedb.GetNonce(addr))
			// if this function is called before demoteUnexecutables, causing a
			// queued contiguous nonce tx (with respect to pending txs) to not
			// be promoted if there is a race on resetting + promoting the
			// queued tx after insert. This is unlikely and would require a
			// misconfiguration by the user, but this is a documented possible
			// scenario.
			nextPendingNonce = max(nextPendingNonce, latestIncludedNonce+1)
		}
		tips[addr] = nextPendingNonce
	}
	return tips
}

// runReorg runs reset and promoteExecutables on behalf of scheduleReorgLoop.
func (pool *LegacyPool) runReorg(input reorgInput) {
	defer func(t0 time.Time) {
		elapsedMS := float64(time.Since(t0).Milliseconds())
		reorgDurationTimer.Record(context.Background(), elapsedMS)
		if input.reset != nil {
			reorgResetTimer.Record(context.Background(), elapsedMS)
		}
	}(time.Now())
	defer close(input.done)

	var promoteAddrs []common.Address
	// Optionally acquire a shared read lock to coordinate with Commit in tests.
	unlock := beginCommitRead(any(pool.chain))
	defer unlock()
	if input.dirtyAccounts != nil && input.reset == nil {
		// Only dirty accounts need to be promoted, unless we're resetting.
		// For resets, all addresses in the tx queue will be promoted and
		// the flatten operation can be avoided.
		promoteAddrs = input.dirtyAccounts.flatten()
	}
	pool.mu.Lock()
	if input.reset != nil {
		// Reset from the old head to the new, rescheduling any reorged transactions
		pool.reset(input.reset.oldHead, input.reset.newHead)

		// Nonces were reset, discard any events that became stale
		for addr := range input.events {
			input.events[addr].Forward(pool.pendingNonces.get(addr))
			if input.events[addr].Len() == 0 {
				delete(input.events, addr)
			}
		}
		// Reset needs promote for all addresses
		promoteAddrs = make([]common.Address, 0, len(pool.queue))
		for addr := range pool.queue {
			promoteAddrs = append(promoteAddrs, addr)
		}
	}

	// Check for pending transactions for every account that sent new ones
	promoted := pool.promoteExecutables(promoteAddrs, input.cancelReset, input.reset)

	// If a new block appeared, validate the pool of pending transactions. This will
	// remove any transaction that has been included in the block or was invalidated
	// because of another transaction (e.g. higher gas price).
	if input.reset != nil {
		pool.demoteUnexecutables(input.cancelReset, input.reset)
		if input.reset.newHead != nil {
			if pool.chainconfig.IsLondon(new(big.Int).Add(input.reset.newHead.Number, big.NewInt(1))) {
				pendingBaseFee := eip1559.CalcBaseFee(pool.chainconfig, input.reset.newHead)
				pool.priced.SetBaseFee(pendingBaseFee)
			} else {
				pool.priced.Reheap()
			}
		}
		// Update all accounts to the latest known pending nonce
		pool.pendingNonces.setAll(pool.computePendingTipNonces())
		pool.validPendingTxs.EndCurrentHeight()
	}

	// Ensure pool.queue and pool.pending sizes stay within the configured limits.
	//
	// TODO: We are adding to the pending builder before this, then we may
	// immediately drop some of the txs we put in pending via truncate. we will
	// likely have to change this algorithm to truncate as we call demote
	// unexecutables per account (currently it needs info on all accounts since
	// it tries to truncate fairly across accounts).
	pool.truncatePending()
	pool.truncateQueue()

	dropBetweenReorgHistogram.Record(context.Background(), int64(pool.changesSinceReorg))
	pool.changesSinceReorg = 0 // Reset change counter
	pool.mu.Unlock()

	// Notify subsystems for newly added transactions
	for _, tx := range promoted {
		addr, _ := types.Sender(pool.signer, tx)
		if _, ok := input.events[addr]; !ok {
			input.events[addr] = NewSortedMap()
		}
		input.events[addr].Put(tx)
	}
	if len(input.events) > 0 {
		var txs []*types.Transaction
		for _, set := range input.events {
			txs = append(txs, set.Flatten()...)
		}

		// NOTE: We are not calling PromoteTx here on txs with events queued
		// for them (even though this does mean they were promoted), however it
		// is possible that this tx was promoted a long time ago, but the
		// runReorg was not scheduled until later, so the event has not been
		// handled. Since this is a possible scenario we opt to call PromoteTx
		// on site where the tx is inserted into the pending queue, not just
		// when handling events.

		pool.txFeed.Send(core.NewTxsEvent{Txs: txs})
	}
}

// resetInternalState initializes the internal state to the current head and reinjects transactions
func (pool *LegacyPool) resetInternalState(newHead *types.Header, reinject types.Transactions) {
	// Initialize the internal state to the current head
	if newHead == nil {
		newHead = pool.chain.CurrentBlock() // Special case during testing
	}
	statedb, err := pool.chain.StateAt(newHead.Root)
	if err != nil {
		log.Error("Failed to reset txpool state", "err", err)
		return
	}
	pool.currentHead.Store(newHead)
	pool.currentState = statedb
	pool.pendingNonces = newNoncer(statedb)

	// a brand new noncer only knows the statedb's committed nonce, which may
	// be behind the tip of pool.pending nonce wise. we need to reconcile
	// the noncer with the current pending tip so that any subsequent
	// promoteExecutables can still promote queued txs whose nonces are
	// contiguous with pending.
	//
	// without this the following scenario is possible:
	// - pool.pending = [tx0, tx1], pool.queue = [tx2].
	// - statedb nonce for this account is 0 (nothing included on chain).
	// - reset runs **before** the async promotion gets scheduled, or they are batched together.
	// - pendingNonces reset = newNoncer(statedb), noncer map empty so
	//   get(addr) falls back to nonce 0 from statedb.
	// - promoteExecutables calls list.Ready(0) on pool.queue = [tx2], sees
	//   lowest nonce 2 > 0, returns a gap, promotes nothing.
	// - tx2 stays stuck in queued even though it is contiguous with pending.
	// - tx2 must wait for another insert from this account in order to be
	//   promoted, or the existing pending txs are included on chain.
	pool.pendingNonces.setAll(pool.computePendingTipNonces())

	ctx, err := pool.chain.GetLatestContext()
	if err != nil {
		panic(fmt.Errorf("failed to get latest context for rechecker: %w", err))
	}
	pool.rechecker.Update(ctx, newHead)
	pool.validPendingTxs.StartNewHeight(newHead.Number)

	// Inject any transactions discarded due to reorgs
	log.Debug("Reinjecting stale transactions", "count", len(reinject))
	core.SenderCacher().Recover(pool.signer, reinject)
	pool.addTxsLocked(reinject)
}

// isResetCancelled returns true if the pool is resetting and it has been
// signaled to cancel the reset.
func isReorgCancelled(reset *txpoolResetRequest, cancelled chan struct{}) bool {
	if reset != nil {
		select {
		case <-cancelled:
			return true
		default:
			return false
		}
	}
	return false
}

// promoteExecutables moves transactions that have become processable from the
// future queue to the set of pending transactions. During this process, all
// invalidated transactions (low nonce, low balance) are deleted.
func (pool *LegacyPool) promoteExecutables(accounts []common.Address, cancelled chan struct{}, reset *txpoolResetRequest) []*types.Transaction {
	// Track the promoted transactions to broadcast them at once
	var promoted []*types.Transaction

	// Get a branch of the latest pending context for recheck
	ctx, write := pool.rechecker.GetContext()

	// Iterate over all accounts and promote any executable transactions
	for _, addr := range accounts {
		if isReorgCancelled(reset, cancelled) {
			queuedPromotedCancelled.Add(context.Background(), 1)
			return promoted
		}
		list := pool.queue[addr]
		if list == nil {
			continue // Just in case someone calls with a non existing account
		}

		// Drop all transactions that are below the latest included nonce for
		// this account based on what we have seen during the latest block
		// execution. Only do this if we are resetting.
		var olds types.Transactions
		if reset != nil {
			olds = pool.removeOlds(addr, list, Queue)
			queuedRemovedOld.Add(context.Background(), int64(len(olds)))
		}

		// Drop all transactions that now fail the pools RecheckTxFn
		//
		// Note this is happening after the nonce removal above since this
		// check is slower, we would like it to happen on the fewest txs as
		// possible.
		recheckStart := time.Now()
		recheckDrops, _ := list.FilterSorted(func(tx *types.Transaction) bool {
			newCtx, err := pool.rechecker.RecheckEVM(ctx, tx)
			if !newCtx.IsZero() {
				ctx = newCtx
			}
			if err == nil && reset == nil {
				// only write changes back to original context if we are not
				// running in reset mode, i.e. a new block has not been seen
				write()
			}
			return tolerateRecheckErr(err) != nil
		})
		for _, tx := range recheckDrops {
			pool.all.Remove(tx.Hash())
			pool.markTxRemoved(addr, tx, Queue)
		}
		log.Trace("Removed queued transactions that failed recheck", "count", len(recheckDrops))
		queuedRecheckDropMeter.Add(context.Background(), int64(len(recheckDrops)))
		queuedRecheckDurationTimer.Record(context.Background(), float64(time.Since(recheckStart).Milliseconds()))

		// Gather all executable transactions and promote them
		listLen := list.Len()
		readies := list.Ready(pool.pendingNonces.get(addr))
		queuedNonReadies.Add(context.Background(), int64(listLen-len(readies)))
		for _, tx := range readies {
			hash := tx.Hash()
			if pool.promoteTx(addr, hash, tx) {
				promoted = append(promoted, tx)
			}
		}
		log.Trace("Promoted queued transactions", "count", len(promoted))
		queuedGauge.Add(context.Background(), -int64(len(readies)))

		// Drop all transactions over the allowed limit
		caps := list.Cap(int(pool.config.AccountQueue))
		for _, tx := range caps {
			hash := tx.Hash()
			pool.all.Remove(hash)
			pool.markTxRemoved(addr, tx, Queue)
			queueRemovalMetric(RemovalReasonCapExceeded).Add(context.Background(), 1)
			log.Trace("Removed cap-exceeding queued transaction", "hash", hash)
		}
		queuedRateLimitMeter.Add(context.Background(), int64(len(caps)))

		// Mark all the items dropped as removed
		totalDropped := len(recheckDrops) + len(caps) + len(olds)
		pool.priced.Removed(totalDropped)
		queuedGauge.Add(context.Background(), -int64(totalDropped))

		// Delete the entire queue entry if it became empty.
		if list.Empty() {
			delete(pool.queue, addr)
			delete(pool.beats, addr)
			if _, ok := pool.pending[addr]; !ok {
				pool.reserver.Release(addr)
			}
		}
	}
	return promoted
}

// truncatePending removes transactions from the pending queue if the pool is above the
// pending limit. The algorithm tries to reduce transaction counts by an approximately
// equal number for all for accounts with many pending transactions.
func (pool *LegacyPool) truncatePending() {
	defer func(t0 time.Time) {
		pendingTruncateTimer.Record(context.Background(), float64(time.Since(t0).Milliseconds()))
	}(time.Now())
	pending := uint64(0)

	// Assemble a spam order to penalize large transactors first
	spammers := prque.New[uint64, common.Address](nil)
	for addr, list := range pool.pending {
		// Only evict transactions from high rollers
		length := uint64(list.Len())
		pending += length
		if length > pool.config.AccountSlots {
			spammers.Push(addr, length)
		}
	}
	if pending <= pool.config.GlobalSlots {
		return
	}
	pendingBeforeCap := pending

	// Gradually drop transactions from offenders
	offenders := []common.Address{}
	for pending > pool.config.GlobalSlots && !spammers.Empty() {
		// Retrieve the next offender
		offender, _ := spammers.Pop()
		offenders = append(offenders, offender)

		// Equalize balances until all the same or below threshold
		if len(offenders) > 1 {
			// Calculate the equalization threshold for all current offenders
			threshold := pool.pending[offender].Len()

			// Iteratively reduce all offenders until below limit or threshold reached
			for pending > pool.config.GlobalSlots && pool.pending[offenders[len(offenders)-2]].Len() > threshold {
				for i := 0; i < len(offenders)-1; i++ {
					list := pool.pending[offenders[i]]

					caps := list.Cap(list.Len() - 1)
					for _, tx := range caps {
						// Drop the transaction from the global pools too
						hash := tx.Hash()
						pool.all.Remove(hash)
						pool.markTxRemoved(offenders[i], tx, Pending)
						pendingRemovalMetric(RemovalReasonCapExceeded).Add(context.Background(), 1)

						// Update the account nonce to the dropped transaction
						pool.pendingNonces.setIfLower(offenders[i], tx.Nonce())
						log.Trace("Removed fairness-exceeding pending transaction", "hash", hash)
					}
					pool.priced.Removed(len(caps))
					pendingGauge.Add(context.Background(), -int64(len(caps)))

					pending--
				}
			}
		}
	}

	// If still above threshold, reduce to limit or min allowance
	if pending > pool.config.GlobalSlots && len(offenders) > 0 {
		for pending > pool.config.GlobalSlots && uint64(pool.pending[offenders[len(offenders)-1]].Len()) > pool.config.AccountSlots {
			for _, addr := range offenders {
				list := pool.pending[addr]

				caps := list.Cap(list.Len() - 1)
				for _, tx := range caps {
					// Drop the transaction from the global pools too
					hash := tx.Hash()
					pool.all.Remove(hash)
					pool.markTxRemoved(addr, tx, Pending)
					pendingRemovalMetric(RemovalReasonCapExceeded).Add(context.Background(), 1)

					// Update the account nonce to the dropped transaction
					pool.pendingNonces.setIfLower(addr, tx.Nonce())
					log.Trace("Removed fairness-exceeding pending transaction", "hash", hash)
				}
				pool.priced.Removed(len(caps))
				pendingGauge.Add(context.Background(), -int64(len(caps)))
				pending--
			}
		}
	}
	pendingRateLimitMeter.Add(context.Background(), int64(pendingBeforeCap-pending))
}

// truncateQueue drops the oldest transactions in the queue if the pool is above the global queue limit.
func (pool *LegacyPool) truncateQueue() {
	queued := uint64(0)
	for _, list := range pool.queue {
		queued += uint64(list.Len())
	}
	if queued <= pool.config.GlobalQueue {
		return
	}

	// Sort all accounts with queued transactions by heartbeat
	addresses := make(addressesByHeartbeat, 0, len(pool.queue))
	for addr := range pool.queue {
		addresses = append(addresses, addressByHeartbeat{addr, pool.beats[addr]})
	}
	sort.Sort(sort.Reverse(addresses))

	// Drop transactions until the total is below the limit
	for drop := queued - pool.config.GlobalQueue; drop > 0 && len(addresses) > 0; {
		addr := addresses[len(addresses)-1]
		list := pool.queue[addr.address]

		addresses = addresses[:len(addresses)-1]

		// Drop all transactions if they are less than the overflow
		if size := uint64(list.Len()); size <= drop {
			for _, tx := range list.Flatten() {
				pool.removeTx(tx.Hash(), true, true, RemovalReasonTruncatedOverflow)
			}
			drop -= size
			queuedRateLimitMeter.Add(context.Background(), int64(size))
			continue
		}
		// Otherwise drop only last few transactions
		txs := list.Flatten()
		for i := len(txs) - 1; i >= 0 && drop > 0; i-- {
			pool.removeTx(txs[i].Hash(), true, true, RemovalReasonTruncatedLast)
			drop--
			queuedRateLimitMeter.Add(context.Background(), 1)
		}
	}
}

// demoteUnexecutables removes invalid and processed transactions from the pools
// executable/pending queue and any subsequent transactions that become unexecutable
// are moved back into the future queue.
//
// Note: transactions are not marked as removed in the priced list because re-heaping
// is always explicitly triggered by SetBaseFee and it would be unnecessary and wasteful
// to trigger a re-heap is this function
func (pool *LegacyPool) demoteUnexecutables(cancelled chan struct{}, reset *txpoolResetRequest) {
	defer func(t0 time.Time) {
		demoteTimer.Record(context.Background(), float64(time.Since(t0).Milliseconds()))
	}(time.Now())

	// Iterate over all accounts and demote any non-executable transactions
	for addr, list := range pool.pending {
		if isReorgCancelled(reset, cancelled) {
			pendingDemotedCancelled.Add(context.Background(), 1)
			return
		}

		// Drop all transactions that are below the latest included nonce for
		// this account based on what we have seen during the latest block
		// execution.
		olds := pool.removeOlds(addr, list, Pending)
		pendingRemovedOld.Add(context.Background(), int64(len(olds)))

		// Drop all transactions that now fail the pools RecheckTxFn
		//
		// Note this is happening after the nonce removal above since this
		// check is slower, we would like it to happen on the fewest txs as
		// possible.
		recheckStart := time.Now()
		ctx, write := pool.rechecker.GetContext()
		recheckDrops, recheckInvalids := list.FilterSorted(func(tx *types.Transaction) bool {
			newCtx, err := pool.rechecker.RecheckEVM(ctx, tx)
			if !newCtx.IsZero() {
				ctx = newCtx
			}
			if err == nil {
				// always write changes back to the original context
				write()
			}
			return tolerateRecheckErr(err) != nil
		})

		for _, tx := range recheckDrops {
			hash := tx.Hash()
			pool.all.Remove(hash)
			pool.markTxRemoved(addr, tx, Pending)
			log.Trace("Removed pending transaction that failed recheck", "hash", hash)
		}
		pendingRecheckDropMeter.Add(context.Background(), int64(len(recheckDrops)))
		pendingDemotedRecheck.Add(context.Background(), int64(len(recheckInvalids)))
		pendingRecheckDurationTimer.Record(context.Background(), float64(time.Since(recheckStart).Milliseconds()))

		invalids := recheckInvalids
		for _, tx := range invalids {
			hash := tx.Hash()
			log.Trace("Demoting pending transaction", "hash", hash)

			// Internal shuffle shouldn't touch the lookup set.
			pool.enqueueTx(hash, tx, false)
		}
		pendingGauge.Add(context.Background(), -int64(len(recheckDrops)+len(invalids)+len(olds)))

		// Delete the entire pending entry if it became empty.
		if list.Empty() {
			delete(pool.pending, addr)
			if _, ok := pool.queue[addr]; !ok {
				pool.reserver.Release(addr)
			}
		} else {
			pool.validPendingTxs.Do(func(store *TxStore) {
				store.AddTxs(addr, list.Flatten())
			})
		}
	}
}

// addressByHeartbeat is an account address tagged with its last activity timestamp.
type addressByHeartbeat struct {
	address   common.Address
	heartbeat time.Time
}

type addressesByHeartbeat []addressByHeartbeat

func (a addressesByHeartbeat) Len() int           { return len(a) }
func (a addressesByHeartbeat) Less(i, j int) bool { return a[i].heartbeat.Before(a[j].heartbeat) }
func (a addressesByHeartbeat) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// accountSet is simply a set of addresses to check for existence, and a signer
// capable of deriving addresses from transactions.
type accountSet struct {
	accounts map[common.Address]struct{}
	signer   types.Signer
	cache    []common.Address
}

// newAccountSet creates a new address set with an associated signer for sender
// derivations.
func newAccountSet(signer types.Signer, addrs ...common.Address) *accountSet {
	as := &accountSet{
		accounts: make(map[common.Address]struct{}, len(addrs)),
		signer:   signer,
	}
	for _, addr := range addrs {
		as.add(addr)
	}
	return as
}

// add inserts a new address into the set to track.
func (as *accountSet) add(addr common.Address) {
	as.accounts[addr] = struct{}{}
	as.cache = nil
}

// addTx adds the sender of tx into the set.
func (as *accountSet) addTx(tx *types.Transaction) {
	if addr, err := types.Sender(as.signer, tx); err == nil {
		as.add(addr)
	}
}

// flatten returns the list of addresses within this set, also caching it for later
// reuse. The returned slice should not be changed!
func (as *accountSet) flatten() []common.Address {
	if as.cache == nil {
		as.cache = slices.Collect(maps.Keys(as.accounts))
	}
	return as.cache
}

// merge adds all addresses from the 'other' set into 'as'.
func (as *accountSet) merge(other *accountSet) {
	maps.Copy(as.accounts, other.accounts)
	as.cache = nil
}

// lookup is used internally by LegacyPool to track transactions while allowing
// lookup without mutex contention.
//
// Note, although this type is properly protected against concurrent access, it
// is **not** a type that should ever be mutated or even exposed outside of the
// transaction pool, since its internal state is tightly coupled with the pools
// internal mechanisms. The sole purpose of the type is to permit out-of-bound
// peeking into the pool in LegacyPool.Get without having to acquire the widely scoped
// LegacyPool.mu mutex.
type lookup struct {
	slots int
	lock  sync.RWMutex
	txs   map[common.Hash]*types.Transaction

	auths map[common.Address][]common.Hash // All accounts with a pooled authorization
}

// newLookup returns a new lookup structure.
func newLookup() *lookup {
	return &lookup{
		txs:   make(map[common.Hash]*types.Transaction),
		auths: make(map[common.Address][]common.Hash),
	}
}

// Range calls f on each key and value present in the map. The callback passed
// should return the indicator whether the iteration needs to be continued.
// Callers need to specify which set (or both) to be iterated.
func (t *lookup) Range(f func(hash common.Hash, tx *types.Transaction) bool) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	for key, value := range t.txs {
		if !f(key, value) {
			return
		}
	}
}

// Get returns a transaction if it exists in the lookup, or nil if not found.
func (t *lookup) Get(hash common.Hash) *types.Transaction {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return t.txs[hash]
}

// Count returns the current number of transactions in the lookup.
func (t *lookup) Count() int {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return len(t.txs)
}

// Slots returns the current number of slots used in the lookup.
func (t *lookup) Slots() int {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return t.slots
}

// Add adds a transaction to the lookup.
func (t *lookup) Add(tx *types.Transaction) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.slots += numSlots(tx)
	slotsGauge.Record(context.Background(), int64(t.slots))

	t.txs[tx.Hash()] = tx
	t.addAuthorities(tx)
}

// Remove removes a transaction from the lookup.
func (t *lookup) Remove(hash common.Hash) {
	t.lock.Lock()
	defer t.lock.Unlock()

	tx, ok := t.txs[hash]
	if !ok {
		log.Error("No transaction found to be deleted", "hash", hash)
		return
	}
	t.removeAuthorities(tx)
	t.slots -= numSlots(tx)
	slotsGauge.Record(context.Background(), int64(t.slots))

	delete(t.txs, hash)
}

// Clear resets the lookup structure, removing all stored entries.
func (t *lookup) Clear() {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.slots = 0
	t.txs = make(map[common.Hash]*types.Transaction)
	t.auths = make(map[common.Address][]common.Hash)
}

// TxsBelowTip finds all remote transactions below the given tip threshold.
func (t *lookup) TxsBelowTip(threshold *big.Int) types.Transactions {
	found := make(types.Transactions, 0, 128)
	t.Range(func(hash common.Hash, tx *types.Transaction) bool {
		if tx.GasTipCapIntCmp(threshold) < 0 {
			found = append(found, tx)
		}
		return true
	})
	return found
}

// addAuthorities tracks the supplied tx in relation to each authority it
// specifies.
func (t *lookup) addAuthorities(tx *types.Transaction) {
	for _, addr := range tx.SetCodeAuthorities() {
		list, ok := t.auths[addr]
		if !ok {
			list = []common.Hash{}
		}
		if slices.Contains(list, tx.Hash()) {
			// Don't add duplicates.
			continue
		}
		list = append(list, tx.Hash())
		t.auths[addr] = list
	}
}

// removeAuthorities stops tracking the supplied tx in relation to its
// authorities.
func (t *lookup) removeAuthorities(tx *types.Transaction) {
	hash := tx.Hash()
	for _, addr := range tx.SetCodeAuthorities() {
		list := t.auths[addr]
		// Remove tx from tracker.
		if i := slices.Index(list, hash); i >= 0 {
			list = append(list[:i], list[i+1:]...)
		} else {
			log.Error("Authority with untracked tx", "addr", addr, "hash", hash)
		}
		if len(list) == 0 {
			// If list is newly empty, delete it entirely.
			delete(t.auths, addr)
			continue
		}
		t.auths[addr] = list
	}
}

// hasAuth returns a flag indicating whether there are pending authorizations
// from the specified address.
func (t *lookup) hasAuth(addr common.Address) bool {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return len(t.auths[addr]) > 0
}

// numSlots calculates the number of slots needed for a single transaction.
func numSlots(tx *types.Transaction) int {
	return int((tx.Size() + txSlotSize - 1) / txSlotSize)
}

// Clear implements txpool.SubPool, removing all tracked txs from the pool
// and rotating the journal.
//
// Note, do not use this in production / live code. In live code, the pool is
// meant to reset on a separate thread to avoid DoS vectors.
func (pool *LegacyPool) Clear() {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// unreserve each tracked account. Ideally, we could just clear the
	// reservation map in the parent txpool context. However, if we clear in
	// parent context, to avoid exposing the subpool lock, we have to lock the
	// reservations and then lock each subpool.
	//
	// This creates the potential for a deadlock situation:
	//
	// * TxPool.Clear locks the reservations
	// * a new transaction is received which locks the subpool mutex
	// * TxPool.Clear attempts to lock subpool mutex
	//
	// The transaction addition may attempt to reserve the sender addr which
	// can't happen until Clear releases the reservation lock. Clear cannot
	// acquire the subpool lock until the transaction addition is completed.

	for addr := range pool.pending {
		if _, ok := pool.queue[addr]; !ok {
			pool.reserver.Release(addr)
		}
	}
	for addr := range pool.queue {
		pool.reserver.Release(addr)
	}
	pool.all.Clear()
	pool.priced.Reheap()
	pool.pending = make(map[common.Address]*list)
	pool.queue = make(map[common.Address]*list)
	pool.pendingNonces = newNoncer(pool.currentState)
}

// HasPendingAuth returns a flag indicating whether there are pending
// authorizations from the specific address cached in the pool.
func (pool *LegacyPool) HasPendingAuth(addr common.Address) bool {
	return pool.all.hasAuth(addr)
}

// markTxPromoted adds the tx to the next valid pending txs set (i.e. to be
// included in the next block if selected by the application), pushes it onto
// the reap list so consensus picks it up, and records the pending-entry
// timestamps on the tracker.
func (pool *LegacyPool) markTxPromoted(addr common.Address, tx *types.Transaction) {
	pool.validPendingTxs.Do(func(store *TxStore) {
		store.AddTx(addr, tx)
	})
	if err := pool.reapList.PushEVMTx(tx); err != nil {
		log.Error("could not push promoted evm tx to ReapList", "err", err, "hash", tx.Hash())
	}
	hash := tx.Hash()
	_ = pool.tracker.ExitedQueued(hash)
	_ = pool.tracker.EnteredPending(hash)
}

// markTxReplaced runs the promotion side-effects for a tx that bypassed the
// queued pool (replaced an existing pending tx at the same nonce). It does
// **not** include the tx in the valid pending txs set, since replacements
// must be revalidated by the Rechecker before we rely on them.
func (pool *LegacyPool) markTxReplaced(_ common.Address, tx *types.Transaction) {
	if err := pool.reapList.PushEVMTx(tx); err != nil {
		log.Error("could not push replaced evm tx to ReapList", "err", err, "hash", tx.Hash())
	}
	hash := tx.Hash()
	_ = pool.tracker.ExitedQueued(hash)
	_ = pool.tracker.EnteredPending(hash)
}

// markTxRemoved records a removal from the given pool: removes the tx from
// the pending snapshot (if applicable), drops it from the reap list, and
// records the final duration on the tracker.
func (pool *LegacyPool) markTxRemoved(addr common.Address, tx *types.Transaction, p PoolType) {
	if p == Pending {
		pool.validPendingTxs.Do(func(store *TxStore) {
			store.RemoveTx(addr, tx)
		})
	}

	pool.reapList.DropEVMTx(tx)
	hash := tx.Hash()
	switch p {
	case Pending:
		_ = pool.tracker.RemovedFromPending(hash)
	case Queue:
		_ = pool.tracker.RemovedFromQueue(hash)
	}
}

// markTxEnqueued records the queued-entry timestamp on the tracker.
func (pool *LegacyPool) markTxEnqueued(tx *types.Transaction) {
	_ = pool.tracker.EnteredQueued(tx.Hash())
}

// tolerateRecheckErr returns nil if err is an error string that should be
// ignored from recheck, i.e. we do not want to drop txs from the mempool if we
// have received specific errors from recheck.
func tolerateRecheckErr(err error) error {
	// TODO: Fix import cycle if we try and properly match on
	// errors.Is(mempool.ErrNonceLow)
	if err != nil && strings.Contains(err.Error(), "tx nonce is higher than account nonce") {
		return nil
	}
	return err
}

func pendingRemovalMetric(reason txpool.RemovalReason) metric.Int64Counter {
	switch reason {
	case RemovalReasonLifetime:
		return pendingRemovedLifetime
	case RemovalReasonBelowTip:
		return pendingRemovedBelowTip
	case RemovalReasonTruncatedOverflow:
		return pendingRemovedTruncatedOverflow
	case RemovalReasonTruncatedLast:
		return pendingRemovedTruncatedLast
	case RemovalReasonUnderpricedFull:
		return pendingRemovedUnderpricedFull
	case RemovalReasonCapExceeded:
		return pendingRemovedCapped
	case RemovalReasonRunTxRecheck:
		return pendingRemovedRunTxRecheck
	case RemovalReasonRunTxFinalize:
		return pendingRemovedRunTxFinalize
	case RemovalReasonPrepareProposalInvalid:
		return pendingRemovedPrepareProposal
	}
	return pendingRemovedUnknown
}

func queueRemovalMetric(reason txpool.RemovalReason) metric.Int64Counter {
	switch reason {
	case RemovalReasonLifetime:
		return queuedRemovedLifetime
	case RemovalReasonBelowTip:
		return queuedRemovedBelowTip
	case RemovalReasonTruncatedOverflow:
		return queuedRemovedTruncatedOverflow
	case RemovalReasonTruncatedLast:
		return queuedRemovedTruncatedLast
	case RemovalReasonUnderpricedFull:
		return queuedRemovedUnderpricedFull
	case RemovalReasonCapExceeded:
		return queuedRemovedCapped
	case RemovalReasonRunTxRecheck:
		return queuedRemovedRunTxRecheck
	case RemovalReasonRunTxFinalize:
		return queuedRemovedRunTxFinalize
	case RemovalReasonPrepareProposalInvalid:
		return queuedRemovedPrepareProposal
	}
	return queuedRemovedUnknown
}
