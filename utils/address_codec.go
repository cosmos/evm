package utils

import (
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"cosmossdk.io/core/address"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

// EvmCodec defines an address codec for EVM compatible cosmos modules
type EvmCodec struct {
	Bech32Prefix string
}

var _ address.Codec = (*EvmCodec)(nil)

// NewEvmCodec returns a new EvmCodec with the given bech32 prefix
func NewEvmCodec(prefix string) address.Codec {
	return EvmCodec{prefix}
}

// StringToBytes decodes text to bytes using either hex or bech32 encoding
func (bc EvmCodec) StringToBytes(text string) ([]byte, error) {
	if len(strings.TrimSpace(text)) == 0 {
		return []byte{}, sdkerrors.ErrInvalidAddress.Wrap("empty address string is not allowed")
	}

	switch {
	case common.IsHexAddress(text):
		return common.HexToAddress(text).Bytes(), nil
	case IsBech32Address(text):
		hrp, bz, err := bech32.DecodeAndConvert(text)
		if err != nil {
			return nil, err
		}
		if hrp != bc.Bech32Prefix {
			return nil, sdkerrors.ErrLogic.Wrapf("hrp does not match bech32 prefix: expected '%s' got '%s'", bc.Bech32Prefix, hrp)
		}
		if err := sdk.VerifyAddressFormat(bz); err != nil {
			return nil, err
		}
		return bz, nil
	default:
		return nil, sdkerrors.ErrUnknownAddress.Wrapf("unknown address format: %s", text)
	}
}

// BytesToString encodes bytes to EIP55-compliant hex string representation of the address
func (bc EvmCodec) BytesToString(bz []byte) (string, error) {
	return common.BytesToAddress(bz).Hex(), nil
}
