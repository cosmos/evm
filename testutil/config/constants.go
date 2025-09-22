package config

import (
	evmconfig "github.com/cosmos/evm/config"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// TestChainID1 is test chain IDs for IBC E2E test
	TestChainID1 = 9005
	// TestChainID2 is test chain IDs for IBC E2E test
	TestChainID2 = 9006
	// TwoDecimalsChainID is the chain ID for the 2 decimals chain.
	TwoDecimalsChainID = 9004
	// SixDecimalsChainID is the chain ID for the 6 decimals chain.
	SixDecimalsChainID = 9002
	// TwelveDecimalsChainID is the chain ID for the 12 decimals chain.
	TwelveDecimalsChainID = 9003
	// EighteenDecimalsChainID is the chain ID for the 18 decimals chain.
	EighteenDecimalsChainID = 9001
)

// TODO: update display denoms so that they less arbitrary (e.g. twodec, sixdec, twelvedec)
var (
	// TwoDecimalEvmCoinInfo is the EvmCoinInfo for the 2 decimals chain
	TwoDecimalEvmCoinInfo = evmtypes.EvmCoinInfo{
		DisplayDenom:  "test3",
		Decimals:      evmtypes.TwoDecimals,
		BaseDenom:     "ttest3",
		ExtendedDenom: "atest3",
	}
	// SixDecimalEvmCoinInfo is the EvmCoinInfo for the 6 decimals chain
	SixDecimalEvmCoinInfo = evmtypes.EvmCoinInfo{
		DisplayDenom:  "test",
		Decimals:      evmtypes.SixDecimals,
		BaseDenom:     "utest",
		ExtendedDenom: "atest",
	}
	// TwelveDecimalEvmCoinInfo is the EvmCoinInfo for a 12 decimals chain
	TwelveDecimalEvmCoinInfo = evmtypes.EvmCoinInfo{
		DisplayDenom:  "test2",
		Decimals:      evmtypes.TwelveDecimals,
		BaseDenom:     "twtest2",
		ExtendedDenom: "atest2",
	}
	// ExampleAttoDenom provides an example denom for use in tests
	ExampleAttoDenom = evmconfig.DefaultEvmCoinInfo.GetDenom()
	// ExampleMicroDenom provides an example micro denom for use in tests
	ExampleMicroDenom = SixDecimalEvmCoinInfo.GetDenom()
	// WevmosContractMainnet is the WEVMOS contract address for mainnet
	WevmosContractMainnet = evmconfig.DefaultWevmosContractMainnet
	// WevmosContractTestnet is the WEVMOS contract address for testnet
	WevmosContractTestnet = "0xcc491f589b45d4a3c679016195b3fb87d7848210"
	// ExampleEvmAddress1 is the example EVM address
	ExampleEvmAddressAlice = "0x1e0DE5DB1a39F99cBc67B00fA3415181b3509e42"
	// ExampleEvmAddress2 is the example EVM address
	ExampleEvmAddressBob = "0x0AFc8e15F0A74E98d0AEC6C67389D2231384D4B2"
)

// TestChainsCoinInfo is a map of the chain id and its corresponding EvmCoinInfo
// used to initialize the app with different coin info based on the chain id
var TestChainsCoinInfo = map[uint64]evmtypes.EvmCoinInfo{
	TestChainID1:                evmconfig.DefaultEvmCoinInfo,
	TestChainID2:                evmconfig.DefaultEvmCoinInfo,
	TwoDecimalsChainID:          TwoDecimalEvmCoinInfo,
	SixDecimalsChainID:          SixDecimalEvmCoinInfo,
	TwelveDecimalsChainID:       TwelveDecimalEvmCoinInfo,
	EighteenDecimalsChainID:     evmconfig.DefaultEvmCoinInfo,
	evmconfig.DefaultEvmChainID: evmconfig.DefaultEvmCoinInfo,
}

// TODO: consolidate the ChainID and uint64 maps and update tests accordingly
type ChainID struct {
	ChainID    string `json:"chain_id"`
	EVMChainID uint64 `json:"evm_chain_id"`
}

var (
	// ExampleChainID provides a chain ID that can be used in tests
	ExampleChainID = ChainID{
		ChainID:    sdk.Bech32MainPrefix + "-1",
		EVMChainID: EighteenDecimalsChainID,
	}
	// TwoDecimalsChainID provides a chain ID which is being set up with 2 decimals
	ExampleTwoDecimalsChainID = ChainID{
		ChainID:    "ostwo-4",
		EVMChainID: TwoDecimalsChainID,
	}
	// SixDecimalsChainID provides a chain ID which is being set up with 6 decimals
	ExampleSixDecimalsChainID = ChainID{
		ChainID:    "ossix-2",
		EVMChainID: SixDecimalsChainID,
	}
	// TwelveDecimalsChainID provides a chain ID which is being set up with 12 decimals
	ExampleTwelveDecimalsChainID = ChainID{
		ChainID:    "ostwelve-3",
		EVMChainID: TwelveDecimalsChainID,
	}
	// ExampleTokenPairs creates a slice of token pairs for the native denom of the example chain.
	ExampleTokenPairs = []erc20types.TokenPair{
		{
			Erc20Address:  WevmosContractMainnet,
			Denom:         ExampleAttoDenom,
			Enabled:       true,
			ContractOwner: erc20types.OWNER_MODULE,
		},
	}
	// ExampleAllowances creates a slice of allowances for the native denom of the example chain.
	ExampleAllowances = []erc20types.Allowance{
		{
			Erc20Address: WevmosContractMainnet,
			Owner:        ExampleEvmAddressAlice,
			Spender:      ExampleEvmAddressBob,
			Value:        math.NewInt(100),
		},
	}
	// OtherCoinDenoms provides a list of other coin denoms that can be used in tests
	OtherCoinDenoms = []string{
		"foo",
		"bar",
	}
)

// ExampleChainCoinInfo provides the coin info for the example chain
var ExampleChainCoinInfo = map[ChainID]evmtypes.EvmCoinInfo{
	ExampleChainID:               evmconfig.DefaultEvmCoinInfo,
	ExampleTwoDecimalsChainID:    TwoDecimalEvmCoinInfo,
	ExampleSixDecimalsChainID:    SixDecimalEvmCoinInfo,
	ExampleTwelveDecimalsChainID: TwelveDecimalEvmCoinInfo,
}
