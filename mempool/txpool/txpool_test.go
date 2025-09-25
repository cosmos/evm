package txpool_test

import (
	"sync"
	"testing"
	"time"

	"github.com/cosmos/evm/mempool/txpool"
	"github.com/cosmos/evm/mempool/txpool/mocks"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTxPool(t *testing.T) {
	gasTip := uint64(100)
	chain := mocks.NewBlockChain(t)
	subpool := mocks.NewSubPool(t)

	genesisHeader := &types.Header{}
	height1Header := &types.Header{ParentHash: genesisHeader.Hash()}
	height2Header := &types.Header{ParentHash: height1Header.Hash()}
	height3Header := &types.Header{ParentHash: height2Header.Hash()}

	// called during txpool initialization
	chain.On("CurrentBlock").Return(genesisHeader).Once()
	chain.On("StateAt", mock.Anything).Return(nil, nil)
	chain.On("Config").Return(&params.ChainConfig{ChainID: nil}).Once()

	subpool.On("Init", gasTip, genesisHeader, txpool.NewReservationTracker().NewHandle(0)).Return(nil).Once()
	subpool.On("Close").Return(nil).Once()

	// handle txpool subscribing to new head events from the chain. grab the
	// reference to the chan that it is going to wait on so we can push mock
	// headers during the test
	var wg sync.WaitGroup
	wg.Add(1)
	var newHeadCh *chan<- core.ChainHeadEvent
	chain.On("SubscribeChainHeadEvent", mock.Anything).Run(func(args mock.Arguments) {
		defer wg.Done()
		c := args.Get(0).(chan<- core.ChainHeadEvent)
		newHeadCh = &c
	}).Return(event.NewSubscription(func(c <-chan struct{}) error { return nil }))

	// SubPool.Reset will take 2 seconds before it returns. we need this to
	// take time to advance newHead in the background while we wait for this
	subpool.On("Reset", genesisHeader, height1Header).Run(func(args mock.Arguments) {
		<-time.After(1 * time.Second)
	}).Once()

	// pool loop is started async in New(), dont actually need a reference to
	// the pool
	pool, err := txpool.New(gasTip, chain, []txpool.SubPool{subpool})
	require.NoError(t, err)

	// wait for chain subscription to happen so we have a valid chan to push
	// headers to
	wg.Wait()

	// txpool loop is now waiting for a new header to come in, send it a header
	// for the first height.
	wg.Add(1)
	go func() {
		defer wg.Done()
		*newHeadCh <- core.ChainHeadEvent{Header: height1Header}
	}()

	//  this will take 5 seconds to process in the subpool
	wg.Wait()

	// send a few more headers to advance newHead
	wg.Add(1)
	go func() {
		defer wg.Done()
		*newHeadCh <- core.ChainHeadEvent{Header: height2Header}
		*newHeadCh <- core.ChainHeadEvent{Header: height3Header}
	}()

	// wait group just to make sure that we wait for subpool reset to run
	// before we exit to demonstrate the bug
	wg.Add(1)
	subpool.On("Reset", height1Header, height3Header).Run(func(args mock.Arguments) {
		defer wg.Done()
		require.Equal(t, height1Header.Hash(), height3Header.ParentHash, "sub pool reset got mismatch old head hash and new head parent hash")
	}).Once()

	// wait to make sure we have called subpool with the unexpected state
	wg.Wait()

	pool.Close()
}
