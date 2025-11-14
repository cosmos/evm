package bech32

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// HexToBech32 converts a hex address to its corresponding Bech32 format. The Human Readable Prefix
// (HRP) must be provided in the arguments. This function fails if the address is invalid or if the
// bech32 conversion fails.
func (p Precompile) HexToBech32(
	args HexToBech32Call,
) (*HexToBech32Return, error) {
	cfg := sdk.GetConfig()

	if strings.TrimSpace(args.Prefix) == "" {
		return nil, fmt.Errorf(
			"invalid bech32 human readable prefix (HRP). Please provide a either an account, validator or consensus address prefix (eg: %s, %s, %s)",
			cfg.GetBech32AccountAddrPrefix(), cfg.GetBech32ValidatorAddrPrefix(), cfg.GetBech32ConsensusAddrPrefix(),
		)
	}

	// NOTE: safety check, should not happen given that the address is 20 bytes.
	if err := sdk.VerifyAddressFormat(args.Addr.Bytes()); err != nil {
		return nil, err
	}

	bech32Str, err := sdk.Bech32ifyAddressBytes(args.Prefix, args.Addr.Bytes())
	if err != nil {
		return nil, err
	}

	return &HexToBech32Return{Bech32Address: bech32Str}, nil
}

// Bech32ToHex converts a bech32 address to its corresponding EIP-55 hex format. The Human Readable Prefix
// (HRP) must be provided in the arguments. This function fails if the address is invalid or if the
// bech32 conversion fails.
func (p Precompile) Bech32ToHex(
	args Bech32ToHexCall,
) (*Bech32ToHexReturn, error) {
	address := args.Bech32Address

	bech32Prefix := strings.SplitN(address, "1", 2)[0]
	if bech32Prefix == address {
		return nil, fmt.Errorf("invalid bech32 address: %s", address)
	}

	addressBz, err := sdk.GetFromBech32(address, bech32Prefix)
	if err != nil {
		return nil, err
	}

	if err := sdk.VerifyAddressFormat(addressBz); err != nil {
		return nil, err
	}

	return &Bech32ToHexReturn{Addr: common.BytesToAddress(addressBz)}, nil
}
