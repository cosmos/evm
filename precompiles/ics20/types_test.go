package ics20

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/testutil"
	transfertypes "github.com/cosmos/ibc-go/v11/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v11/modules/core/02-client/types"
	ibcerrors "github.com/cosmos/ibc-go/v11/modules/core/errors"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
)

func TestNewDenomsRequest(t *testing.T) {
	method, ok := ABI.Methods[DenomsMethod]
	require.True(t, ok)

	pageRequest := query.PageRequest{
		Key:        []byte{1, 2, 3},
		Offset:     4,
		Limit:      5,
		CountTotal: true,
		Reverse:    true,
	}
	packed, err := method.Inputs.Pack(pageRequest)
	require.NoError(t, err)
	args, err := method.Inputs.Unpack(packed)
	require.NoError(t, err)

	t.Run("valid pagination", func(t *testing.T) {
		req, err := NewDenomsRequest(&method, args)

		require.NoError(t, err)
		require.NotNil(t, req)
		require.Equal(t, &pageRequest, req.Pagination)
	})

	t.Run("invalid pagination", func(t *testing.T) {
		req, err := NewDenomsRequest(&method, []interface{}{"bad-pagination"})
		wantErr := cmn.NewRevertWithSolidityError(
			ABI,
			cmn.SolidityErrInvalidPageRequest,
			DenomsMethod,
			big.NewInt(0),
			"bad-pagination",
		)

		testutil.RequireExactError(t, err, wantErr)
		require.Nil(t, req)
	})
}

func TestNewMsgTransferCoinValidationErrors(t *testing.T) {
	method := ABI.Methods[TransferMethod]
	sender := common.HexToAddress("0x1234567890123456789012345678901234567890")
	receiver := sdk.AccAddress(sender.Bytes()).String()
	for _, tc := range []struct {
		name, denom                               string
		amount                                    int64
		want                                      string
		invalidChannel, invalidReceiver, longMemo bool
	}{
		{name: "zero amount", denom: "uatom", want: SolidityErrIBCTransferInvalidAmount},
		{name: "invalid base denom", denom: "a", amount: 1, want: SolidityErrIBCTransferInvalidDenom},
		{name: "invalid IBC hash", denom: "ibc/not-a-hash", amount: 1, want: SolidityErrIBCTransferInvalidDenom},
		{name: "missing IBC hash", denom: "ibc", amount: 1, want: SolidityErrIBCTransferInvalidDenom},
		{name: "base denom before zero amount", denom: "a", want: SolidityErrIBCTransferInvalidDenom},
		{name: "zero amount before IBC hash", denom: "ibc/not-a-hash", want: SolidityErrIBCTransferInvalidAmount},
		{name: "coin before memo length", denom: "uatom", longMemo: true, want: SolidityErrIBCTransferInvalidAmount},
		{name: "channel before coin", denom: "a", invalidChannel: true, want: SolidityErrInvalidSourceChannel},
		{name: "receiver before coin", denom: "a", invalidReceiver: true, want: SolidityErrInvalidReceiver},
		{name: "valid base denom", denom: "uatom", amount: 1},
		{name: "valid native denom path", denom: "gamm/pool/1", amount: 1},
		{name: "valid IBC denom", denom: "ibc/" + strings.Repeat("A", 64), amount: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			channel, recipient, memo := "channel-0", receiver, ""
			var wantArgs []interface{}
			if tc.invalidChannel {
				channel = "invalid/channel"
				wantArgs = []interface{}{TransferMethod, ErrInvalidSourceChannel}
			}
			if tc.invalidReceiver {
				recipient = ""
				wantArgs = []interface{}{TransferMethod, "invalid receiver: "}
			}
			if tc.longMemo {
				memo = strings.Repeat("x", transfertypes.MaximumMemoLength+1)
			}
			amount := big.NewInt(tc.amount)
			height := clienttypes.NewHeight(0, 1)
			encoded, err := method.Inputs.Pack("transfer", channel, tc.denom, amount, sender, recipient, height, uint64(0), memo)
			require.NoError(t, err)
			args, err := method.Inputs.Unpack(encoded)
			require.NoError(t, err)
			msg, actualSender, err := NewMsgTransfer(&method, args)
			if tc.want == "" {
				require.NoError(t, err)
				require.NotNil(t, msg)
				require.Equal(t, sender, actualSender)
				require.Equal(t, tc.denom, msg.Token.Denom)
				require.Equal(t, amount, msg.Token.Amount.BigInt())
				return
			}
			require.Error(t, err)
			require.Nil(t, msg)
			definition := ABI.Errors[tc.want]
			payload, packErr := definition.Inputs.Pack(wantArgs...)
			require.NoError(t, packErr)
			expected := append(append([]byte{}, definition.ID[:4]...), payload...)
			require.Equal(t, expected, err.(cmn.RevertDataCarrier).RevertData())
			if tc.want == SolidityErrIBCTransferInvalidAmount || tc.want == SolidityErrIBCTransferInvalidDenom {
				_, raw := CreateAndValidateMsgTransfer("transfer", channel, sdk.Coin{Denom: tc.denom, Amount: sdkmath.NewInt(tc.amount)}, receiver, recipient, height, 0, memo)
				require.ErrorIs(t, raw, ibcerrors.ErrInvalidCoins, "the exported raw validation helper retains its SDK error contract")
			}
		})
	}
}
