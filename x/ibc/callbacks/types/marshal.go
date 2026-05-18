package types

import (
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ porttypes.PacketDataUnmarshaler = (*Unmarshaler)(nil)

type Unmarshaler struct{}

// UnmarshalPacketData will unmarshal the packet data for the IBC transfer callback.
// It expects the data to be in the format of transfertypes.FungibleTokenPacketData.
// If the data is not in the expected format, it returns an error.
func (u Unmarshaler) UnmarshalPacketData(ctx sdk.Context, portID, channelID string, data []byte) (any, string, error) {
	transferData, err := transfertypes.UnmarshalPacketData(
		data, transfertypes.V1, transfertypes.EncodingJSON,
	)
	if err != nil {
		return nil, "", err
	}
	return transferData, transfertypes.V1, nil
}
