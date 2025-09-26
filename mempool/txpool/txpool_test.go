package txpool_test

import (
	"math"
	"math/big"
	"testing"
	"time"

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
	chain.On("Config").Return(cfg)
	legacyChain.On("StateAt", genesisHeader.Root).Return(genesisState, nil)
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

	legacyPool := legacypool.New(legacypool.DefaultConfig, legacyChain)

	// handle txpool subscribing to new head events from the chain. grab the
	// reference to the chan that it is going to wait on so we can push mock
	// headers during the test
	waitForSubscription := make(chan struct{}, 1)
	var newHeadCh chan<- core.ChainHeadEvent
	chain.On("SubscribeChainHeadEvent", mock.Anything).Run(func(args mock.Arguments) {
		newHeadCh = args.Get(0).(chan<- core.ChainHeadEvent)
		waitForSubscription <- struct{}{}
	}).Return(event.NewSubscription(func(c <-chan struct{}) error { return nil }))

	pool, err := txpool.New(gasTip, chain, []txpool.SubPool{legacyPool})
	require.NoError(t, err)
	defer pool.Close()

	// wait for newHeadCh to be initialized
	<-waitForSubscription

	// override broadcast fn to wait until we advance the chain a few blocks
	broadcastGuard := make(chan struct{})
	legacyPool.BroadcastTxFn = func(txs []*types.Transaction) error {
		<-broadcastGuard
		return nil
	}

	// add tx1 to the pool so that the blocking broadcast fn will be called,
	// simulating a slow runReorg
	tx1, _ := types.SignTx(types.NewTransaction(1, common.Address{}, big.NewInt(100), 100_000, big.NewInt(int64(gasTip)+1), nil), signer, key)
	errs := pool.Add([]*types.Transaction{tx1}, false)
	for _, err := range errs {
		require.NoError(t, err)
	}

	// broadcast fn is now blocking, waiting for broadcastGuard

	// during this time, we will simulate advancing the chain multiple times by
	// sending headers on the newHeadCh
	newHeadCh <- core.ChainHeadEvent{Header: height1Header}
	newHeadCh <- core.ChainHeadEvent{Header: height2Header}
	newHeadCh <- core.ChainHeadEvent{Header: height3Header}

	// now that we have advanced the headers, unblock the broadcast fn
	broadcastGuard <- struct{}{}

	// a runReorg call will now be scheduled with oldHead=genesis and
	// newHead=height3

	time.Sleep(500 * time.Millisecond)

	// push another tx to make sure that runReorg was processed with the above
	// headers
	legacyPool.BroadcastTxFn = func(txs []*types.Transaction) error { return nil }
	tx2, _ := types.SignTx(types.NewTransaction(2, common.Address{}, big.NewInt(100), 100_000, big.NewInt(int64(gasTip)+1), nil), signer, key)
	errs = pool.Add([]*types.Transaction{tx2}, false)
	for _, err := range errs {
		require.NoError(t, err)
	}
}
