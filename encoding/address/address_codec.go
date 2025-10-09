package address

import (
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/utils"

	"cosmossdk.io/core/address"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ address.Codec = (*evmCodec)(nil)

// evmCodec defines an address codec for EVM compatible cosmos modules
type evmCodec struct {
	bech32Prefix string
}

// NewEvmCodec returns a new EvmCodec with the given bech32 prefix
func NewEvmCodec(prefix string) address.Codec {
	return evmCodec{prefix}
}

// StringToBytes decodes text to bytes using either hex or bech32 encoding
func (bc evmCodec) StringToBytes(text string) ([]byte, error) {
	if len(strings.TrimSpace(text)) == 0 {
		return []byte{}, sdkerrors.ErrInvalidAddress.Wrap("empty address string is not allowed")
	}

	switch {
	case common.IsHexAddress(text):
		return common.HexToAddress(text).Bytes(), nil
	case utils.IsBech32Address(text):
		hrp, bz, err := bech32.DecodeAndConvert(text)
		if err != nil {
			return nil, err
		}
		if hrp != bc.bech32Prefix {
			return nil, sdkerrors.ErrLogic.Wrapf("hrp does not match bech32 prefix: expected '%s' got '%s'", bc.bech32Prefix, hrp)
		}
		if err := sdk.VerifyAddressFormat(bz); err != nil {
			return nil, err
		}
		return bz, nil
	default:
		return nil, sdkerrors.ErrUnknownAddress.Wrapf("unknown address format: %s", text)
	}
}

// BytesToString decodes bytes to text
func (bc evmCodec) BytesToString(bz []byte) (string, error) {
	text, err := bech32.ConvertAndEncode(bc.bech32Prefix, bz)
	if err != nil {
		return "", err
	}

	return text, nil
}
