package ics20

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
	callbackstypes "github.com/cosmos/evm/x/ibc/callbacks/types"
	transfertypes "github.com/cosmos/ibc-go/v11/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v11/modules/core/02-client/types"
	clienttypesv2 "github.com/cosmos/ibc-go/v11/modules/core/02-client/v2/types"
	connectiontypes "github.com/cosmos/ibc-go/v11/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v11/modules/core/04-channel/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v11/modules/core/04-channel/v2/types"
	ibcerrors "github.com/cosmos/ibc-go/v11/modules/core/errors"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var errSyntheticICS20Drift = errorsmod.Register("ics20-phase-three-drift", 77, "unstable reason")

func TestTranslateICS20RegisteredErrorsDirectAndWrapped(t *testing.T) {
	p := Precompile{ABI: ABI}
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())
	tests := []struct {
		err  error
		name string
	}{
		{clienttypes.ErrClientNotActive, SolidityErrIBCClientNotActive},
		{channeltypes.ErrChannelNotFound, SolidityErrIBCChannelNotFound},
		{channeltypes.ErrInvalidChannelState, SolidityErrIBCChannelInvalidState},
		{connectiontypes.ErrConnectionNotFound, SolidityErrIBCConnectionNotFound},
		{connectiontypes.ErrInvalidConnectionState, SolidityErrIBCConnectionInvalidState},
		{transfertypes.ErrInvalidDenomForTransfer, SolidityErrIBCTransferInvalidDenom},
		{transfertypes.ErrInvalidAmount, SolidityErrIBCTransferInvalidAmount},
		{transfertypes.ErrDenomNotFound, SolidityErrIBCTransferDenomNotFound},
		{transfertypes.ErrSendDisabled, SolidityErrIBCTransferSendDisabled},
		{transfertypes.ErrInvalidMemo, SolidityErrIBCTransferInvalidMemo},
		{channeltypes.ErrSequenceSendNotFound, SolidityErrIBCChannelSequenceSendNotFound},
		{clienttypes.ErrInvalidHeight, SolidityErrIBCClientInvalidHeight},
		{channeltypes.ErrTimeoutElapsed, SolidityErrIBCChannelTimeoutElapsed},
		{clienttypesv2.ErrCounterpartyNotFound, SolidityErrIBCClientV2CounterpartyNotFound},
		{channeltypesv2.ErrInvalidPacket, SolidityErrIBCChannelV2InvalidPacket},
		{channeltypesv2.ErrSequenceSendNotFound, SolidityErrIBCChannelV2SequenceSendNotFound},
		{channeltypesv2.ErrInvalidTimeout, SolidityErrIBCChannelV2InvalidTimeout},
		{channeltypesv2.ErrTimeoutElapsed, SolidityErrIBCChannelV2TimeoutElapsed},
		{ibcerrors.ErrUnauthorized, SolidityErrIBCUnauthorized},
		{callbackstypes.ErrNestedSourceCallbackTransfer, SolidityErrIBCCallbacksNestedSourceTransfer},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, returned := range []error{
				tc.err,
				errorsmod.Wrap(tc.err, "message changed"),
				fmt.Errorf("standard wrapper: %w", tc.err),
			} {
				got := p.ics20MsgError(ctx, returned)
				require.Equal(t, ics20ErrorSelector(tc.name), got.(cmn.RevertDataCarrier).RevertData())
				assertICS20NotFallback(t, got)
			}
		})
	}
}

func TestICS20UnregisteredMsgAndQueryFailuresKeepLegacyFallbacks(t *testing.T) {
	p := Precompile{ABI: ABI}
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())
	plain := errors.New("infrastructure")

	msgErr := p.ics20MsgError(ctx, plain)
	require.Equal(t, ics20ErrorSelector(cmn.SolidityErrMsgServerFailed), msgErr.(cmn.RevertDataCarrier).RevertData()[:4])

	queryErr := p.ics20QueryError(ctx, DenomsMethod, plain)
	require.Equal(t, ics20ErrorSelector(cmn.SolidityErrQueryFailed), queryErr.(cmn.RevertDataCarrier).RevertData()[:4])
}

func TestICS20ValidatedInputMapsPublishedError(t *testing.T) {
	p := Precompile{ABI: ABI}
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())

	mapped := p.ics20ValidatedInputError(ctx, transfertypes.ErrInvalidMemo)
	require.Equal(t, ics20ErrorSelector(SolidityErrIBCTransferInvalidMemo), mapped.(cmn.RevertDataCarrier).RevertData())
	assertICS20NotFallback(t, mapped)
}

func TestICS20ValidatedInputUnmappedProtocolErrorLogsOnceAndReturnsUnmappedRevert(t *testing.T) {
	var output bytes.Buffer
	ctx := sdk.Context{}.WithLogger(log.NewLogger(&output, log.OutputJSONOption()))
	p := Precompile{ABI: ABI}
	returned := fmt.Errorf("validation wrapper with changed message: %w", errSyntheticICS20Drift)
	classification := cmn.TranslateCosmosError(ABI, cosmosErrorRegistry, returned)
	require.Equal(t, cmn.MappingKindUnmapped, classification.Kind)
	key, ok := cmn.ExtractCosmosErrorKey(returned)
	require.True(t, ok)

	err := p.ics20ValidatedInputError(ctx, returned)
	expected := classification.Revert
	require.Equal(t, expected.(cmn.RevertDataCarrier).RevertData(), err.(cmn.RevertDataCarrier).RevertData())

	logs := output.String()
	require.Equal(t, 1, strings.Count(logs, "unmapped registered Cosmos error"))
	require.Contains(t, logs, `"precompile":"ics20"`)
	require.Contains(t, logs, `"method":"transfer"`)
	require.Contains(t, logs, fmt.Sprintf(`"codespace":"%s"`, key.Codespace))
	require.Contains(t, logs, fmt.Sprintf(`"code":%d`, key.Code))
	require.NotContains(t, logs, "unstable reason")
	require.NotContains(t, logs, "validation wrapper with changed message")

	_ = p.ics20ValidatedInputError(ctx, transfertypes.ErrInvalidMemo)
	require.Equal(t, 1, strings.Count(output.String(), "unmapped registered Cosmos error"), "known mappings must not emit the unmapped signal")
}

func TestNewMsgTransferInvalidSourcePortUsesPrecompileNativeError(t *testing.T) {
	method := ABI.Methods[TransferMethod]
	_, _, err := NewMsgTransfer(&method, []interface{}{
		"invalid/port",
		"channel-0",
		"uatom",
		big.NewInt(1),
		common.HexToAddress("0x1"),
		"cosmos1receiver",
		clienttypes.NewHeight(0, 1),
		uint64(0),
		"",
	})
	require.Error(t, err)
	expected := cmn.NewRevertWithSolidityError(ABI, SolidityErrInvalidSourcePort, TransferMethod, ErrInvalidSourcePort)
	require.Equal(t, expected.(cmn.RevertDataCarrier).RevertData(), err.(cmn.RevertDataCarrier).RevertData())
	assertICS20NotFallback(t, err)
}

func TestNewMsgTransferInvalidSourceChannelUsesPrecompileNativeError(t *testing.T) {
	method := ABI.Methods[TransferMethod]
	_, _, err := NewMsgTransfer(&method, []interface{}{
		transfertypes.PortID,
		"invalid/channel",
		"uatom",
		big.NewInt(1),
		common.HexToAddress("0x1"),
		"cosmos1receiver",
		clienttypes.NewHeight(0, 1),
		uint64(0),
		"",
	})
	require.Error(t, err)
	expected := cmn.NewRevertWithSolidityError(ABI, SolidityErrInvalidSourceChannel, TransferMethod, ErrInvalidSourceChannel)
	require.Equal(t, expected.(cmn.RevertDataCarrier).RevertData(), err.(cmn.RevertDataCarrier).RevertData())
	assertICS20NotFallback(t, err)
}

func TestNewMsgTransferInvalidReceiverUsesPrecompileNativeError(t *testing.T) {
	method := ABI.Methods[TransferMethod]
	_, _, err := NewMsgTransfer(&method, []interface{}{
		transfertypes.PortID,
		"channel-0",
		"uatom",
		big.NewInt(1),
		common.HexToAddress("0x1"),
		"",
		clienttypes.NewHeight(0, 1),
		uint64(0),
		"",
	})
	require.Error(t, err)
	expected := cmn.NewRevertWithSolidityError(ABI, SolidityErrInvalidReceiver, TransferMethod, "invalid receiver: ")
	require.Equal(t, expected.(cmn.RevertDataCarrier).RevertData(), err.(cmn.RevertDataCarrier).RevertData())
	assertICS20NotFallback(t, err)
}

func TestNewMsgTransferMapsPublishedValidationError(t *testing.T) {
	method := ABI.Methods[TransferMethod]
	_, _, err := NewMsgTransfer(&method, []interface{}{
		transfertypes.PortID,
		"channel-0",
		"uatom",
		big.NewInt(1),
		common.HexToAddress("0x1"),
		"cosmos1receiver",
		clienttypes.NewHeight(0, 1),
		uint64(0),
		strings.Repeat("x", transfertypes.MaximumMemoLength+1),
	})
	require.Error(t, err)
	require.Equal(t, ics20ErrorSelector(SolidityErrIBCTransferInvalidMemo), err.(cmn.RevertDataCarrier).RevertData())
	assertICS20NotFallback(t, err)
}

func TestICS20UnmappedRegisteredErrorLogsOnceWithoutReason(t *testing.T) {
	var output bytes.Buffer
	ctx := sdk.Context{}.WithLogger(log.NewLogger(&output, log.OutputJSONOption()))
	p := Precompile{ABI: ABI}

	err := p.ics20MsgError(ctx, errSyntheticICS20Drift)
	require.Equal(t, ics20ErrorSelector(cmn.SolidityErrUnmappedCosmosError), err.(cmn.RevertDataCarrier).RevertData()[:4])

	logs := output.String()
	require.Equal(t, 1, strings.Count(logs, "unmapped registered Cosmos error"))
	require.Contains(t, logs, `"precompile":"ics20"`)
	require.Contains(t, logs, `"method":"transfer"`)
	require.Contains(t, logs, `"codespace":"ics20-phase-three-drift"`)
	require.Contains(t, logs, `"code":77`)
	require.NotContains(t, logs, "unstable reason")

	_ = p.ics20MsgError(ctx, transfertypes.ErrInvalidMemo)
	require.Equal(t, 1, strings.Count(output.String(), "unmapped registered Cosmos error"), "known mappings must not emit the unmapped signal")
}

func assertICS20NotFallback(t *testing.T, err error) {
	t.Helper()
	selector := err.(cmn.RevertDataCarrier).RevertData()[:4]
	require.NotEqual(t, ics20ErrorSelector(cmn.SolidityErrMsgServerFailed), selector)
	require.NotEqual(t, ics20ErrorSelector(cmn.SolidityErrQueryFailed), selector)
	require.NotEqual(t, ics20ErrorSelector(cmn.SolidityErrUnmappedCosmosError), selector)
}

func ics20ErrorSelector(name string) []byte {
	definition := ABI.Errors[name]
	return definition.ID[:4]
}
