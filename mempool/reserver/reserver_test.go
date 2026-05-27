package reserver

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

//nolint:thelper
func TestReserver(t *testing.T) {
	accA := common.HexToAddress("0x123")
	accB := common.HexToAddress("0x456")
	t.Run("Basic", func(t *testing.T) {
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
				cosmosID, evmID := tracker.NewHandle(123), tracker.NewHandle(456)

				// ACT
				tt.run(t, tracker, cosmosID, evmID)
			})
		}
	})

	t.Run("WithRefCounter", func(t *testing.T) {
		// ARRANGE
		tracker := NewReservationTracker()
		cosmos := tracker.NewHandle(123, WithRefCounter())
		evm := tracker.NewHandle(456)

		// ACT
		// reserve A by cosmos x2
		err := cosmos.Hold(accA)
		require.NoError(t, err)

		err = cosmos.Hold(accA)
		require.NoError(t, err)

		// check that we have two reservations
		require.Equal(t, 2, cosmos.refsCounter[accA])

		// unreserve A by cosmos
		err = cosmos.Release(accA)
		require.NoError(t, err)

		// reserve by evm --> error
		err = evm.Hold(accA)
		require.ErrorIs(t, err, ErrAlreadyReserved)

		// check that we have one reservation
		require.Equal(t, 1, cosmos.refsCounter[accA])
		require.True(t, evm.Has(accA))

		// unserver by cosmos again
		err = cosmos.Release(accA)
		require.NoError(t, err)

		// reserve by evm -> OK
		err = evm.Hold(accA)
		require.NoError(t, err)

		// release by evm -> OK
		err = evm.Release(accA)
		require.NoError(t, err)

		// release by evm again -> error
		err = evm.Release(accA)
		require.ErrorContains(t, err, "not reserved")
	})
}
