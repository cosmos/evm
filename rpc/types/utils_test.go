package types_test

import (
	"math/big"
	"testing"
	"time"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	cmttypes "github.com/cometbft/cometbft/types"

	rpctypes "github.com/cosmos/evm/rpc/types"
)

func TestEthHeaderFromComet_NegativeTimestamp(t *testing.T) {
	baseFee := big.NewInt(1000)
	bloom := ethtypes.Bloom{}

	tests := []struct {
		name         string
		time         time.Time
		expectedTime uint64
	}{
		{
			name:         "negative timestamp clamped to zero",
			time:         time.Date(1960, 1, 1, 0, 0, 0, 0, time.UTC),
			expectedTime: 0,
		},
		{
			name:         "zero timestamp",
			time:         time.Unix(0, 0).UTC(),
			expectedTime: 0,
		},
		{
			name:         "positive timestamp preserved",
			time:         time.Unix(1700000000, 0).UTC(),
			expectedTime: 1700000000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			header := cmttypes.Header{
				Height: 1,
				Time:   tc.time,
			}
			ethHeader := rpctypes.EthHeaderFromComet(header, bloom, baseFee)
			require.Equal(t, tc.expectedTime, ethHeader.Time)
		})
	}
}
