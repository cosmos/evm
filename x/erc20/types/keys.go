package types

import (
	"github.com/ethereum/go-ethereum/common"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// constants
const (
	// module name
	ModuleName = "erc20"

	// StoreKey to be used when creating the KVStore
	StoreKey = ModuleName

	// RouterKey to be used for message routing
	RouterKey = ModuleName
)

// ModuleAddress is the native module address for ERC-20
var ModuleAddress common.Address

func init() {
	ModuleAddress = common.BytesToAddress(authtypes.NewModuleAddress(ModuleName).Bytes())
}

// prefix bytes for the ERC-20 persistent store
const (
	prefixTokenPair = iota + 1
	prefixTokenPairByERC20
	prefixTokenPairByDenom
	prefixSTRv2Addresses
	prefixAllowance
)

// KVStore key prefixes
var (
	KeyPrefixTokenPair        = []byte{prefixTokenPair}
	KeyPrefixTokenPairByERC20 = []byte{prefixTokenPairByERC20}
	KeyPrefixTokenPairByDenom = []byte{prefixTokenPairByDenom}
	KeyPrefixSTRv2Addresses   = []byte{prefixSTRv2Addresses}
	KeyPrefixAllowance        = []byte{prefixAllowance}
)

func AllowanceKey(
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
) []byte {
	return append(append(erc20.Bytes(), owner.Bytes()...), spender.Bytes()...)
}
