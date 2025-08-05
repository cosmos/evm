package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/address"
)

const (
	// ModuleName defines the module name
	ModuleName = "ibc-callbacks"
)

// GenerateIsolatedAddress generates an isolated address for the given channel ID and sender address.
// This provides a safe address to call the receiver contract address with custom calldata
func GenerateIsolatedAddress(channelID string, sender string) sdk.AccAddress {
	return sdk.AccAddress(address.Module(ModuleName, []byte(channelID), []byte(sender))[:20])
}
