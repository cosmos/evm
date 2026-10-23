package cli

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"

	"github.com/cosmos/evm/utils"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func accountToHex(addr string) (string, error) {
	if strings.HasPrefix(addr, sdk.GetConfig().GetBech32AccountAddrPrefix()) {
		// Check to see if address is Cosmos bech32 formatted
		toAddr, err := sdk.AccAddressFromBech32(addr)
		if err != nil {
			return "", errors.Wrap(err, "must provide a valid Bech32 address")
		}
		ethAddr := common.BytesToAddress(toAddr.Bytes())
		return ethAddr.Hex(), nil
	}

	if !strings.HasPrefix(addr, "0x") {
		addr = "0x" + addr
	}

	valid := common.IsHexAddress(addr)
	if !valid {
		return "", fmt.Errorf("%s is not a valid Ethereum or Cosmos address", addr)
	}

	ethAddr := common.HexToAddress(addr)

	return ethAddr.Hex(), nil
}

// hexAddress parses a hex address. Unlike common.HexToAddress it rejects input
// that is not a valid hex address instead of decoding it to a zero-padded or
// cropped address.
func hexAddress(addr string) (common.Address, error) {
	if !common.IsHexAddress(addr) {
		return common.Address{}, fmt.Errorf("%q is not a valid hex address", addr)
	}

	return common.HexToAddress(addr), nil
}

// hexToBech32 converts a hex address to the bech32 account address of the same
// bytes, rejecting input that is not a valid hex address.
func hexToBech32(addr string) (string, error) {
	if _, err := hexAddress(addr); err != nil {
		return "", err
	}

	return utils.Bech32StringFromHexAddress(addr), nil
}

// accountToBech32 returns the bech32 account address for a hex or bech32
// address.
func accountToBech32(addr string) (string, error) {
	// 0x-prefixed input is never bech32, so report it as a hex address.
	if common.IsHexAddress(addr) || strings.HasPrefix(strings.ToLower(addr), "0x") {
		return hexToBech32(addr)
	}

	accAddr, err := sdk.AccAddressFromBech32(addr)
	if err != nil {
		return "", errors.Wrapf(err, "%q is not a valid hex or bech32 address", addr)
	}

	return accAddr.String(), nil
}

func formatKeyToHash(key string) string {
	if !strings.HasPrefix(key, "0x") {
		key = "0x" + key
	}

	ethkey := common.HexToHash(key)

	return ethkey.Hex()
}
