package types

import "github.com/ethereum/go-ethereum/common"

// GetAddress casts the hex string address of the ClientPrecompile to common.Address
func (p ClientPrecompile) GetContractAddress() common.Address {
	return common.HexToAddress(p.Address)
}
