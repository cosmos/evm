package suite

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"

	"github.com/cosmos/cosmos-sdk/testutil/systemtests"
)

// SetStakingUnbondingTime sets the staking unbonding time in genesis.
// This is useful for testing unbonding completion events without waiting for
// the default 21-day unbonding period.
func SetStakingUnbondingTime(t *testing.T, period time.Duration) systemtests.GenesisMutator {
	t.Helper()
	return func(genesis []byte) []byte {
		unbondingTimeStr := fmt.Sprintf("%ds", int64(period.Seconds()))
		state, err := sjson.SetRawBytes(genesis, "app_state.staking.params.unbonding_time", []byte(fmt.Sprintf("%q", unbondingTimeStr)))
		require.NoError(t, err)
		return state
	}
}
