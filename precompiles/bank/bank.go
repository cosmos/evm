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

//go:generate go run ../cmd -var=HumanABI -output bank.abi.go

var HumanABI = []string{
	// backwards compatibility
	"struct Balance{ address contractAddress; uint amount; }",
	"function balances(address account) returns (Balance[] balances)",
	"function totalSupply() returns (Balance[] totalSupply)",
	"function supplyOf(address contract) returns (uint totalSupply)",

	// v2 design
	"function name(string denom) returns (string name)",
	"function symbol(string denom) returns (string symbol)",
	"function decimals(string denom) returns (uint8 decimals)",
	"function totalSupply(string denom) returns (uint256 supply)",
	"function balanceOf(address account, string denom) returns (uint256 balance)",
	"function transferFrom(address from, address to, uint256 value, string denom) returns (bool)",

	// generate the erc20 constructor abi
	"function erc20ctor(string denom, address bank)",
}

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

const (
	// GasBalances defines the gas cost for a single ERC-20 balanceOf query
	GasBalances = 2_851

	// GasTotalSupply defines the gas cost for a single ERC-20 totalSupply query
	GasTotalSupply = 2_477

	// GasSupplyOf defines the gas cost for a single ERC-20 supplyOf query, taken from totalSupply of ERC20
	GasSupplyOf = 2_477

	TransferFromMethod = "transferFrom"
)

var _ vm.PrecompiledContract = &Precompile{}

// Precompile defines the bank precompile
type Precompile struct {
	cmn.Precompile

	abi.ABI
	bankMsgServer cmn.BankMsgServer
	bankKeeper    cmn.BankKeeper
	erc20Keeper   cmn.ERC20Keeper
}

// NewPrecompile creates a new bank Precompile instance implementing the
// PrecompiledContract interface.
func NewPrecompile(
	bankMsgServer cmn.BankMsgServer,
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
		ABI:           ABI,
		bankMsgServer: bankMsgServer,
		bankKeeper:    bankKeeper,
		erc20Keeper:   erc20Keeper,
	}
}

// RequiredGas calculates the precompiled contract's base gas rate.
func (p Precompile) RequiredGas(input []byte) uint64 {
	methodID, input, err := cmn.SplitMethodID(input)
	if err != nil {
		return 0
	}

	// backward compatibility
	switch methodID {
	case BalancesID:
		return GasBalances
	case TotalSupplyID:
		return GasTotalSupply
	case SupplyOfID:
		return GasSupplyOf
	}

	return p.Precompile.RequiredGas(input, p.IsTransactionID(methodID))
}

func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return p.Execute(ctx, evm.StateDB, contract, readonly)
	})
}

// Execute executes the precompiled contract bank query methods defined in the ABI.
func (p Precompile) Execute(ctx sdk.Context, stateDB vm.StateDB, contract *vm.Contract, readOnly bool) ([]byte, error) {
	methodID, input, err := cmn.ParseMethod(contract.Input, readOnly, p.IsTransactionID)
	if err != nil {
		return nil, err
	}

	switch methodID {
	// backward compatibility
	case BalancesID:
		return cmn.Run(ctx, p.Balances, input)
	case TotalSupplyID:
		return cmn.Run(ctx, p.TotalSupply, input)
	case SupplyOfID:
		return cmn.Run(ctx, p.SupplyOf, input)

	// v2 design
	case NameID:
		return cmn.Run(ctx, p.Name, input)
	case SymbolID:
		return cmn.Run(ctx, p.Symbol, input)
	case DecimalsID:
		return cmn.Run(ctx, p.Decimals, input)
	case TotalSupply0ID:
		return cmn.Run(ctx, p.TotalSupplyV2, input)
	case BalanceOfID:
		return cmn.Run(ctx, p.BalanceOf, input)
	case TransferFromID:
		return cmn.RunWithStateDB(ctx, p.TransferFrom, input, stateDB, contract)

	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethodID, methodID)
	}

}

// IsTransaction checks if the given method name corresponds to a transaction or query.
// It returns false since all bank methods are queries.
func (Precompile) IsTransaction(method *abi.Method) bool {
	return method.Name == TransferFromMethod
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
// It returns false since all bank methods are queries.
func (Precompile) IsTransactionID(methodID uint32) bool {
	return methodID == TransferFromID
}
