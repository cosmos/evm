package erc20

import (
	"errors"

	"github.com/cosmos/evm/ibc"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	"github.com/cosmos/ibc-go/v10/modules/core/exported"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	_ porttypes.IBCModule             = &IBCMiddleware{}
	_ porttypes.PacketDataUnmarshaler = &IBCMiddleware{}
)

// IBCMiddleware implements the ICS26 callbacks for the transfer middleware given
// the erc20 keeper and the underlying application.
type IBCMiddleware struct {
	*ibc.Module
	keeper erc20types.Erc20Keeper
}

// NewIBCMiddleware creates a new IBCMiddleware given the keeper and underlying application
func NewIBCMiddleware(k erc20types.Erc20Keeper, app porttypes.IBCModule) IBCMiddleware {
	if app == nil {
		panic(errors.New("underlying application cannot be nil"))
	}
	if k == nil {
		panic(errors.New("erc20 keeper cannot be nil"))
	}

	return IBCMiddleware{
		Module: ibc.NewModule(app),
		keeper: k,
	}
}

// OnRecvPacket implements the IBCModule interface.
// It receives the tokens through the default ICS20 OnRecvPacket callback logic
// and then automatically converts the Cosmos Coin to their ERC20 token
// representation.
// If the acknowledgement fails, this callback will default to the ibc-core
// packet callback.
func (im IBCMiddleware) OnRecvPacket(
	ctx sdk.Context,
	channelVersion string,
	packet channeltypes.Packet,
	relayer sdk.AccAddress,
) exported.Acknowledgement {
	ack := im.Module.OnRecvPacket(ctx, channelVersion, packet, relayer)

	// return if the acknowledgement is an error ACK
	if !ack.Success() {
		return ack
	}

	return im.keeper.OnRecvPacket(ctx, packet, ack)
}

// OnAcknowledgementPacket implements the IBCModule interface.
// It refunds the token transferred and then automatically converts the
// Cosmos Coin to their ERC20 token representation.
func (im IBCMiddleware) OnAcknowledgementPacket(
	ctx sdk.Context,
	channelVersion string,
	packet channeltypes.Packet,
	acknowledgement []byte,
	relayer sdk.AccAddress,
) error {
	var ack channeltypes.Acknowledgement
	if err := transfertypes.ModuleCdc.UnmarshalJSON(acknowledgement, &ack); err != nil {
		return errorsmod.Wrapf(errortypes.ErrUnknownRequest, "cannot unmarshal ICS-20 transfer packet acknowledgement: %v", err)
	}

	var data transfertypes.FungibleTokenPacketData
	if err := transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		return errorsmod.Wrapf(errortypes.ErrUnknownRequest, "cannot unmarshal ICS-20 transfer packet data: %s", err.Error())
	}

	if err := im.Module.OnAcknowledgementPacket(ctx, channelVersion, packet, acknowledgement, relayer); err != nil {
		return err
	}

	return im.keeper.OnAcknowledgementPacket(ctx, packet, data, ack)
}

// OnTimeoutPacket implements the IBCModule interface.
// It refunds the token transferred and then automatically converts the
// Cosmos Coin to their ERC20 token representation.
func (im IBCMiddleware) OnTimeoutPacket(
	ctx sdk.Context,
	channelVersion string,
	packet channeltypes.Packet,
	relayer sdk.AccAddress,
) error {
	var data transfertypes.FungibleTokenPacketData
	if err := transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		return errorsmod.Wrapf(errortypes.ErrUnknownRequest, "cannot unmarshal ICS-20 transfer packet data: %s", err.Error())
	}

	if err := im.Module.OnTimeoutPacket(ctx, channelVersion, packet, relayer); err != nil {
		return err
	}

	return im.keeper.OnTimeoutPacket(ctx, packet, data)
}

// UnmarshalPacketa implements the PacketDataUnmarshaler interface.
func (im IBCMiddleware) UnmarshalPacketData(
	ctx sdk.Context,
	portID, channelID string,
	data []byte,
) (any, string, error) {
	return im.Module.UnmarshalPacketData(ctx, portID, channelID, data)
}
