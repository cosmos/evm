package evmd

import (
	appcfg "github.com/cosmos/evm/evmd/config"
	erc20types "github.com/cosmos/evm/x/erc20/types"
)

// WEVMOSContractMainnet is the WEVMOS contract address for mainnet
const WEVMOSContractMainnet = "0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"

// ExampleTokenPairs creates a slice of token pairs, that contains a pair for the native denom of the example chain
// implementation.
var ExampleTokenPairs = []erc20types.TokenPair{
	{
		Erc20Address:  WEVMOSContractMainnet,
		Denom:         appcfg.ExampleChainDenom,
		Enabled:       true,
		ContractOwner: erc20types.OWNER_MODULE,
	},
}
