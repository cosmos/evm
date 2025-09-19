package utils

import (
	fmt "fmt"
	math "math"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

const maxBitLen = 256

// SafeInt64 checks for overflows while casting a uint64 to int64 value.
func SafeInt64(value uint64) (int64, error) {
	if value > uint64(math.MaxInt64) {
		return 0, errorsmod.Wrapf(errortypes.ErrInvalidHeight, "uint64 value %v cannot exceed %v", value, int64(math.MaxInt64))
	}

	return int64(value), nil // #nosec G115 -- checked for int overflow already
}

// SafeUint64 checks for underflows while casting an int64 to uint64 value.
func SafeUint64(value int64) (uint64, error) {
	if value < 0 {
		return 0, fmt.Errorf("invalid value: %d", value)
	}
	return uint64(value), nil
}

// SafeNewIntFromBigInt constructs Int from big.Int, return error if more than 256bits
func SafeNewIntFromBigInt(i *big.Int) (sdkmath.Int, error) {
	if !IsValidInt256(i) {
		return sdkmath.NewInt(0), fmt.Errorf("big int out of bound: %s", i)
	}
	return sdkmath.NewIntFromBigInt(i), nil
}

// IsValidInt256 check the bound of 256 bit number
func IsValidInt256(i *big.Int) bool {
	return i == nil || i.BitLen() <= maxBitLen
}

// SafeHexToInt64 converts a hexutil.Uint64 to int64, returning an error if it exceeds the max int64 value.
func SafeHexToInt64(value hexutil.Uint64) (int64, error) {
	if value > math.MaxInt64 {
		return 0, fmt.Errorf("hexutil.Uint64 value %v cannot exceed %v", value, math.MaxInt64)
	}

	return int64(value), nil //nolint:gosec // checked
}
