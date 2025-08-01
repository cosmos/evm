package types_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/vm/types"
)

func TestNewExecErrorWithReason(t *testing.T) {
	testCases := []struct {
		name         string
		errorMessage string
		revertReason []byte
		data         string
	}{
		{
			"Empty reason",
			"execution reverted",
			nil,
			"0x",
		},
		{
			"With unpackable reason",
			"execution reverted",
			[]byte("a"),
			"0x61",
		},
		{
			"With packable reason but empty reason",
			"execution reverted",
			types.RevertSelector,
			"0x08c379a0",
		},
		{
			"With packable reason with reason",
			"execution reverted: COUNTER_TOO_LOW",
			hexutil.MustDecode("0x08C379A00000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000F434F554E5445525F544F4F5F4C4F570000000000000000000000000000000000"),
			"0x08c379a00000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000f434f554e5445525f544f4f5f4c4f570000000000000000000000000000000000",
		},
	}

	for _, tc := range testCases {
		errWithReason := types.NewExecErrorWithReason(tc.revertReason)
		require.Equal(t, tc.errorMessage, errWithReason.Error())
		require.Equal(t, tc.data, errWithReason.ErrorData())
		require.Equal(t, 3, errWithReason.ErrorCode())
	}
}
