package common

import (
	"fmt"
	"math/big"

	"github.com/holiman/uint256"

	"github.com/cosmos/evm/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// ParseAddress parses the address from the event attributes
func ParseAddress(event sdk.Event, key string) (sdk.AccAddress, error) {
	attr, ok := event.GetAttribute(key)
	if !ok {
		return sdk.AccAddress{}, fmt.Errorf("event %q missing attribute %q", event.Type, key)
	}

	accAddr, err := sdk.AccAddressFromBech32(attr.Value)
	if err != nil {
		return sdk.AccAddress{}, fmt.Errorf("invalid address %q: %w", attr.Value, err)
	}

	return accAddr, nil
}

func ParseAmount(event sdk.Event) (*uint256.Int, error) {
	amountAttr, ok := event.GetAttribute(sdk.AttributeKeyAmount)
	if !ok {
		return nil, fmt.Errorf("event %q missing attribute %q", banktypes.EventTypeCoinSpent, sdk.AttributeKeyAmount)
	}

	amountCoins, err := sdk.ParseCoinsNormalized(amountAttr.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to parse coins from %q: %w", amountAttr.Value, err)
	}

	baseAmount := amountCoins.AmountOf(evmtypes.GetEVMCoinDenom()).BigInt()
	extendedAmount := amountCoins.AmountOf(evmtypes.GetEVMCoinExtendedDenom()).BigInt()

	var amountBigInt *big.Int
	if baseAmount.Sign() > 0 {
		amountBigInt = evmtypes.ConvertAmountTo18DecimalsBigInt(baseAmount)
	} else {
		// The extended denom is already represented in 18 decimals.
		amountBigInt = extendedAmount
	}

	amount, err := utils.Uint256FromBigInt(amountBigInt)
	if err != nil {
		return nil, fmt.Errorf("failed to convert coin amount to Uint256: %w", err)
	}
	return amount, nil
}
