package network

import (
	testconfig "github.com/cosmos/evm/testutil/config"
)

// chainsWEVMOSHex is an utility map used to retrieve the WEVMOS contract
// address in hex format from the chain ID.
//
// TODO: refactor to define this in the example chain initialization and pass as function argument
var chainsWEVMOSHex = map[testconfig.ChainInfo]string{
	testconfig.DefaultChainConfig.ChainInfo: testconfig.DefaultWevmosContractMainnet,
}

// GetWEVMOSContractHex returns the hex format of address for the WEVMOS contract
// given the chainID. If the chainID is not found, it defaults to the mainnet
// address.
func GetWEVMOSContractHex(chainInfo testconfig.ChainInfo) string {
	address, found := chainsWEVMOSHex[chainInfo]

	// default to mainnet address
	if !found {
		address = chainsWEVMOSHex[testconfig.DefaultChainConfig.ChainInfo]
	}

	return address
}
