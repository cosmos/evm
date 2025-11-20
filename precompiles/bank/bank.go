//
// The bank package contains the implementation of the x/bank module precompile.
// The precompiles returns all bank's information in the original decimals
// representation stored in the module.

package bank

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	_ "embed"

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

var _ vm.PrecompiledContract = &Precompile{}

var (
	// Embed abi json file to the executable binary. Needed when importing as dependency.
	//
	//go:embed abi.json
	f   []byte
	ABI abi.ABI
)

func init() {
	var err error
	ABI, err = abi.JSON(bytes.NewReader(f))
	if err != nil {
		panic(err)
	}
}

// Precompile defines the bank precompile
type Precompile struct {
	cmn.Precompile

	abi.ABI
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
		ABI:         ABI,
		bankKeeper:  bankKeeper,
		erc20Keeper: erc20Keeper,
	}
}

// RequiredGas calculates the precompiled contract's base gas rate.
func (p Precompile) RequiredGas(input []byte) uint64 {
	// NOTE: This check avoid panicking when trying to decode the method ID
	if len(input) < 4 {
		return 0
	}

	methodID := input[:4]

	method, err := p.MethodById(methodID)
	if err != nil {
		// This should never happen since this method is going to fail during Run
		return 0
	}

	switch method.Name {
	case BalancesMethod:
		return GasBalances
	case TotalSupplyMethod:
		return GasTotalSupply
	case SupplyOfMethod:
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
	method, args, err := cmn.SetupABI(p.ABI, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	var bz []byte
	switch method.Name {
	// Bank queries
	case BalancesMethod:
		bz, err = p.Balances(ctx, method, args)
	case TotalSupplyMethod:
		bz, err = p.TotalSupply(ctx, method, args)
	case SupplyOfMethod:
		bz, err = p.SupplyOf(ctx, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
// It returns false since all bank methods are queries.
func (Precompile) IsTransaction(_ *abi.Method) bool {
	return false
}
