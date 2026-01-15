package bank

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// ERC20InitCode returns the initcode (bytecode + constructor args) for deploying
// the canonical ERC20 wrapper contract for a given denom.
func ERC20InitCode(denom string, bankPrecompile common.Address) ([]byte, error) {
	ctor, err := NewErc20ctorCall(denom, bankPrecompile).Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode constructor args: %w", err)
	}
	return append(ERC20Bin, ctor...), nil
}

// Create2DeployData returns the data to send to the CREATE2 factory to deploy
// the canonical ERC20 wrapper for a given denom.
// The CREATE2 factory expects: salt (32 bytes) + initcode
func Create2DeployData(denom string, bankPrecompile common.Address) ([]byte, error) {
	initcode, err := ERC20InitCode(denom, bankPrecompile)
	if err != nil {
		return nil, err
	}
	return append(ERC20Salt, initcode...), nil
}
