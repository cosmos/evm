package testutil

import (
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
)

func GetVoucherDenomFromPacketData(
	data transfertypes.InternalTransferRepresentation,
	destPort string,
	destChannel string,
) string {
	token := data.Token
	trace := []transfertypes.Hop{transfertypes.NewHop(destPort, destChannel)}
	token.Denom.Trace = append(trace, token.Denom.Trace...)
	voucherDenom := token.Denom.IBCDenom()
	return voucherDenom
}
