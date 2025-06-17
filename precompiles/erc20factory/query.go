package erc20factory

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	// CalculateAddressMethod defines the ABI method name for the CalculateAddress
	// query.
	CalculateAddressMethod = "calculateAddress"
)

// CalculateAddress calculates the address of a new ERC20 Token Pair
func (p Precompile) CalculateAddress(
	method *abi.Method,
	caller common.Address,
	args []interface{},
) ([]byte, error) {
	tokenType, salt, err := ParseCalculateAddressArgs(args)
	if err != nil {
		return nil, err
	}

	address := crypto.CreateAddress2(caller, salt, calculateCodeHash(tokenType))

	return method.Outputs.Pack(address)
}
