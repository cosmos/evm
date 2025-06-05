package v2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	erc20types "github.com/cosmos/evm/x/erc20/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/types"
	ibcapi "github.com/cosmos/ibc-go/v10/modules/core/api"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ ibcapi.IBCModule = &IBCMiddleware{}

// IBCMiddleware implements the ICS26 callbacks for the transfer middleware given
// the erc20 keeper and the underlying application.
// The logics are same as the IBCMiddleware, but this is a v2 version of the middleware
type IBCMiddleware struct {
	app    ibcapi.IBCModule
	keeper erc20types.Erc20Keeper
}

// NewIBCMiddleware creates a new IBCMiddleware given the keeper and underlying application
func NewIBCMiddleware(
	app ibcapi.IBCModule,
	k erc20types.Erc20Keeper,
) IBCMiddleware {
	if app == nil {
		panic(errors.New("underlying application cannot be nil"))
	}
	if k == nil {
		panic(errors.New("erc20 keeper cannot be nil"))
	}

	return IBCMiddleware{
		app:    app,
		keeper: k,
	}
}

// OnSendPacket doesn't do anything in the erc20 middleware.
func (im IBCMiddleware) OnSendPacket(
	ctx sdk.Context,
	sourceClient string,
	destinationClient string,
	sequence uint64,
	payload channeltypesv2.Payload,
	signer sdk.AccAddress,
) error {
	return im.app.OnSendPacket(ctx, sourceClient, destinationClient, sequence, payload, signer)
}

// OnRecvPacket implements the IBCModule interface.
// It receives the tokens through the default ICS20 OnRecvPacket callback logic
// and then automatically converts the Cosmos Coin to their ERC20 token
// representation.
// If the acknowledgement fails, this callback will default to the ibc-core
// packet callback.
func (im IBCMiddleware) OnRecvPacket(
	ctx sdk.Context,
	sourceClient string,
	destinationClient string,
	sequence uint64,
	payload channeltypesv2.Payload,
	relayer sdk.AccAddress,
) channeltypesv2.RecvPacketResult {
	recvResult := im.app.OnRecvPacket(ctx, sourceClient, destinationClient, sequence, payload, relayer)
	if recvResult.Status == channeltypesv2.PacketStatus_Failure {
		return recvResult
	}
	packet, err := v2ToV1Packet(payload, sourceClient, destinationClient, sequence)
	if err != nil {
		return channeltypesv2.RecvPacketResult{
			Status: channeltypesv2.PacketStatus_Failure,
		}
	}
	var ack channeltypes.Acknowledgement
	if err := transfertypes.ModuleCdc.UnmarshalJSON(recvResult.Acknowledgement, &ack); err != nil {
		return channeltypesv2.RecvPacketResult{
			Status: channeltypesv2.PacketStatus_Failure,
		}
	}
	im.keeper.OnRecvPacket(ctx, packet, ack)
	return recvResult
}

// OnAcknowledgementPacket implements the IBCModule interface.
// It refunds the token transferred and then automatically converts the
// Cosmos Coin to their ERC20 token representation.
func (im IBCMiddleware) OnAcknowledgementPacket(
	ctx sdk.Context,
	sourceClient string,
	destinationClient string,
	sequence uint64,
	acknowledgement []byte,
	payload channeltypesv2.Payload,
	relayer sdk.AccAddress,
) error {
	if err := im.app.OnAcknowledgementPacket(ctx, sourceClient, destinationClient, sequence, acknowledgement, payload, relayer); err != nil {
		im.keeper.Logger(ctx).Error(fmt.Sprintf("erc20 middleware OnAckPacket failed to call underlying app: %s", err.Error()))
		return err
	}

	packet, err := v2ToV1Packet(payload, sourceClient, destinationClient, sequence)
	if err != nil {
		im.keeper.Logger(ctx).Error(fmt.Sprintf("erc20 middleware OnAckPacketfailed failed to convert v2 packet to v1 packet: %s", err.Error()))
		return err
	}
	var data transfertypes.FungibleTokenPacketData
	if err = transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		im.keeper.Logger(ctx).Error(fmt.Sprintf("erc20 middleware OnAckPacket failed to unmarshal packet data: %s", err.Error()))
		return err
	}
	var ack channeltypes.Acknowledgement
	if bytes.Equal(acknowledgement, channeltypesv2.ErrorAcknowledgement[:]) {
		ack = channeltypes.NewErrorAcknowledgement(transfertypes.ErrReceiveFailed)
	} else {
		if err = transfertypes.ModuleCdc.UnmarshalJSON(acknowledgement, &ack); err != nil {
			im.keeper.Logger(ctx).Error(fmt.Sprintf("erc20 middleware OnAckPacket failed to unmarshal acknowledgement: %s", err.Error()))
			return err
		}
	}
	return im.keeper.OnAcknowledgementPacket(ctx, packet, data, ack)
}

// OnTimeoutPacket implements the IBCModule interface.
// It refunds the token transferred and then automatically converts the
// Cosmos Coin to their ERC20 token representation.
func (im IBCMiddleware) OnTimeoutPacket(
	ctx sdk.Context,
	sourceClient string,
	destinationClient string,
	sequence uint64,
	payload channeltypesv2.Payload,
	relayer sdk.AccAddress,
) error {
	packet, err := v2ToV1Packet(payload, sourceClient, destinationClient, sequence)
	if err != nil {
		im.keeper.Logger(ctx).Error(fmt.Sprintf("erc20 middleware OnTimeoutPacketfailed failed to convert v2 packet to v1 packet: %s", err.Error()))
		return err
	}
	var data transfertypes.FungibleTokenPacketData
	if err = transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		im.keeper.Logger(ctx).Error(fmt.Sprintf("erc20 middleware OnTimeoutPacket failed to unmarshal packet data: %s", err.Error()))
		return err
	}

	if err = im.app.OnTimeoutPacket(ctx, sourceClient, destinationClient, sequence, payload, relayer); err != nil {
		im.keeper.Logger(ctx).Error(fmt.Sprintf("erc20 middleware OnTimeoutPacket failed to call underlying app: %s", err.Error()))
		return err
	}

	return im.keeper.OnTimeoutPacket(ctx, packet, data)
}

func v2ToV1Packet(payload channeltypesv2.Payload, sourceClient, destinationClient string, sequence uint64) (channeltypes.Packet, error) {
	transferRepresentation, err := transfertypes.UnmarshalPacketData(payload.Value, payload.Version, payload.Encoding)
	if err != nil {
		return channeltypes.Packet{}, err
	}

	packetData := transfertypes.FungibleTokenPacketData{
		Denom:    transferRepresentation.Token.Denom.Path(),
		Amount:   transferRepresentation.Token.Amount,
		Sender:   transferRepresentation.Sender,
		Receiver: transferRepresentation.Receiver,
		Memo:     transferRepresentation.Memo,
	}

	packetDataBz, err := json.Marshal(packetData)
	if err != nil {
		return channeltypes.Packet{}, err
	}

	return channeltypes.Packet{
		Sequence:           sequence,
		SourcePort:         payload.SourcePort,
		SourceChannel:      sourceClient,
		DestinationPort:    payload.DestinationPort,
		DestinationChannel: destinationClient,
		Data:               packetDataBz,
		TimeoutHeight:      clienttypes.Height{},
		TimeoutTimestamp:   0,
	}, nil
}
