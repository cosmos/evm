package evmd

import (
	"cosmossdk.io/math"

	erc20types "github.com/cosmos/evm/x/erc20/types"
)

const (
	ExampleSpender = "0x1e0DE5DB1a39F99cBc67B00fA3415181b3509e42"
	ExampleOwner   = "0x0AFc8e15F0A74E98d0AEC6C67389D2231384D4B2"
)

// ExampleAllowances creates a slice of allowance, that contains an allowance for the native denom of the example chain
// implementation.
var ExampleAllowances = []erc20types.Allowance{
	{
		Erc20Address: WEVMOSContractMainnet,
		Owner:        ExampleOwner,
		Spender:      ExampleSpender,
		Value:        math.NewInt(100),
	},
}
