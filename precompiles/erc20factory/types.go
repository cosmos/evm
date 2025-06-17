package erc20factory

import (
	"fmt"

	cmn "github.com/cosmos/evm/precompiles/common"
)

// ParseCreateArgs parses the arguments from the create method and returns
// the token type, salt, name, symbol and decimals.
func ParseCreateArgs(args []interface{}) (tokenType uint8, salt [32]uint8, name string, symbol string, decimals uint8, err error) {
	if len(args) != 5 {
		return uint8(0), [32]uint8{}, "", "", uint8(0), fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 5, len(args))
	}

	tokenType, ok := args[0].(uint8)
	if !ok {
		return uint8(0), [32]uint8{}, "", "", uint8(0), fmt.Errorf("invalid tokenType")
	}

	salt, ok = args[1].([32]uint8)
	if !ok {
		return uint8(0), [32]uint8{}, "", "", uint8(0), fmt.Errorf("invalid salt")
	}

	name, ok = args[2].(string)
	if !ok || len(name) < 3 || len(name) > 128 {
		return uint8(0), [32]uint8{}, "", "", uint8(0), fmt.Errorf("invalid name")
	}

	symbol, ok = args[3].(string)
	if !ok || len(symbol) < 3 || len(symbol) > 16 {
		return uint8(0), [32]uint8{}, "", "", uint8(0), fmt.Errorf("invalid symbol")
	}

	decimals, ok = args[4].(uint8)
	if !ok {
		return uint8(0), [32]uint8{}, "", "", uint8(0), fmt.Errorf("invalid decimals")
	}

	return tokenType, salt, name, symbol, decimals, nil
}

// ParseCalculateAddressArgs parses the arguments from the calculateAddress method and returns
// the token type and salt.
func ParseCalculateAddressArgs(args []interface{}) (tokenType uint8, salt [32]uint8, err error) {
	if len(args) != 2 {
		return uint8(0), [32]uint8{}, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}

	tokenType, ok := args[0].(uint8)
	if !ok {
		return uint8(0), [32]uint8{}, fmt.Errorf("invalid tokenType")
	}

	salt, ok = args[1].([32]uint8)
	if !ok {
		return uint8(0), [32]uint8{}, fmt.Errorf("invalid salt")
	}

	return tokenType, salt, nil
}
