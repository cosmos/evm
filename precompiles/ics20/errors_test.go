package ics20

import (
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
	precompiletest "github.com/cosmos/evm/precompiles/testutil"
	callbackstypes "github.com/cosmos/evm/x/ibc/callbacks/types"
	transfertypes "github.com/cosmos/ibc-go/v11/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v11/modules/core/02-client/types"
	clienttypesv2 "github.com/cosmos/ibc-go/v11/modules/core/02-client/v2/types"
	connectiontypes "github.com/cosmos/ibc-go/v11/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v11/modules/core/04-channel/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v11/modules/core/04-channel/v2/types"
	host "github.com/cosmos/ibc-go/v11/modules/core/24-host"
	ibcerrors "github.com/cosmos/ibc-go/v11/modules/core/errors"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var errSyntheticICS20Drift = errorsmod.Register("ics20-phase-three-drift", 77, "unstable reason")

func TestTranslateICS20RegisteredErrorsDirectAndWrapped(t *testing.T) {
	p := Precompile{ABI: ABI}
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
				got := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, TransferMethod, returned, nil).Err
				require.Equal(t, ics20ErrorSelector(tc.name), got.(cmn.RevertDataCarrier).RevertData())
				assertICS20NotFallback(t, got)
			}
		})
	}
}

func TestICS20UnregisteredMsgAndQueryFailuresKeepLegacyFallbacks(t *testing.T) {
	p := Precompile{ABI: ABI}
	plain := errors.New("infrastructure")

	msgErr := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, TransferMethod, plain, nil).Err
	require.Equal(t, ics20ErrorSelector(cmn.SolidityErrMsgServerFailed), msgErr.(cmn.RevertDataCarrier).RevertData()[:4])

	queryErr := cosmosErrorRegistry.ResolveQueryError(p.ABI, DenomsMethod, plain, nil).Err
	require.Equal(t, ics20ErrorSelector(cmn.SolidityErrQueryFailed), queryErr.(cmn.RevertDataCarrier).RevertData()[:4])
}

func TestICS20ValidatedInputMapsPublishedError(t *testing.T) {
	p := Precompile{ABI: ABI}

	mapped := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, TransferMethod, transfertypes.ErrInvalidMemo, nil).Err
	require.Equal(t, ics20ErrorSelector(SolidityErrIBCTransferInvalidMemo), mapped.(cmn.RevertDataCarrier).RevertData())
	assertICS20NotFallback(t, mapped)
}

func TestICS20ValidatedInputUnmappedProtocolErrorReturnsUnmappedRevert(t *testing.T) {
	p := Precompile{ABI: ABI}
	returned := fmt.Errorf("validation wrapper with changed message: %w", errSyntheticICS20Drift)
	classification := cmn.TranslateCosmosError(ABI, cosmosErrorRegistry, returned)
	require.Equal(t, cmn.MappingKindUnmapped, classification.Kind)
	key, ok := cmn.ExtractCosmosErrorKey(returned)
	require.True(t, ok)

	err := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, TransferMethod, returned, nil).Err
	expected := classification.Revert
	require.Equal(t, expected.(cmn.RevertDataCarrier).RevertData(), err.(cmn.RevertDataCarrier).RevertData())
	decoded, unpackErr := ABI.Errors[cmn.SolidityErrUnmappedCosmosError].Inputs.Unpack(err.(cmn.RevertDataCarrier).RevertData()[4:])
	require.NoError(t, unpackErr)
	require.Equal(t, []any{key.Codespace, key.Code}, decoded)
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

func TestICS20UnmappedRegisteredErrorReturnsUnmappedRevert(t *testing.T) {
	p := Precompile{ABI: ABI}

	err := cosmosErrorRegistry.ResolveMsgServerError(p.ABI, TransferMethod, errSyntheticICS20Drift, nil).Err
	require.Equal(t, ics20ErrorSelector(cmn.SolidityErrUnmappedCosmosError), err.(cmn.RevertDataCarrier).RevertData()[:4])
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

func TestICS20BoundaryPreservationAndEquivalence(t *testing.T) {
	p := Precompile{ABI: ABI}
	t.Run("query", func(t *testing.T) {
		adapter := func(ctx sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveQueryError(p.ABI, "method", err, nil).Err
		}
		precompiletest.TestBoundaryAdapter(t, adapter)
		precompiletest.TestBoundaryEquivalence(t, ABI, cosmosErrorRegistry, "method", false, adapter, errSyntheticICS20Drift, sdkerrors.ErrUnauthorized, errors.New("internal"))
	})
	t.Run("msg", func(t *testing.T) {
		adapter := func(_ sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveMsgServerError(p.ABI, TransferMethod, err, nil).Err
		}
		precompiletest.TestBoundaryAdapter(t, adapter)
		precompiletest.TestBoundaryEquivalence(t, ABI, cosmosErrorRegistry, TransferMethod, true, adapter, errSyntheticICS20Drift, sdkerrors.ErrUnauthorized, errors.New("internal"))
	})
	t.Run("validated", func(t *testing.T) {
		adapter := func(_ sdk.Context, err error) error {
			return cosmosErrorRegistry.ResolveMsgServerError(p.ABI, TransferMethod, err, translateModuleError).Err
		}
		precompiletest.TestBoundaryAdapter(t, adapter)
		precompiletest.TestBoundaryEquivalence(t, ABI, cosmosErrorRegistry, TransferMethod, true, adapter, errSyntheticICS20Drift, sdkerrors.ErrUnauthorized, errors.New("internal"))
	})
}

func TestTranslateModuleErrorIdentifiers(t *testing.T) {
	expected := cmn.NewRevertWithSolidityError(ABI, SolidityErrInvalidSourceChannel, TransferMethod, ErrInvalidSourceChannel)
	for _, input := range []error{host.ErrInvalidID, fmt.Errorf("wrapped: %w", host.ErrInvalidID)} {
		matched, translated := translateModuleError(input)
		require.True(t, matched)
		require.Equal(t, expected.(cmn.RevertDataCarrier).RevertData(), translated.(cmn.RevertDataCarrier).RevertData())
		resolved := cosmosErrorRegistry.ResolveMsgServerError(ABI, TransferMethod, input, translateModuleError)
		require.Equal(t, expected.(cmn.RevertDataCarrier).RevertData(), resolved.Err.(cmn.RevertDataCarrier).RevertData())
		require.Equal(t, cmn.ErrorTranslation{}, resolved.Translation)
	}
	for _, input := range []error{nil, errors.New("invalid identifier"), sdkerrors.ErrUnauthorized, errSyntheticICS20Drift} {
		matched, translated := translateModuleError(input)
		require.False(t, matched)
		require.Nil(t, translated)
	}
	for _, input := range []error{
		nil,
		precompiletest.StatusRevert{},
		precompiletest.StatusOutOfGas{},
		fmt.Errorf("wrapped: %w", precompiletest.StatusRevert{}),
		fmt.Errorf("wrapped: %w", precompiletest.StatusOutOfGas{}),
		errors.Join(host.ErrInvalidID, precompiletest.StatusRevert{}),
		errors.Join(host.ErrInvalidID, precompiletest.StatusOutOfGas{}),
	} {
		resolved := cosmosErrorRegistry.ResolveMsgServerError(ABI, TransferMethod, input, translateModuleError)
		require.Equal(t, input, resolved.Err)
		require.Equal(t, cmn.ErrorTranslation{}, resolved.Translation)
	}
}

func TestTransferMessageValidationError(t *testing.T) {
	for _, tc := range []struct {
		name  string
		token sdk.Coin
		want  string
	}{
		{"zero amount", sdk.Coin{Denom: "uatom", Amount: sdkmath.ZeroInt()}, SolidityErrIBCTransferInvalidAmount},
		{"nil amount", sdk.Coin{Denom: "uatom"}, SolidityErrIBCTransferInvalidAmount},
		{"negative amount", sdk.Coin{Denom: "uatom", Amount: sdkmath.NewInt(-1)}, SolidityErrIBCTransferInvalidAmount},
		{"invalid SDK denom before amount", sdk.Coin{Denom: "a", Amount: sdkmath.ZeroInt()}, SolidityErrIBCTransferInvalidDenom},
		{"amount before IBC denom", sdk.Coin{Denom: "ibc/not-a-hash", Amount: sdkmath.ZeroInt()}, SolidityErrIBCTransferInvalidAmount},
		{"invalid IBC denom", sdk.Coin{Denom: "ibc/not-a-hash", Amount: sdkmath.OneInt()}, SolidityErrIBCTransferInvalidDenom},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cause := fmt.Errorf("wrapped SDK error: %w", ibcerrors.ErrInvalidCoins)
			validationErr := newTransferMessageValidationError(tc.token, cause)
			require.Equal(t, cause.Error(), validationErr.Error())
			require.Same(t, cause, errors.Unwrap(validationErr))
			for _, input := range []error{validationErr, fmt.Errorf("wrapped: %w", validationErr)} {
				require.ErrorIs(t, input, ibcerrors.ErrInvalidCoins)
				var contextual transferMessageValidationError
				require.ErrorAs(t, input, &contextual)
				require.Equal(t, tc.token, contextual.token)
				resolved := cosmosErrorRegistry.ResolveMsgServerError(ABI, TransferMethod, input, translateModuleError)
				definition := ABI.Errors[tc.want]
				args, err := definition.Inputs.Pack()
				require.NoError(t, err)
				require.Equal(t, append(definition.ID[:4:4], args...), resolved.Err.(cmn.RevertDataCarrier).RevertData())
				require.Equal(t, cmn.ErrorTranslation{}, resolved.Translation)
			}
		})
	}

	token := sdk.Coin{Denom: "a", Amount: sdkmath.ZeroInt()}
	t.Run("terminal errors precede token classification", func(t *testing.T) {
		for _, input := range []error{
			nil,
			precompiletest.StatusRevert{},
			precompiletest.StatusOutOfGas{},
			fmt.Errorf("wrapped: %w", precompiletest.StatusRevert{}),
			fmt.Errorf("wrapped: %w", precompiletest.StatusOutOfGas{}),
			errors.Join(ibcerrors.ErrInvalidCoins, precompiletest.StatusRevert{}),
			errors.Join(ibcerrors.ErrInvalidCoins, precompiletest.StatusOutOfGas{}),
		} {
			require.Equal(t, input, newTransferMessageValidationError(token, input))
			if input != nil {
				input = transferMessageValidationError{token: token, cause: input}
			}
			resolved := cosmosErrorRegistry.ResolveMsgServerError(ABI, TransferMethod, input, translateModuleError)
			require.Equal(t, input, resolved.Err)
			require.Equal(t, cmn.ErrorTranslation{}, resolved.Translation)
		}
	})
	t.Run("other errors retain original resolution", func(t *testing.T) {
		for _, input := range []error{
			host.ErrInvalidID,
			fmt.Errorf("wrapped: %w", host.ErrInvalidID),
			sdkerrors.ErrUnauthorized,
			transfertypes.ErrInvalidMemo,
			errSyntheticICS20Drift,
			errors.New("internal"),
		} {
			want := cosmosErrorRegistry.ResolveMsgServerError(ABI, TransferMethod, input, translateModuleError)
			require.Equal(t, input, newTransferMessageValidationError(token, input))
			contextual := transferMessageValidationError{token: token, cause: input}
			matched, translated := translateModuleError(contextual)
			wantMatched, wantTranslated := translateModuleError(input)
			require.Equal(t, wantMatched, matched)
			require.Equal(t, wantTranslated, translated)
			got := cosmosErrorRegistry.ResolveMsgServerError(ABI, TransferMethod, contextual, translateModuleError)
			require.Equal(t, want.Err.(cmn.RevertDataCarrier).RevertData(), got.Err.(cmn.RevertDataCarrier).RevertData())
			switch {
			case matched:
				require.Equal(t, cmn.ErrorTranslation{}, got.Translation)
			case got.Translation.Kind == cmn.MappingKindInternal:
				require.ErrorIs(t, got.Translation.Revert, input)
			default:
				require.Equal(t, want.Translation, got.Translation)
			}
		}
	})
	t.Run("invalid coins remain unmapped outside message validation", func(t *testing.T) {
		definition := ABI.Errors[cmn.SolidityErrUnmappedCosmosError]
		args, err := definition.Inputs.Pack("ibc", uint32(6))
		require.NoError(t, err)
		for _, resolved := range []cmn.ErrorResolution{
			cosmosErrorRegistry.ResolveQueryError(ABI, TransferMethod, ibcerrors.ErrInvalidCoins, nil),
			cosmosErrorRegistry.ResolveMsgServerError(ABI, TransferMethod, ibcerrors.ErrInvalidCoins, nil),
			cosmosErrorRegistry.ResolveMsgServerError(ABI, TransferMethod, ibcerrors.ErrInvalidCoins, translateModuleError),
		} {
			require.Equal(t, append(definition.ID[:4:4], args...), resolved.Err.(cmn.RevertDataCarrier).RevertData())
			require.True(t, resolved.Translation.IsUnmapped)
			require.Equal(t, cmn.NewCosmosErrorKey(ibcerrors.ErrInvalidCoins), resolved.Translation.Key)
		}
	})
}
