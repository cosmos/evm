package reserver

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

//nolint:thelper
func TestReserver(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		accA := common.HexToAddress("0x123")
		accB := common.HexToAddress("0x456")

		for _, tt := range []struct {
			name string
			run  func(t *testing.T, tracker *ReservationTracker, cosmos, evm *ReservationHandle)
		}{
			{
				name: "holdAlreadyReserved",
				run: func(t *testing.T, tracker *ReservationTracker, cosmos, evm *ReservationHandle) {
					// ACT
					require.NoError(t, evm.Hold(accA))
					err := cosmos.Hold(accA)

					// ASSERT
					require.ErrorIs(t, err, ErrAlreadyReserved)
					require.True(t, cosmos.Has(accA))
					require.False(t, evm.Has(accA))
				},
			},
			{
				name: "releaseNotReserved",
				run: func(t *testing.T, tracker *ReservationTracker, cosmos, evm *ReservationHandle) {
					// ACT
					err := cosmos.Release(accA)

					// ASSERT
					require.ErrorContains(t, err, "not reserved")
					require.False(t, cosmos.Has(accA))
					require.False(t, evm.Has(accA))
				},
			},
			{
				name: "releaseNotOwned",
				run: func(t *testing.T, tracker *ReservationTracker, cosmos, evm *ReservationHandle) {
					// ACT
					require.NoError(t, evm.Hold(accA))
					err := cosmos.Release(accA)

					// ASSERT
					require.ErrorContains(t, err, "not owned by sub-pool")
					require.True(t, cosmos.Has(accA))
					require.False(t, evm.Has(accA))
				},
			},
			{
				name: "holdMultipleAlreadyReservedIsAtomic",
				run: func(t *testing.T, tracker *ReservationTracker, cosmos, evm *ReservationHandle) {
					// ACT
					require.NoError(t, evm.Hold(accA))
					err := cosmos.Hold(accB, accA)

					// ASSERT
					require.ErrorIs(t, err, ErrAlreadyReserved)
					require.ErrorContains(t, err, accA.String())

					require.True(t, cosmos.Has(accA))
					require.False(t, evm.Has(accA))
					require.False(t, cosmos.Has(accB))
					require.False(t, evm.Has(accB))
				},
			},
			{
				name: "releaseMultipleNotOwnedIsAtomic",
				run: func(t *testing.T, tracker *ReservationTracker, cosmos, evm *ReservationHandle) {
					// ACT
					require.NoError(t, evm.Hold(accA))
					require.NoError(t, cosmos.Hold(accB))
					err := cosmos.Release(accB, accA)

					// ASSERT
					require.ErrorContains(t, err, "not owned by sub-pool")
					require.True(t, cosmos.Has(accA))
					require.False(t, evm.Has(accA))
					require.False(t, cosmos.Has(accB))
					require.True(t, evm.Has(accB))
				},
			},
			{
				name: "holdRelease",
				run: func(t *testing.T, tracker *ReservationTracker, cosmos, evm *ReservationHandle) {
					// ACT #1
					require.NoError(t, cosmos.Hold(accA))

					// ASSERT #1
					require.False(t, cosmos.Has(accA))
					require.True(t, evm.Has(accA))

					// ACT #2
					require.NoError(t, cosmos.Release(accA))

					// ASSERT #2
					require.False(t, cosmos.Has(accA))
					require.False(t, evm.Has(accA))
				},
			},
			{
				name: "holdSeparately",
				run: func(t *testing.T, tracker *ReservationTracker, cosmos, evm *ReservationHandle) {
					// ACT #1
					require.NoError(t, cosmos.Hold(accA))

					// ASSERT #1
					require.False(t, cosmos.Has(accA))

					// ACT #2
					require.NoError(t, evm.Hold(accB))

					// ASSERT #2
					require.False(t, evm.Has(accB))

					require.True(t, cosmos.Has(accB))
					require.True(t, evm.Has(accA))
				},
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				// ARRANGE
				// Given reserver
				tracker := NewReservationTracker()

				// Create handles
				cosmosID, evmID := tracker.NewHandle(CosmosReserverHandlerID), tracker.NewHandle(0)

				// ACT
				tt.run(t, tracker, cosmosID, evmID)
			})
		}
	})
}
