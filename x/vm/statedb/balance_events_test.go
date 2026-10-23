package statedb

import (
	"testing"

	"github.com/stretchr/testify/require"

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
// "After any revert, every processed event index < current event count"
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

				// Mark the emitted event processed (simulates FlushToCacheCtx)
				n := len(cacheCtx.EventManager().Events())
				stateDB.MarkEventsProcessed(n-1, n)

				// Take snapshot after precompile
				snap := stateDB.Snapshot()
				snapshots = append(snapshots, snap)
			}

			// Revert to the target snapshot
			stateDB.RevertToSnapshot(snapshots[tc.revertToIndex])

			currentEventCount := len(cacheCtx.EventManager().Events())
			require.Equal(t, tc.expectedEvents, currentEventCount, "event count mismatch after revert")

			// Verify invariant: no processed range extends past the current events
			for _, r := range stateDB.processedEvents {
				require.LessOrEqual(t, r.end, currentEventCount,
					"processed range %v exceeds current event count %d", r, currentEventCount)
			}
			for i := 0; i < tc.numPrecompiles; i++ {
				require.Equal(t, i < tc.expectedEvents, stateDB.IsEventProcessed(i),
					"IsEventProcessed(%d)", i)
			}
		})
	}
}

func TestMarkEventsProcessed(t *testing.T) {
	type mark struct{ start, end int }
	testCases := []struct {
		name     string
		marks    []mark
		expected []eventRange
	}{
		{"empty range ignored", []mark{{3, 3}, {5, 2}, {-1, 2}}, nil},
		{"single", []mark{{2, 4}}, []eventRange{{2, 4}}},
		{"disjoint out of order", []mark{{6, 8}, {0, 2}, {3, 4}}, []eventRange{{0, 2}, {3, 4}, {6, 8}}},
		{"adjacent coalesce", []mark{{0, 2}, {2, 4}}, []eventRange{{0, 4}}},
		{"overlap spanning several", []mark{{0, 2}, {4, 5}, {7, 9}, {1, 8}}, []eventRange{{0, 9}}},
		{"contained", []mark{{0, 10}, {3, 5}}, []eventRange{{0, 10}}},
		{"single index", []mark{{1, 2}, {3, 4}, {2, 3}}, []eventRange{{1, 4}}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := &StateDB{}
			for _, m := range tc.marks {
				s.MarkEventsProcessed(m.start, m.end)
			}
			require.Equal(t, tc.expected, s.processedEvents)
			for i := -1; i < 12; i++ {
				want := false
				for _, r := range tc.expected {
					if r.start <= i && i < r.end {
						want = true
					}
				}
				require.Equal(t, want, s.IsEventProcessed(i), "IsEventProcessed(%d)", i)
			}
		})
	}
}

// TestRevertRestoresProcessedEvents checks that reverting a precompile call
// restores the processed ranges recorded before it, including ranges that
// were merged with ones added after the snapshot.
func TestRevertRestoresProcessedEvents(t *testing.T) {
	mockSnap := newMockSnapshotter()
	cacheCtx := sdk.Context{}.WithEventManager(sdk.NewEventManager())
	s := &StateDB{
		journal:     newJournal(),
		snapshotter: mockSnap,
		ctx:         sdk.Context{}.WithEventManager(sdk.NewEventManager()),
		cacheCtx:    cacheCtx,
		writeCache:  func() {},
	}
	emit := func(n int) {
		for i := 0; i < n; i++ {
			cacheCtx.EventManager().EmitEvent(sdk.NewEvent("test"))
		}
	}

	emit(2)
	s.MarkEventsProcessed(0, 2)
	emit(2) // indices 2,3 unprocessed

	rev := s.Snapshot()
	require.NoError(t, s.AddPrecompileFn(mockSnap.Snapshot()))
	emit(3)
	s.MarkEventsProcessed(1, 7) // merges with [0,2)
	require.Equal(t, []eventRange{{0, 7}}, s.processedEvents)

	s.RevertToSnapshot(rev)
	require.Len(t, cacheCtx.EventManager().Events(), 4)
	require.Equal(t, []eventRange{{0, 2}}, s.processedEvents)
	require.False(t, s.IsEventProcessed(2))
	require.False(t, s.IsEventProcessed(3))
}
