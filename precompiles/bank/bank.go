//
// The bank package contains the implementation of the x/bank module precompile.
// The precompiles returns all bank's information in the original decimals
// representation stored in the module.

package bank

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// GasBalances defines the gas cost for a single ERC-20 balanceOf query
	GasBalances = 2_851

	// GasTotalSupply defines the gas cost for a single ERC-20 totalSupply query
	GasTotalSupply = 2_477

	// GasSupplyOf defines the gas cost for a single ERC-20 supplyOf query, taken from totalSupply of ERC20
	GasSupplyOf = 2_477
)

//go:generate go run ../cmd -input abi.json -output bank.abi.go

var _ vm.PrecompiledContract = &Precompile{}

// Precompile defines the bank precompile
type Precompile struct {
	cmn.Precompile

	bankKeeper  cmn.BankKeeper
	erc20Keeper cmn.ERC20Keeper
}

// NewPrecompile creates a new bank Precompile instance implementing the
// PrecompiledContract interface.
func NewPrecompile(
	bankKeeper cmn.BankKeeper,
	erc20Keeper cmn.ERC20Keeper,
) *Precompile {
	// NOTE: we set an empty gas configuration to avoid extra gas costs
	// during the run execution
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.GasConfig{},
			TransientKVGasConfig: storetypes.GasConfig{},
			ContractAddress:      common.HexToAddress(evmtypes.BankPrecompileAddress),
		},
		bankKeeper:  bankKeeper,
		erc20Keeper: erc20Keeper,
	}
}

// RequiredGas calculates the precompiled contract's base gas rate.
func (p Precompile) RequiredGas(input []byte) uint64 {
	methodID, _, err := cmn.SplitMethodID(input)
	if err != nil {
		return 0
	}

	switch methodID {
	case BalancesID:
		return GasBalances
	case TotalSupplyID:
		return GasTotalSupply
	case SupplyOfID:
		return GasSupplyOf
	}

	return 0
}

func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return p.Execute(ctx, contract, readonly)
	})
}

// Execute executes the precompiled contract bank query methods defined in the ABI.
func (p Precompile) Execute(ctx sdk.Context, contract *vm.Contract, readOnly bool) ([]byte, error) {
	methodID, input, err := cmn.ParseMethod(contract.Input, readOnly, func(uint32) bool {
		// all methods are queries
		return false
	})
	if err != nil {
		return nil, err
	}

	switch methodID {
	// Bank queries
	case BalancesID:
		return cmn.Run(ctx, p.Balances, input)
	case TotalSupplyID:
		return cmn.Run(ctx, p.TotalSupply, input)
	case SupplyOfID:
		return cmn.Run(ctx, p.SupplyOf, input)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, methodID)
	}
}
