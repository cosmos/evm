package statedb

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// mockSnapshotter is a simple mock implementation of the Snapshotter interface for testing.
type mockSnapshotter struct {
	snapshots []int
	current   int
}

func newMockSnapshotter() *mockSnapshotter {
	return &mockSnapshotter{
		snapshots: []int{},
		current:   0,
	}
}

func (m *mockSnapshotter) Snapshot() int {
	id := m.current
	m.snapshots = append(m.snapshots, id)
	m.current++
	return id
}

func (m *mockSnapshotter) RevertToSnapshot(snapshot int) {
	// Find the snapshot index
	for i, s := range m.snapshots {
		if s == snapshot {
			m.snapshots = m.snapshots[:i+1]
			m.current = snapshot + 1
			return
		}
	}
}

// TestRevertToSnapshot_ProcessedEventsInvariant verifies the invariant:
// "After any revert, processedEventsCount <= current event count"
// This tests cacheCtx event manager behavior during EVM execution with precompile calls and reverts.
func TestRevertToSnapshot_ProcessedEventsInvariant(t *testing.T) {
	// Test each revert scenario independently since reverting invalidates future snapshots
	testCases := []struct {
		name           string
		numPrecompiles int
		revertToIndex  int
		expectedEvents int
	}{
		{"revert to 5 precompile calls", 10, 5, 5},
		{"revert to 2 precompile calls", 10, 2, 2},
		{"revert to 0 precompile calls", 10, 0, 0},
		{"revert to 8 precompile calls", 10, 8, 8},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockSnap := newMockSnapshotter()
			stateDB := &StateDB{
				validRevisions: []revision{},
				nextRevisionID: 0,
				journal:        newJournal(),
				snapshotter:    mockSnap,
			}

			ctx := sdk.Context{}.WithEventManager(sdk.NewEventManager())
			cacheCtx := sdk.Context{}.WithEventManager(sdk.NewEventManager())
			stateDB.ctx = ctx
			stateDB.cacheCtx = cacheCtx
			stateDB.writeCache = func() {} // Simulate cache exists (after first precompile call)

			// Snapshot with 0 events (before any precompile calls)
			snapshots := []int{stateDB.Snapshot()}

			// Simulate precompile calls - each emits an event
			for i := 0; i < tc.numPrecompiles; i++ {
				// Create multi-store snapshot for precompile journal entry
				multiStoreSnapshot := mockSnap.Snapshot()

				// Add precompile journal entry (captures events before precompile)
				err := stateDB.AddPrecompileFn(multiStoreSnapshot)
				require.NoError(t, err)

				// Emit event during "precompile execution"
				cacheCtx.EventManager().EmitEvent(sdk.NewEvent("test", sdk.NewAttribute("count", string(rune(i)))))

				// Update processed events count (simulates FlushToCacheCtx)
				stateDB.processedEventsCount = len(cacheCtx.EventManager().Events())

				// Take snapshot after precompile
				snap := stateDB.Snapshot()
				snapshots = append(snapshots, snap)
			}

			// Revert to the target snapshot
			stateDB.RevertToSnapshot(snapshots[tc.revertToIndex])

			currentEventCount := len(cacheCtx.EventManager().Events())
			require.Equal(t, tc.expectedEvents, currentEventCount, "event count mismatch after revert")

			// Verify invariant: processedEventsCount <= current event count
			require.LessOrEqual(t, stateDB.processedEventsCount, currentEventCount,
				"processedEventsCount %d exceeds current event count %d",
				stateDB.processedEventsCount, currentEventCount)

			require.Equal(t, tc.expectedEvents, stateDB.processedEventsCount,
				"processedEventsCount should match expected event count")
		})
	}
}

// TestAddPrecompileFn_CounterDoesNotWrap verifies that calls rejected by the
// precompile call limit do not advance the counter (regression test for the
// uint8 counter wraparound that reopened call slots after 256 attempts) and
// that reverting a precompile call still consumes the call limit.
func TestAddPrecompileFn_CounterDoesNotWrap(t *testing.T) {
	mockSnap := newMockSnapshotter()
	stateDB := &StateDB{
		validRevisions: []revision{},
		nextRevisionID: 0,
		journal:        newJournal(),
		snapshotter:    mockSnap,
	}
	stateDB.ctx = sdk.Context{}.WithEventManager(sdk.NewEventManager())
	stateDB.cacheCtx = sdk.Context{}.WithEventManager(sdk.NewEventManager())
	stateDB.writeCache = func() {}

	accepted := 0
	for i := 0; i < 512; i++ {
		err := stateDB.AddPrecompileFn(mockSnap.Snapshot())
		if i < int(types.MaxPrecompileCalls) {
			require.NoError(t, err, "call %d should be accepted", i)
			accepted++
		} else {
			require.Error(t, err, "call %d beyond the limit should be rejected", i)
		}
	}
	require.Equal(t, int(types.MaxPrecompileCalls), accepted)

	// A reverted accepted precompile call must still consume its quota.
	mockSnap2 := newMockSnapshotter()
	db2 := &StateDB{
		validRevisions: []revision{},
		nextRevisionID: 0,
		journal:        newJournal(),
		snapshotter:    mockSnap2,
	}
	db2.ctx = sdk.Context{}.WithEventManager(sdk.NewEventManager())
	db2.cacheCtx = sdk.Context{}.WithEventManager(sdk.NewEventManager())
	db2.writeCache = func() {}

	for i := 0; i < int(types.MaxPrecompileCalls)-1; i++ {
		require.NoError(t, db2.AddPrecompileFn(mockSnap2.Snapshot()))
	}
	// Snapshot before the last accepted call.
	snap := db2.Snapshot()
	require.NoError(t, db2.AddPrecompileFn(mockSnap2.Snapshot()))
	require.Error(t, db2.AddPrecompileFn(mockSnap2.Snapshot()), "limit reached")

	db2.RevertToSnapshot(snap)
	require.Error(t, db2.AddPrecompileFn(mockSnap2.Snapshot()),
		"a reverted call must still consume quota")
}
