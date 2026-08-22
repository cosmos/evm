package bech32

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// HexToBech32Method defines the ABI method name to convert a EIP-55
	// hex formatted address to bech32 address string.
	HexToBech32Method = "hexToBech32"
	// Bech32ToHexMethod defines the ABI method name to convert a bech32
	// formatted address string to an EIP-55 address.
	Bech32ToHexMethod = "bech32ToHex"
)

// HexToBech32 converts a hex address to its corresponding Bech32 format. The Human Readable Prefix
// (HRP) must be provided in the arguments. This function fails if the address is invalid or if the
// bech32 conversion fails.
func (Precompile) HexToBech32(
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 2 {
		return nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(2), big.NewInt(int64(len(args))))
	}

	address, ok := args[0].(common.Address)
	if !ok {
		return nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, fmt.Sprintf("%v", args[0]))
	}

	cfg := sdk.GetConfig()

	prefix, _ := args[1].(string)
	if strings.TrimSpace(prefix) == "" {
		return nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, fmt.Sprintf(
			"invalid HRP: empty; expected account (%s), validator (%s), or consensus (%s) style prefix",
			cfg.GetBech32AccountAddrPrefix(), cfg.GetBech32ValidatorAddrPrefix(), cfg.GetBech32ConsensusAddrPrefix(),
		))
	}

	// NOTE: safety check, should not happen given that the address is 20 bytes.
	if err := sdk.VerifyAddressFormat(address.Bytes()); err != nil {
		return nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, err.Error())
	}

	bech32Str, err := sdk.Bech32ifyAddressBytes(prefix, address.Bytes())
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrQueryFailed, HexToBech32Method, err.Error())
	}

	return method.Outputs.Pack(bech32Str)
}

// Bech32ToHex converts a bech32 address to its corresponding EIP-55 hex format. The Human Readable Prefix
// (HRP) must be provided in the arguments. This function fails if the address is invalid or if the
// bech32 conversion fails.
func (Precompile) Bech32ToHex(
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	if len(args) != 1 {
		return nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(1), big.NewInt(int64(len(args))))
	}

	address, ok := args[0].(string)
	if !ok || address == "" {
		return nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, fmt.Sprintf("%v", args[0]))
	}

	bech32Prefix := strings.SplitN(address, "1", 2)[0]
	if bech32Prefix == address {
		return nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, address)
	}

	addressBz, err := sdk.GetFromBech32(address, bech32Prefix)
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrQueryFailed, Bech32ToHexMethod, err.Error())
	}

	if err := sdk.VerifyAddressFormat(addressBz); err != nil {
		return nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, err.Error())
	}

	return method.Outputs.Pack(common.BytesToAddress(addressBz))
}
