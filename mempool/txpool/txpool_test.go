package txpool_test

import (
	"context"
	"math"
	"math/big"
	"sync"
	"testing"
	"time"

	"cosmossdk.io/log/v2"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/mempool/internal/reaplist"
	"github.com/cosmos/evm/mempool/internal/txtracker"
	"github.com/cosmos/evm/mempool/reserver"
	"github.com/cosmos/evm/mempool/txpool"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	legacypool_mocks "github.com/cosmos/evm/mempool/txpool/legacypool/mocks"
	statedb_mocks "github.com/cosmos/evm/x/vm/statedb/mocks"

	"github.com/cosmos/evm/mempool/txpool/mocks"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestTxPoolCosmosReorg is a regression test for when slow processing of the
// legacypool run reorg function (as a subpool) would cause a panic if new
// headers are produced during this slow processing.
//
// Here we are using the legacypool as a subpool of the txpool. We then add tx
// to the mempool, and simulate a long broadcast to the comet mempool via
// overriding the BroadcastFn. We then advance the chain 3 blocks by sending
// three headers on the newHeadCh. This will then cause runReorg to be run with
// a newHead that is at oldHead + 3. Previously, this incorrectly was seen as a
// reorg by the legacypool, and would call GetBlock on the mempools BlockChain,
// which would cause a panic.

// NOTE: we are using a mocked BlockChain impl here, but are simply manually
// making any calls to GetBlock panic).
func TestTxPoolCosmosReorg(t *testing.T) {
	gasTip := uint64(100)
	gasLimit := uint64(1_000_000)

	// mock tx signer and priv key
	signer := types.HomesteadSigner{}
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	// the blockchain interface that the legacypool and txpool want are
	// slightly different, so sadly we have to use two different mocks for this
	chain := mocks.NewBlockChain(t)
	legacyChain := legacypool_mocks.NewBlockChain(t)
	genesisState := statedb_mocks.NewStateDB(t)

	// simulated headers on chain
	genesisHeader := &types.Header{GasLimit: gasLimit, Difficulty: big.NewInt(1), Number: big.NewInt(0)}
	height1Header := &types.Header{ParentHash: genesisHeader.Hash(), GasLimit: gasLimit, Difficulty: big.NewInt(1), Number: big.NewInt(1)}
	height2Header := &types.Header{ParentHash: height1Header.Hash(), GasLimit: gasLimit, Difficulty: big.NewInt(1), Number: big.NewInt(2)}
	height3Header := &types.Header{ParentHash: height2Header.Hash(), GasLimit: gasLimit, Difficulty: big.NewInt(1), Number: big.NewInt(3)}

	// called during legacypool initialization
	cfg := &params.ChainConfig{ChainID: nil}
	legacyChain.On("Config").Return(cfg)
	legacyChain.On("CurrentBlock").Return(&types.Header{Number: big.NewInt(0)})
	chain.On("Config").Return(cfg)
	legacyChain.On("StateAt", genesisHeader.Root).Return(genesisState, nil)
	legacyChain.On("GetLatestContext").Return(sdk.Context{}, nil).Maybe()
	chain.On("StateAt", genesisHeader.Root).Return(nil, nil)

	// starting the chain(s) at genesisHeader
	chain.On("CurrentBlock").Return(genesisHeader)

	// we have to mock this, but this matches the behavior of the real impl if
	// GetBlock is called
	legacyChain.On("GetBlock", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		panic("GetBlock called means reorg detected when there was not one!")
	}).Maybe()

	// all accounts have max balance at genesis
	genesisState.On("GetBalance", mock.Anything).Return(uint256.NewInt(math.MaxUint64))
	genesisState.On("GetNonce", mock.Anything).Return(uint64(1))
	genesisState.On("GetCodeHash", mock.Anything).Return(types.EmptyCodeHash)

	recheckGuard := make(chan struct{})
	legacyPool := legacypool.New(legacypool.DefaultConfig, log.NewNopLogger(), legacyChain, reaplist.New(stubTxEncoder{}, reaplist.Unbounded, nil), txtracker.New(), legacypool.WithRecheck(&BlockingRechecker{guard: recheckGuard}))

	// handle txpool subscribing to new head events from the chain. grab the
	// reference to the chan that it is going to wait on so we can push mock
	// headers during the test
	waitForSubscription := make(chan struct{}, 1)
	var newHeadCh chan<- core.ChainHeadEvent
	chain.On("SubscribeChainHeadEvent", mock.Anything).Run(func(args mock.Arguments) {
		newHeadCh = args.Get(0).(chan<- core.ChainHeadEvent)
		waitForSubscription <- struct{}{}
	}).Return(event.NewSubscription(func(c <-chan struct{}) error { return nil }))

	tracker := reserver.NewReservationTracker()
	pool, err := txpool.New(gasTip, chain, tracker, []txpool.SubPool{legacyPool})
	require.NoError(t, err)
	defer pool.Close()

	// wait for newHeadCh to be initialized
	<-waitForSubscription

	// add tx1 to the pool so that the blocking recheck fn will be called,
	// simulating a slow runReorg
	tx1, _ := types.SignTx(types.NewTransaction(1, common.Address{}, big.NewInt(100), 100_000, big.NewInt(int64(gasTip)+1), nil), signer, key)
	errs := pool.Add([]*types.Transaction{tx1}, false)
	for _, err := range errs {
		require.NoError(t, err)
	}

	// recheck fn is now blocking, waiting for recheckGuard

	// during this time, we will simulate advancing the chain multiple times by
	// sending headers on the newHeadCh
	newHeadCh <- core.ChainHeadEvent{Header: height1Header}
	newHeadCh <- core.ChainHeadEvent{Header: height2Header}
	newHeadCh <- core.ChainHeadEvent{Header: height3Header}

	// now that we have advanced the headers, unblock the recheck fn
	recheckGuard <- struct{}{}

	// a runReorg call will now be scheduled with oldHead=genesis and
	// newHead=height3

	time.Sleep(500 * time.Millisecond)

	// sync the pool to make sure that runReorg has processed the above headers
	require.NoError(t, pool.Sync())
}

type BlockingRechecker struct {
	guard chan struct{}
	once  sync.Once
}

func (mr *BlockingRechecker) GetContext() (sdk.Context, func()) {
	return sdk.Context{}, func() {}
}

func (mr *BlockingRechecker) RecheckEVM(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
	mr.once.Do(func() {
		<-mr.guard
	})
	return sdk.Context{}, nil
}

func (mr *BlockingRechecker) Update(ctx sdk.Context, header *types.Header) {}

// newTxPool builds a TxPool with the given mock subpools, bypassing New() to
// avoid needing a full BlockChain mock.
func newTxPool(subpools ...txpool.SubPool) *txpool.TxPool {
	return &txpool.TxPool{Subpools: subpools}
}

func TestTxPoolPending_MergesSubpools(t *testing.T) {
	addr1 := common.HexToAddress("0x1")
	addr2 := common.HexToAddress("0x2")
	filter := txpool.PendingFilter{}
	ctx := context.Background()

	lazy1 := &txpool.LazyTransaction{}
	lazy2 := &txpool.LazyTransaction{}

	sub1 := mocks.NewSubPool(t)
	sub1.On("Pending", ctx, filter).Return(map[common.Address][]*txpool.LazyTransaction{addr1: {lazy1}})

	sub2 := mocks.NewSubPool(t)
	sub2.On("Pending", ctx, filter).Return(map[common.Address][]*txpool.LazyTransaction{addr2: {lazy2}})

	result := newTxPool(sub1, sub2).Pending(ctx, filter)

	require.Len(t, result, 2)
	require.Equal(t, []*txpool.LazyTransaction{lazy1}, result[addr1])
	require.Equal(t, []*txpool.LazyTransaction{lazy2}, result[addr2])
}

func TestTxPoolPending_EmptySubpools(t *testing.T) {
	filter := txpool.PendingFilter{}
	ctx := context.Background()

	sub := mocks.NewSubPool(t)
	sub.On("Pending", ctx, filter).Return(map[common.Address][]*txpool.LazyTransaction{})

	require.Empty(t, newTxPool(sub).Pending(ctx, filter))
}

func TestTxPoolRechecked_MergesSubpools(t *testing.T) {
	addr1 := common.HexToAddress("0x1")
	addr2 := common.HexToAddress("0x2")
	height := big.NewInt(10)
	filter := txpool.PendingFilter{}
	ctx := context.Background()

	lazy1 := &txpool.LazyTransaction{}
	lazy2 := &txpool.LazyTransaction{}

	sub1 := mocks.NewSubPool(t)
	sub1.On("Rechecked", ctx, height, filter).Return(map[common.Address][]*txpool.LazyTransaction{addr1: {lazy1}})

	sub2 := mocks.NewSubPool(t)
	sub2.On("Rechecked", ctx, height, filter).Return(map[common.Address][]*txpool.LazyTransaction{addr2: {lazy2}})

	result := newTxPool(sub1, sub2).Rechecked(ctx, height, filter)

	require.Len(t, result, 2)
	require.Equal(t, []*txpool.LazyTransaction{lazy1}, result[addr1])
	require.Equal(t, []*txpool.LazyTransaction{lazy2}, result[addr2])
}

func TestTxPoolRechecked_EmptySubpools(t *testing.T) {
	height := big.NewInt(10)
	filter := txpool.PendingFilter{}
	ctx := context.Background()

	sub := mocks.NewSubPool(t)
	sub.On("Rechecked", ctx, height, filter).Return(map[common.Address][]*txpool.LazyTransaction{})

	require.Empty(t, newTxPool(sub).Rechecked(ctx, height, filter))
}

func TestTxPoolContent_MergesSubpools(t *testing.T) {
	addr1 := common.HexToAddress("0x1")
	addr2 := common.HexToAddress("0x2")

	tx1 := types.NewTransaction(0, addr1, big.NewInt(0), 21000, big.NewInt(1), nil)
	tx2 := types.NewTransaction(0, addr2, big.NewInt(0), 21000, big.NewInt(1), nil)

	sub1 := mocks.NewSubPool(t)
	sub1.On("Content").Return(
		map[common.Address][]*types.Transaction{addr1: {tx1}},
		map[common.Address][]*types.Transaction{},
	)

	sub2 := mocks.NewSubPool(t)
	sub2.On("Content").Return(
		map[common.Address][]*types.Transaction{},
		map[common.Address][]*types.Transaction{addr2: {tx2}},
	)

	runnable, blocked := newTxPool(sub1, sub2).Content()

	require.Len(t, runnable, 1)
	require.Equal(t, []*types.Transaction{tx1}, runnable[addr1])
	require.Len(t, blocked, 1)
	require.Equal(t, []*types.Transaction{tx2}, blocked[addr2])
}

func TestTxPoolContent_EmptySubpools(t *testing.T) {
	sub := mocks.NewSubPool(t)
	sub.On("Content").Return(
		map[common.Address][]*types.Transaction{},
		map[common.Address][]*types.Transaction{},
	)

	runnable, blocked := newTxPool(sub).Content()
	require.Empty(t, runnable)
	require.Empty(t, blocked)
}

// minimal tx encoder to construct a testing reaplist
type stubTxEncoder struct{}

func (stubTxEncoder) EVMTx(tx *types.Transaction) ([]byte, error) { return tx.Hash().Bytes(), nil }
func (stubTxEncoder) CosmosTx(sdk.Tx) ([]byte, error)             { return nil, nil }
