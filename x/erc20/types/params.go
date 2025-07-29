package types

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/types"
)

// Parameter store key
var (
	ParamStoreKeyEnableErc20                = []byte("EnableErc20")
	ParamStoreKeyDynamicPrecompiles         = []byte("DynamicPrecompiles")
	ParamStoreKeyNativePrecompiles          = []byte("NativePrecompiles")
	ParamStoreKeyPermissionlessRegistration = []byte("PermissionlessRegistration")
)

var (
	// NOTE: We strongly recommend to use the canonical address for the ERC-20 representation
	// of the chain's native denomination as defined by
	// [ERC-7528](https://eips.ethereum.org/EIPS/eip-7528).
	//
	// 0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE
	DefaultNativePrecompiles  []string
	DefaultDynamicPrecompiles []string
)

// NewParams creates a new Params object
func NewParams(
	enableErc20 bool,
	nativePrecompiles []string,
	dynamicPrecompiles []string,
	permissionlessRegistration bool,
) Params {
	slices.Sort(nativePrecompiles)
	slices.Sort(dynamicPrecompiles)
	return Params{
		EnableErc20:                enableErc20,
		NativePrecompiles:          nativePrecompiles,
		DynamicPrecompiles:         dynamicPrecompiles,
		PermissionlessRegistration: permissionlessRegistration,
	}
}

func DefaultParams() Params {
	return Params{
		EnableErc20:                true,
		NativePrecompiles:          DefaultNativePrecompiles,
		DynamicPrecompiles:         DefaultDynamicPrecompiles,
		PermissionlessRegistration: true,
	}
}

func ValidateBool(i interface{}) error {
	_, ok := i.(bool)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	return nil
}

func (p Params) Validate() error {
	if err := ValidateBool(p.EnableErc20); err != nil {
		return err
	}

	npAddrs, err := ValidatePrecompiles(p.NativePrecompiles)
	if err != nil {
		return err
	}

	dpAddrs, err := ValidatePrecompiles(p.DynamicPrecompiles)
	if err != nil {
		return err
	}

	if err := ValidateBool(p.PermissionlessRegistration); err != nil {
		return err
	}

	combined := dpAddrs
	combined = append(combined, npAddrs...)
	return validatePrecompilesUniqueness(combined)
}

// ValidatePrecompiles checks if the precompile addresses are valid and unique.
func ValidatePrecompiles(i interface{}) ([]common.Address, error) {
	precompiles, ok := i.([]string)
	if !ok {
		return nil, fmt.Errorf("invalid precompile slice type: %T", i)
	}

	precAddrs := make([]common.Address, 0, len(precompiles))
	for _, precompile := range precompiles {
		err := types.ValidateAddress(precompile)
		if err != nil {
			return nil, fmt.Errorf("invalid precompile %s", precompile)
		}
		precAddrs = append(precAddrs, common.HexToAddress(precompile))
	}

	// NOTE: Check that the precompiles are sorted. This is required
	// to ensure determinism
	if !slices.IsSorted(precompiles) {
		return nil, fmt.Errorf("precompiles need to be sorted: %s", precompiles)
	}
	return precAddrs, nil
}

func validatePrecompilesUniqueness(i interface{}) error {
	precompiles, ok := i.([]common.Address)
	if !ok {
		return fmt.Errorf("invalid precompile slice type: %T", i)
	}

	seenPrecompiles := make(map[string]struct{})
	for _, precompile := range precompiles {
		// use address.Hex() to make sure all addresses are using EIP-55
		if _, ok := seenPrecompiles[precompile.Hex()]; ok {
			return fmt.Errorf("duplicate precompile %s", precompile)
		}

		seenPrecompiles[precompile.Hex()] = struct{}{}
	}
	return nil
}

// IsNativePrecompile checks if the provided address is within the native precompiles
func (p Params) IsNativePrecompile(addr common.Address) bool {
	return isAddrIncluded(addr, p.NativePrecompiles)
}

// IsDynamicPrecompile checks if the provided address is within the dynamic precompiles
func (p Params) IsDynamicPrecompile(addr common.Address) bool {
	return isAddrIncluded(addr, p.DynamicPrecompiles)
}

// isAddrIncluded checks if the provided common.Address is within a slice
// of hex string addresses
func isAddrIncluded(addr common.Address, strAddrs []string) bool {
	for _, sa := range strAddrs {
		// check address bytes instead of the string due to possible differences
		// on the address string related to EIP-55
		cmnAddr := common.HexToAddress(sa)
		if bytes.Equal(addr.Bytes(), cmnAddr.Bytes()) {
			return true
		}
	}
	return false
}
