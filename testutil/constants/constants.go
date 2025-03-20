package constants

import (
	"fmt"

	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

const (
	// DefaultGasPrice is used in testing as the default to use for transactions
	DefaultGasPrice = 20

	// ExampleAttoDenom provides an example denom for use in tests
	ExampleAttoDenom = "aatom"

	// ExampleMicroDenom provides an example denom for use in tests
	ExampleMicroDenom = "uatom"

	// ExampleDisplayDenom provides an example display denom for use in tests
	ExampleDisplayDenom = "atom"

	// ExampleBech32Prefix provides an example Bech32 prefix for use in tests
	ExampleBech32Prefix = "cosmos"

	// ExampleEIP155ChainID provides an example EIP-155 chain ID for use in tests
	ExampleEIP155ChainID = 9001

	// WEVMOSContractMainnet is the WEVMOS contract address for mainnet
	WEVMOSContractMainnet = "0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"
	// WEVMOSContractTestnet is the WEVMOS contract address for testnet
	WEVMOSContractTestnet = "0xcc491f589b45d4a3c679016195b3fb87d7848210"
)

var (
	// ExampleChainIDPrefix provides a chain ID prefix for EIP-155 that can be used in tests
	ExampleChainIDPrefix = fmt.Sprintf("cosmos_%d", ExampleEIP155ChainID)

	// ExampleChainID provides a chain ID that can be used in tests
	ExampleChainID = ExampleChainIDPrefix + "-1"

	// SixDecimalsChainID provides a chain ID which is being set up with 6 decimals
	SixDecimalsChainID = "ossix_9002-2"

	// ExampleChainCoinInfo provides the coin info for the example chain
	//
	// It is a map of the chain id and its corresponding EvmCoinInfo
	// that allows initializing the app with different coin info based on the
	// chain id
	ExampleChainCoinInfo = map[string]evmtypes.EvmCoinInfo{
		ExampleChainID: {
			Denom:        ExampleAttoDenom,
			DisplayDenom: ExampleDisplayDenom,
			Decimals:     evmtypes.EighteenDecimals,
		},
		SixDecimalsChainID: {
			Denom:        ExampleMicroDenom,
			DisplayDenom: ExampleDisplayDenom,
			Decimals:     evmtypes.SixDecimals,
		},
	}

	// ExampleTokenPairs creates a slice of token pairs, that contains a pair for the native denom of the example chain
	// implementation.
	ExampleTokenPairs = []erc20types.TokenPair{
		{
			Erc20Address:  WEVMOSContractMainnet,
			Denom:         ExampleAttoDenom,
			Enabled:       true,
			ContractOwner: erc20types.OWNER_MODULE,
		},
	}
)
