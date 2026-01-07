package utils

import (
	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func ValidatorConsAddressToHex(valAddress string) common.Address {
	valAddr, err := sdk.ValAddressFromBech32(valAddress)
	if err != nil {
		return common.Address{}
	}
	return common.BytesToAddress(valAddr)
}
