package eip712

import (
	"testing"

	"github.com/stretchr/testify/require"

	basev1beta1 "cosmossdk.io/api/cosmos/base/v1beta1"
	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func TestToAnyMsgs(t *testing.T) {
	msg := &banktypes.MsgSend{
		FromAddress: "cosmos1x8fhpj9nmhqk8z9kpgjt95ck2xwyue0ptzkucp",
		ToAddress:   "cosmos1dx67l23hz9l0k9hcher8xz04uj7wf3yu26l2yn",
		Amount:      sdk.Coins{sdk.Coin{Amount: math.NewInt(10), Denom: "atest"}},
	}
	expectedAny, err := types.NewAnyWithValue(msg)
	require.NoError(t, err)
	testCases := []struct {
		name      string
		msgs      []sdk.Msg
		wantLen   int
		wantError bool
	}{
		{
			name:      "single valid message",
			msgs:      []sdk.Msg{msg},
			wantLen:   1,
			wantError: false,
		},
		{
			name:      "empty slice",
			msgs:      []sdk.Msg{},
			wantLen:   0,
			wantError: false,
		},
		{
			name:      "invalid message (nil)",
			msgs:      []sdk.Msg{nil},
			wantLen:   0,
			wantError: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			anyMsgs, err := ToAnyMsgs(tc.msgs)
			if tc.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, anyMsgs, tc.wantLen)
				if tc.wantLen == 1 {
					require.Equal(t, expectedAny.TypeUrl, anyMsgs[0].TypeUrl)
					require.Equal(t, expectedAny.Value, anyMsgs[0].Value)
				}
			}
		})
	}
}

func TestToFeeAmount(t *testing.T) {
	const denom1 = "atest1"
	const denom2 = "atest2"
	testCases := []struct {
		name     string
		coins    sdk.Coins
		expected []*basev1beta1.Coin
	}{
		{
			name:     "single coin",
			coins:    sdk.NewCoins(sdk.NewInt64Coin(denom1, 100)),
			expected: []*basev1beta1.Coin{{Denom: denom1, Amount: "100"}},
		},
		{
			name:     "multiple coins",
			coins:    sdk.NewCoins(sdk.NewInt64Coin(denom1, 100), sdk.NewInt64Coin(denom2, 50)),
			expected: []*basev1beta1.Coin{{Denom: denom1, Amount: "100"}, {Denom: denom2, Amount: "50"}},
		},
		{
			name:     "empty coins",
			coins:    sdk.NewCoins(),
			expected: []*basev1beta1.Coin{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ToFeeAmount(tc.coins)
			require.Equal(t, tc.expected, result)
		})
	}
}
