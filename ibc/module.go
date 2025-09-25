package ibc

import (
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	"github.com/cosmos/ibc-go/v10/modules/core/exported"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ porttypes.IBCModule = &Module{}

// Module a concrete type for a module boilerplate.
type Module struct {
	app porttypes.IBCModule
}

// NewModule creates a new IBC Module boilerplate given the underlying IBC app
func NewModule(app porttypes.IBCModule) *Module {
	return &Module{
		app: app,
	}
}

// OnChanOpenInit implements the Module interface
// It calls the underlying app's OnChanOpenInit callback.
func (im Module) OnChanOpenInit(
	ctx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID string,
	channelID string,
	counterparty channeltypes.Counterparty,
	version string,
) (string, error) {
	return im.app.OnChanOpenInit(ctx, order, connectionHops, portID, channelID, counterparty, version)
}

// OnChanOpenTry implements the Module interface.
// It calls the underlying app's OnChanOpenTry callback.
func (im Module) OnChanOpenTry(
	ctx sdk.Context,
	order channeltypes.Order,
	connectionHops []string,
	portID,
	channelID string,
	counterparty channeltypes.Counterparty,
	counterpartyVersion string,
) (version string, err error) {
	return im.app.OnChanOpenTry(ctx, order, connectionHops, portID, channelID, counterparty, counterpartyVersion)
}

// OnChanOpenAck implements the Module interface.
// It calls the underlying app's OnChanOpenAck callback.
func (im Module) OnChanOpenAck(
	ctx sdk.Context,
	portID,
	channelID,
	counterpartyChannelID,
	counterpartyVersion string,
) error {
	return im.app.OnChanOpenAck(ctx, portID, channelID, counterpartyChannelID, counterpartyVersion)
}

// OnChanOpenConfirm implements the Module interface.
// It calls the underlying app's OnChanOpenConfirm callback.
func (im Module) OnChanOpenConfirm(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	return im.app.OnChanOpenConfirm(ctx, portID, channelID)
}

// OnChanCloseInit implements the Module interface
// It calls the underlying app's OnChanCloseInit callback.
func (im Module) OnChanCloseInit(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	return im.app.OnChanCloseInit(ctx, portID, channelID)
}

// OnChanCloseConfirm implements the Module interface.
// It calls the underlying app's OnChanCloseConfirm callback.
func (im Module) OnChanCloseConfirm(
	ctx sdk.Context,
	portID,
	channelID string,
) error {
	return im.app.OnChanCloseConfirm(ctx, portID, channelID)
}

// OnRecvPacket implements the Module interface.
// It calls the underlying app's OnRecvPacket callback.
func (im Module) OnRecvPacket(
	ctx sdk.Context,
	channelVersion string,
	packet channeltypes.Packet,
	relayer sdk.AccAddress,
) exported.Acknowledgement {
	return im.app.OnRecvPacket(ctx, channelVersion, packet, relayer)
}

// OnAcknowledgementPacket implements the Module interface.
// It calls the underlying app's OnAcknowledgementPacket callback.
func (im Module) OnAcknowledgementPacket(
	ctx sdk.Context,
	channelVersion string,
	packet channeltypes.Packet,
	acknowledgement []byte,
	relayer sdk.AccAddress,
) error {
	return im.app.OnAcknowledgementPacket(ctx, channelVersion, packet, acknowledgement, relayer)
}

// OnTimeoutPacket implements the Module interface.
// It calls the underlying app's OnTimeoutPacket callback.
func (im Module) OnTimeoutPacket(
	ctx sdk.Context,
	channelVersion string,
	packet channeltypes.Packet,
	relayer sdk.AccAddress,
) error {
	return im.app.OnTimeoutPacket(ctx, channelVersion, packet, relayer)
}

func (im Module) UnmarshalPacketData(ctx sdk.Context, portID string, channelID string, bz []byte) (any, string, error) {
	pd, ok := im.app.(porttypes.PacketDataUnmarshaler)
	if !ok {
		panic("app does not implement porttypes.PacketDataUnmarshaler")
	}
	return pd.UnmarshalPacketData(ctx, portID, channelID, bz)
}

func (im Module) SetICS4Wrapper(wrapper porttypes.ICS4Wrapper) {
	im.app.SetICS4Wrapper(wrapper)
}
