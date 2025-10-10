package ibc

import (
	"strings"

	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

// GetTransferAmount returns the amount from an ICS20 FungibleTokenPacketData as a string.
func GetTransferAmount(packet channeltypes.Packet) (string, error) {
	// unmarshal packet data to obtain the sender and recipient
	var data transfertypes.FungibleTokenPacketData
	if err := transfertypes.ModuleCdc.UnmarshalJSON(packet.GetData(), &data); err != nil {
		return "", errorsmod.Wrapf(errortypes.ErrUnknownRequest, "cannot unmarshal ICS-20 transfer packet data")
	}

	if data.Amount == "" {
		return "", errorsmod.Wrapf(errortypes.ErrInvalidCoins, "empty amount")
	}

	if _, ok := math.NewIntFromString(data.Amount); !ok {
		return "", errorsmod.Wrapf(errortypes.ErrInvalidCoins, "invalid amount")
	}

	return data.Amount, nil
}

// GetReceivedCoin returns the transferred coin from an ICS20 FungibleTokenPacketData
// as seen from the destination chain.
// If the receiving chain is the source chain of the tokens, it removes the prefix
// path added by source (i.e sender) chain to the denom. Otherwise, it adds the
// prefix path from the destination chain to the denom.
func GetReceivedCoin(packet channeltypes.Packet, token transfertypes.Token) sdk.Coin {
	// NOTE: Denom and amount are already validated
	amountInt, _ := math.NewIntFromString(token.Amount)

	if token.Denom.HasPrefix(packet.SourcePort, packet.SourceChannel) {
		token.Denom.Trace = token.Denom.Trace[1:]

		// coin denomination used in sending from the escrow address
		// The denomination used to send the coins is either the native denom or the hash of the path
		// if the denomination is not native.
		return sdk.Coin{
			Denom:  token.Denom.IBCDenom(),
			Amount: amountInt,
		}
	}

	// since SendPacket did not prefix the denomination, we must prefix denomination here
	hop := []transfertypes.Hop{transfertypes.NewHop(packet.DestinationPort, packet.DestinationChannel)}
	token.Denom.Trace = append(hop, token.Denom.Trace...)

	return sdk.Coin{
		Denom:  token.Denom.IBCDenom(),
		Amount: amountInt,
	}
}

// GetSentCoin returns the sent coin from an ICS20 FungibleTokenPacketData.
func GetSentCoin(rawDenom, rawAmt string) sdk.Coin {
	// NOTE: Denom and amount are already validated
	amount, _ := math.NewIntFromString(rawAmt)
	denom := transfertypes.ExtractDenomFromPath(rawDenom)

	return sdk.Coin{
		Denom:  denom.IBCDenom(),
		Amount: amount,
	}
}

// GetDenom returns the denomination from the corresponding IBC denomination. If the
// denomination is not an IBC voucher or the trace is not found, it returns an error.
func GetDenom(
	transferKeeper TransferKeeper,
	ctx sdk.Context,
	voucherDenom string,
) (transfertypes.Denom, error) {
	if !strings.HasPrefix(voucherDenom, "ibc/") {
		return transfertypes.Denom{}, errorsmod.Wrapf(ErrNoIBCVoucherDenom, "denom: %s", voucherDenom)
	}

	hash, err := transfertypes.ParseHexHash(voucherDenom[4:])
	if err != nil {
		return transfertypes.Denom{}, err
	}

	denom, found := transferKeeper.GetDenom(ctx, hash)
	if !found {
		return transfertypes.Denom{}, ErrDenomNotFound
	}

	return denom, nil
}

// DeriveDecimalsFromDenom returns the number of decimals of an IBC coin
// depending on the prefix of the base denomination
func DeriveDecimalsFromDenom(baseDenom string) (uint8, error) {
	var decimals uint8
	if len(baseDenom) == 0 {
		return decimals, errorsmod.Wrapf(ErrInvalidBaseDenom, "Base denom cannot be an empty string")
	}

	switch baseDenom[0] {
	case 'u': // micro (u) -> 6 decimals
		decimals = 6
	case 'a': // atto (a) -> 18 decimals
		decimals = 18
	default:
		return decimals, errorsmod.Wrapf(
			ErrInvalidBaseDenom,
			"Should be either micro ('u[...]') or atto ('a[...]'); got: %q",
			baseDenom,
		)
	}
	return decimals, nil
}
