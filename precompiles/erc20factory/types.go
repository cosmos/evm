package erc20factory

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"
)

// EventCreate defines the event data for the ERC20 Factory Create event.
type EventCreate struct {
	TokenAddress    common.Address
	TokenPairType   uint8
	Salt            [32]uint8
	Name            string
	Symbol          string
	Decimals        uint8
	Minter          common.Address
	PremintedSupply *big.Int
}

// ParseCreateArgs parses the arguments from the create method and returns
// the token type, salt, name, symbol, decimals, minter, and preminted supply.
func ParseCreateArgs(args []interface{}) (salt [32]uint8, name string, symbol string, decimals uint8, minter common.Address, premintedSupply *big.Int, err error) {
	if len(args) != 6 {
		return [32]uint8{}, "", "", uint8(0), common.Address{}, nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 7, len(args))
	}

	salt, ok := args[0].([32]uint8)
	if !ok {
		return [32]uint8{}, "", "", uint8(0), common.Address{}, nil, fmt.Errorf("invalid salt")
	}

	name, ok = args[1].(string)
	if !ok || len(name) < 3 || len(name) > 128 {
		return [32]uint8{}, "", "", uint8(0), common.Address{}, nil, fmt.Errorf("invalid name")
	}

	symbol, ok = args[2].(string)
	if !ok || len(symbol) < 1 || len(symbol) > 16 {
		return [32]uint8{}, "", "", uint8(0), common.Address{}, nil, fmt.Errorf("invalid symbol")
	}

	decimals, ok = args[3].(uint8)
	if !ok {
		return [32]uint8{}, "", "", uint8(0), common.Address{}, nil, fmt.Errorf("invalid decimals")
	}

	minter, ok = args[4].(common.Address)
	if !ok {
		return [32]uint8{}, "", "", uint8(0), common.Address{}, nil, fmt.Errorf("invalid minter")
	}

	// Validate that minter is not the zero address
	if minter == (common.Address{}) {
		return [32]uint8{}, "", "", uint8(0), common.Address{}, nil, fmt.Errorf("invalid minter: cannot be zero address")
	}

	premintedSupply, ok = args[5].(*big.Int)
	if !ok {
		return [32]uint8{}, "", "", uint8(0), common.Address{}, nil, fmt.Errorf("invalid premintedSupply: expected *big.Int")
	}

	if premintedSupply.Sign() < 0 {
		return [32]uint8{}, "", "", uint8(0), common.Address{}, nil, fmt.Errorf("invalid premintedSupply: cannot be negative")
	}

	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	if premintedSupply.Cmp(maxUint256) > 0 {
		return [32]uint8{}, "", "", uint8(0), common.Address{}, nil, fmt.Errorf("premintedSupply exceeds uint256 maximum")
	}

	return salt, name, symbol, decimals, minter, premintedSupply, nil
}

// ParseCalculateAddressArgs parses the arguments from the calculateAddress method and returns
// the token type and salt.
func ParseCalculateAddressArgs(args []interface{}) (salt [32]uint8, err error) {
	if len(args) != 1 {
		return [32]uint8{}, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}

	salt, ok := args[0].([32]uint8)
	if !ok {
		return [32]uint8{}, fmt.Errorf("invalid salt")
	}

	return salt, nil
}
