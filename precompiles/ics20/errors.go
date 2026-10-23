package ics20

import (
	"errors"

	cmn "github.com/cosmos/evm/precompiles/common"
	callbackstypes "github.com/cosmos/evm/x/ibc/callbacks/types"
	transfertypes "github.com/cosmos/ibc-go/v11/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v11/modules/core/02-client/types"
	clienttypesv2 "github.com/cosmos/ibc-go/v11/modules/core/02-client/v2/types"
	connectiontypes "github.com/cosmos/ibc-go/v11/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v11/modules/core/04-channel/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v11/modules/core/04-channel/v2/types"
	host "github.com/cosmos/ibc-go/v11/modules/core/24-host"
	ibcerrors "github.com/cosmos/ibc-go/v11/modules/core/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// ErrInvalidSourcePort is raised when the source port is invalid.
	ErrInvalidSourcePort = "invalid source port"
	// ErrInvalidSourceChannel is raised when the source channel is invalid.
	ErrInvalidSourceChannel = "invalid source channel"
	// ErrInvalidSender is raised when the sender is invalid.
	ErrInvalidSender = "invalid sender: %s"
	// ErrInvalidReceiver is raised when the receiver is invalid.
	ErrInvalidReceiver = "invalid receiver: %s"
	// ErrInvalidTimeoutTimestamp is raised when the timeout timestamp is invalid.
	ErrInvalidTimeoutTimestamp = "invalid timeout timestamp: %d"
	// ErrInvalidMemo is raised when the memo is invalid.
	ErrInvalidMemo = "invalid memo: %s"
	// ErrInvalidHash is raised when the hash is invalid.
	ErrInvalidHash = "invalid hash: %s"
	// ErrNoMatchingAllocation is raised when no matching allocation is found.
	ErrNoMatchingAllocation = "no matching allocation found for source port: %s, source channel: %s, and denom: %s"
	// ErrDifferentOriginFromSender is raised when the origin address is not the same as the sender address.
	ErrDifferentOriginFromSender = "origin address %s is not the same as sender address %s"
	// ErrDenomNotFound is raised when the denom for the specified request does not exist.
	ErrDenomNotFound = "denomination not found"

	// Solidity custom error names defined in ICS20I.sol.
	SolidityErrInvalidSourcePort       = "InvalidSourcePort"
	SolidityErrInvalidSourceChannel    = "InvalidSourceChannel"
	SolidityErrInvalidReceiver         = "InvalidReceiver"
	SolidityErrInvalidTimeoutTimestamp = "InvalidTimeoutTimestamp"
	SolidityErrInvalidMemo             = "InvalidMemo"
	SolidityErrInvalidHash             = "InvalidHash"
	SolidityErrInvalidTrace            = "InvalidTrace"

	// Registered IBC and local callback custom errors defined in ICS20I.sol.
	SolidityErrIBCClientNotActive               = "IBCClientNotActive"
	SolidityErrIBCChannelNotFound               = "IBCChannelNotFound"
	SolidityErrIBCChannelInvalidState           = "IBCChannelInvalidState"
	SolidityErrIBCConnectionNotFound            = "IBCConnectionNotFound"
	SolidityErrIBCConnectionInvalidState        = "IBCConnectionInvalidState"
	SolidityErrIBCTransferInvalidDenom          = "IBCTransferInvalidDenom"
	SolidityErrIBCTransferInvalidAmount         = "IBCTransferInvalidAmount"
	SolidityErrIBCTransferDenomNotFound         = "IBCTransferDenomNotFound"
	SolidityErrIBCTransferSendDisabled          = "IBCTransferSendDisabled"
	SolidityErrIBCTransferInvalidMemo           = "IBCTransferInvalidMemo"
	SolidityErrIBCChannelSequenceSendNotFound   = "IBCChannelSequenceSendNotFound"
	SolidityErrIBCClientInvalidHeight           = "IBCClientInvalidHeight"
	SolidityErrIBCChannelTimeoutElapsed         = "IBCChannelTimeoutElapsed"
	SolidityErrIBCClientV2CounterpartyNotFound  = "IBCClientV2CounterpartyNotFound"
	SolidityErrIBCChannelV2InvalidPacket        = "IBCChannelV2InvalidPacket"
	SolidityErrIBCChannelV2SequenceSendNotFound = "IBCChannelV2SequenceSendNotFound"
	SolidityErrIBCChannelV2InvalidTimeout       = "IBCChannelV2InvalidTimeout"
	SolidityErrIBCChannelV2TimeoutElapsed       = "IBCChannelV2TimeoutElapsed"
	SolidityErrIBCUnauthorized                  = "IBCUnauthorized"
	SolidityErrIBCCallbacksNestedSourceTransfer = "IBCCallbacksNestedSourceTransfer"
)

// ErrorMappings contains the published IBC and local callback errors
// reachable from the ICS20 transfer boundary. Keys come from real sentinels.
var ics20ErrorMappings = cmn.CosmosErrorMappings{
	cmn.NewCosmosErrorMapping(clienttypes.ErrClientNotActive, SolidityErrIBCClientNotActive),
	cmn.NewCosmosErrorMapping(channeltypes.ErrChannelNotFound, SolidityErrIBCChannelNotFound),
	cmn.NewCosmosErrorMapping(channeltypes.ErrInvalidChannelState, SolidityErrIBCChannelInvalidState),
	cmn.NewCosmosErrorMapping(connectiontypes.ErrConnectionNotFound, SolidityErrIBCConnectionNotFound),
	cmn.NewCosmosErrorMapping(connectiontypes.ErrInvalidConnectionState, SolidityErrIBCConnectionInvalidState),
	cmn.NewCosmosErrorMapping(transfertypes.ErrInvalidDenomForTransfer, SolidityErrIBCTransferInvalidDenom),
	cmn.NewCosmosErrorMapping(transfertypes.ErrInvalidAmount, SolidityErrIBCTransferInvalidAmount),
	cmn.NewCosmosErrorMapping(transfertypes.ErrDenomNotFound, SolidityErrIBCTransferDenomNotFound),
	cmn.NewCosmosErrorMapping(transfertypes.ErrSendDisabled, SolidityErrIBCTransferSendDisabled),
	cmn.NewCosmosErrorMapping(transfertypes.ErrInvalidMemo, SolidityErrIBCTransferInvalidMemo),
	cmn.NewCosmosErrorMapping(channeltypes.ErrSequenceSendNotFound, SolidityErrIBCChannelSequenceSendNotFound),
	cmn.NewCosmosErrorMapping(clienttypes.ErrInvalidHeight, SolidityErrIBCClientInvalidHeight),
	cmn.NewCosmosErrorMapping(channeltypes.ErrTimeoutElapsed, SolidityErrIBCChannelTimeoutElapsed),
	cmn.NewCosmosErrorMapping(clienttypesv2.ErrCounterpartyNotFound, SolidityErrIBCClientV2CounterpartyNotFound),
	cmn.NewCosmosErrorMapping(channeltypesv2.ErrInvalidPacket, SolidityErrIBCChannelV2InvalidPacket),
	cmn.NewCosmosErrorMapping(channeltypesv2.ErrSequenceSendNotFound, SolidityErrIBCChannelV2SequenceSendNotFound),
	cmn.NewCosmosErrorMapping(channeltypesv2.ErrInvalidTimeout, SolidityErrIBCChannelV2InvalidTimeout),
	cmn.NewCosmosErrorMapping(channeltypesv2.ErrTimeoutElapsed, SolidityErrIBCChannelV2TimeoutElapsed),
	cmn.NewCosmosErrorMapping(ibcerrors.ErrUnauthorized, SolidityErrIBCUnauthorized),
	cmn.NewCosmosErrorMapping(callbackstypes.ErrNestedSourceCallbackTransfer, SolidityErrIBCCallbacksNestedSourceTransfer),
}

func ErrorMappings() cmn.CosmosErrorMappings {
	return ics20ErrorMappings.Clone()
}

// transferMessageValidationError retains the token when MsgTransfer.ValidateBasic
// collapses its coin validation failure into ErrInvalidCoins.
type transferMessageValidationError struct {
	token sdk.Coin
	cause error
}

// newTransferMessageValidationError attaches token context only to nonterminal
// invalid-coin failures, returning all other errors unchanged.
func newTransferMessageValidationError(token sdk.Coin, cause error) error {
	if !cmn.NeedsErrorTranslation(cause) || !errors.Is(cause, ibcerrors.ErrInvalidCoins) {
		return cause
	}
	return transferMessageValidationError{token: token, cause: cause}
}

func (e transferMessageValidationError) Error() string { return e.cause.Error() }

func (e transferMessageValidationError) Unwrap() error { return e.cause }

// translateModuleError applies ICS20-specific translation at input-validation boundaries.
func translateModuleError(err error) (bool, error) {
	key, ok := cmn.ExtractCosmosErrorKey(err)
	if !ok {
		return false, nil
	}
	if key == cmn.NewCosmosErrorKey(host.ErrInvalidID) {
		return true, cmn.NewRevertWithSolidityError(ABI, SolidityErrInvalidSourceChannel, TransferMethod, ErrInvalidSourceChannel)
	}
	var validationErr transferMessageValidationError
	if key != cmn.NewCosmosErrorKey(ibcerrors.ErrInvalidCoins) || !errors.As(err, &validationErr) {
		return false, nil
	}
	// Keep IBC's validation order: SDK denom syntax, amount, then IBC denom.
	if sdk.ValidateDenom(validationErr.token.Denom) != nil {
		return true, cmn.NewRevertWithSolidityError(ABI, SolidityErrIBCTransferInvalidDenom)
	}
	if validationErr.token.Amount.IsNil() || !validationErr.token.Amount.IsPositive() {
		return true, cmn.NewRevertWithSolidityError(ABI, SolidityErrIBCTransferInvalidAmount)
	}
	// ErrInvalidCoins already proves validation failed. With valid SDK syntax
	// and a positive amount, only IBC denom validation remains in IBC v11.
	return true, cmn.NewRevertWithSolidityError(ABI, SolidityErrIBCTransferInvalidDenom)
}
