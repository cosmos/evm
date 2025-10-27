package constants

import (
	"github.com/cosmos/evm/server/config"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"
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

	// EighteenDecimalsChainID provides an example EIP-155 chain ID for use in tests
	EighteenDecimalsChainID = 9001

	// WEVMOSContractMainnet is the WEVMOS contract address for mainnet
	WEVMOSContractMainnet = "0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"
	// WEVMOSContractTestnet is the WEVMOS contract address for testnet
	WEVMOSContractTestnet = "0xcc491f589b45d4a3c679016195b3fb87d7848210"
	// ExampleEvmAddress1 is the example EVM address
	ExampleEvmAddressAlice = "0x1e0DE5DB1a39F99cBc67B00fA3415181b3509e42"
	// ExampleEvmAddress2 is the example EVM address
	ExampleEvmAddressBob = "0x0AFc8e15F0A74E98d0AEC6C67389D2231384D4B2"
)

// ChainsCoinInfo is a map of the chain id and its corresponding EvmCoinInfo
// that allows initializing the app with different coin info based on the
// chain id
var ChainsCoinInfo = map[uint64]evmtypes.EvmCoinInfo{ // TODO:VLAD - deduplicate this for tests
	EighteenDecimalsChainID: {
		Denom:         ExampleAttoDenom,
		ExtendedDenom: ExampleAttoDenom,
		DisplayDenom:  ExampleDisplayDenom,
		Decimals:      evmtypes.EighteenDecimals.Uint32(),
	},
	// SixDecimalsChainID provides a chain ID which is being set up with 6 decimals
	SixDecimalsChainID.EVMChainID: {
		Denom:         "utest",
		ExtendedDenom: "atest",
		DisplayDenom:  "test",
		Decimals:      evmtypes.SixDecimals.Uint32(),
	},
	// EVMChainID provides a chain ID used for internal testing
	config.DefaultEVMChainID: {
		Denom:         "atest",
		ExtendedDenom: "atest",
		DisplayDenom:  "test",
		Decimals:      evmtypes.EighteenDecimals.Uint32(),
	},
	TwelveDecimalsChainID.EVMChainID: {
		Denom:         "ptest2",
		ExtendedDenom: "atest2",
		DisplayDenom:  "test2",
		Decimals:      evmtypes.TwelveDecimals.Uint32(),
	},
	TwoDecimalsChainID.EVMChainID: {
		Denom:         "ctest3",
		ExtendedDenom: "atest3",
		DisplayDenom:  "test3",
		Decimals:      evmtypes.TwoDecimals.Uint32(),
	},
}

type ChainID struct {
	ChainID    string `json:"chain_id"`
	EVMChainID uint64 `json:"evm_chain_id"`
}

var (
	// ExampleChainIDPrefix provides a chain ID prefix for EIP-155 that can be used in tests
	ExampleChainIDPrefix = "cosmos"

	// ExampleChainID provides a chain ID that can be used in tests
	ExampleChainID = ChainID{
		ChainID:    ExampleChainIDPrefix + "-1",
		EVMChainID: 9001,
	}

	// SixDecimalsChainID provides a chain ID which is being set up with 6 decimals
	SixDecimalsChainID = ChainID{
		ChainID:    "ossix-2",
		EVMChainID: 9002,
	}

	// TwelveDecimalsChainID provides a chain ID which is being set up with 12 decimals
	TwelveDecimalsChainID = ChainID{
		ChainID:    "ostwelve-3",
		EVMChainID: 9003,
	}

	// TwoDecimalsChainID provides a chain ID which is being set up with 2 decimals
	TwoDecimalsChainID = ChainID{
		ChainID:    "ostwo-4",
		EVMChainID: 9004,
	}

	// ExampleChainCoinInfo provides the coin info for the example chain
	//
	// It is a map of the chain id and its corresponding EvmCoinInfo
	// that allows initializing the app with different coin info based on the
	// chain id
	ExampleChainCoinInfo = map[ChainID]evmtypes.EvmCoinInfo{
		ExampleChainID: {
			Denom:         ExampleAttoDenom,
			ExtendedDenom: ExampleAttoDenom,
			DisplayDenom:  ExampleDisplayDenom,
			Decimals:      evmtypes.EighteenDecimals.Uint32(),
		},
		SixDecimalsChainID: {
			Denom:         "utest",
			ExtendedDenom: "atest",
			DisplayDenom:  "test",
			Decimals:      evmtypes.SixDecimals.Uint32(),
		},
		TwelveDecimalsChainID: {
			Denom:         "ptest2",
			ExtendedDenom: "atest2",
			DisplayDenom:  "test2",
			Decimals:      evmtypes.TwelveDecimals.Uint32(),
		},
		TwoDecimalsChainID: {
			Denom:         "ctest3",
			ExtendedDenom: "atest3",
			DisplayDenom:  "test3",
			Decimals:      evmtypes.TwoDecimals.Uint32(),
		},
	}

	// OtherCoinDenoms provides a list of other coin denoms that can be used in tests
	OtherCoinDenoms = []string{
		"foo",
		"bar",
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

	// ExampleAllowances creates a slice of allowances, that contains an allowance for the native denom of the example chain
	// implementation.
	ExampleAllowances = []erc20types.Allowance{
		{
			Erc20Address: WEVMOSContractMainnet,
			Owner:        ExampleEvmAddressAlice,
			Spender:      ExampleEvmAddressBob,
			Value:        math.NewInt(100),
		},
	}
)
