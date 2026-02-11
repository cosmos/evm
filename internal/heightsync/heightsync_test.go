package heightsync_test

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/internal/heightsync"
)

// testValue is a simple value for testing the generic height-sync behavior.
type testValue struct {
	items []string
	mu    sync.Mutex
}

func newTestValue() *testValue {
	return &testValue{}
}

func (s *testValue) add(item string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
}

func (s *testValue) get() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.items...)
}

func TestBasicGetAfterCompletion(t *testing.T) {
	hv := heightsync.NewHeightSync(big.NewInt(1), newTestValue)

	hv.StartNewHeight(big.NewInt(1))
	hv.Do(func(s *testValue) {
		s.add("a")
		s.add("b")
	})
	hv.EndCurrentHeight()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	value := hv.GetValue(ctx, big.NewInt(1))
	require.NotNil(t, value)

	items := value.get()
	require.Len(t, items, 2)
	require.Equal(t, "a", items[0])
	require.Equal(t, "b", items[1])
}

func TestGetTimeoutBeforeHeight(t *testing.T) {
	hv := heightsync.NewHeightSync(big.NewInt(1), newTestValue)

	// request height 3 but don't advance to it
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	value := hv.GetValue(ctx, big.NewInt(3))
	require.Nil(t, value)
}

func TestGetPartialResults(t *testing.T) {
	synctest.Test(t, func(t *testing.T) { //nolint:thelper
		hv := heightsync.NewHeightSync(big.NewInt(1), newTestValue)

		// start new height but don't call EndCurrentHeight
		hv.StartNewHeight(big.NewInt(1))
		hv.Do(func(s *testValue) {
			s.add("partial")
		})

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		value := hv.GetValue(ctx, big.NewInt(1))
		require.NotNil(t, value)
		require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded) // ensure we waiting for context to timeout
		require.Equal(t, []string{"partial"}, value.get())
	})
}

func TestGetBehindByOneHeight(t *testing.T) {
	synctest.Test(t, func(t *testing.T) { //nolint:thelper
		hv := heightsync.NewHeightSync(big.NewInt(1), newTestValue)

		hv.StartNewHeight(big.NewInt(1))
		hv.Do(func(s *testValue) { s.add("height1") })

		// request height 2 in background
		valueChan := make(chan *testValue)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			valueChan <- hv.GetValue(ctx, big.NewInt(2))
		}()

		time.Sleep(1 * time.Second)

		hv.EndCurrentHeight()

		// advance to height 2
		hv.StartNewHeight(big.NewInt(2))
		hv.Do(func(s *testValue) { s.add("height2") })
		hv.EndCurrentHeight()

		value := <-valueChan
		require.NotNil(t, value)
		require.Equal(t, []string{"height2"}, value.get())
	})
}

func TestGetBehindByTwoHeights(t *testing.T) {
	synctest.Test(t, func(t *testing.T) { //nolint:thelper
		hv := heightsync.NewHeightSync(big.NewInt(1), newTestValue)

		hv.StartNewHeight(big.NewInt(1))
		hv.Do(func(s *testValue) { s.add("height1") })

		// request height 3 in background
		valueChan := make(chan *testValue)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			valueChan <- hv.GetValue(ctx, big.NewInt(3))
		}()

		time.Sleep(1 * time.Second)

		// advance through height 2
		hv.EndCurrentHeight()
		hv.StartNewHeight(big.NewInt(2))
		hv.Do(func(s *testValue) { s.add("height2") })

		time.Sleep(1 * time.Second)

		// advance to height 3
		hv.EndCurrentHeight()
		hv.StartNewHeight(big.NewInt(3))
		hv.Do(func(s *testValue) { s.add("height3") })
		hv.EndCurrentHeight()

		value := <-valueChan
		require.NotNil(t, value)
		require.Equal(t, []string{"height3"}, value.get())
	})
}

func TestPanicOnOldHeight(t *testing.T) {
	hv := heightsync.NewHeightSync(big.NewInt(1), newTestValue)

	hv.StartNewHeight(big.NewInt(1))
	hv.EndCurrentHeight()
	hv.StartNewHeight(big.NewInt(2))

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	require.Panics(t, func() {
		hv.GetValue(ctx, big.NewInt(1))
	})
}

func TestStartNewHeightResetsValue(t *testing.T) {
	hv := heightsync.NewHeightSync(big.NewInt(1), newTestValue)

	hv.StartNewHeight(big.NewInt(1))
	hv.Do(func(s *testValue) { s.add("old") })
	hv.EndCurrentHeight()

	// advance to height 2 - should get a fresh store
	hv.StartNewHeight(big.NewInt(2))
	hv.EndCurrentHeight()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result := hv.GetValue(ctx, big.NewInt(2))
	require.NotNil(t, result)
	require.Empty(t, result.get())
}

func TestConcurrentDo(t *testing.T) {
	hv := heightsync.NewHeightSync(big.NewInt(1), newTestValue)

	hv.StartNewHeight(big.NewInt(1))

	numOps := 100
	var wg sync.WaitGroup
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hv.Do(func(s *testValue) {
				s.add("x")
			})
		}()
	}

	wg.Wait()
	hv.EndCurrentHeight()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	result := hv.GetValue(ctx, big.NewInt(1))
	require.NotNil(t, result)
	require.Len(t, result.get(), numOps)
}
