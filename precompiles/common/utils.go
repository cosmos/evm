package common

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"

	"github.com/cosmos/evm/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func ParseHexAddress(event sdk.Event, key string) (common.Address, error) {
	attr, ok := event.GetAttribute(key)
	if !ok {
		return common.Address{}, fmt.Errorf("event %q missing attribute %q", event.Type, key)
	}

	accAddr, err := sdk.AccAddressFromBech32(attr.Value)
	if err != nil {
		return common.Address{}, fmt.Errorf("invalid address %q: %w", attr.Value, err)
	}

	return common.BytesToAddress(accAddr), nil
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

	amountBigInt := amountCoins.AmountOf(evmtypes.GetEVMCoinDenom()).BigInt()
	amount, err := utils.Uint256FromBigInt(evmtypes.ConvertAmountTo18DecimalsBigInt(amountBigInt))
	if err != nil {
		return nil, fmt.Errorf("failed to convert coin amount to Uint256: %w", err)
	}
	return amount, nil
}
