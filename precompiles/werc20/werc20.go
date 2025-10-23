package werc20

import (
	"encoding/binary"
	"slices"

	"github.com/ethereum/go-ethereum/core/vm"

	ibcutils "github.com/cosmos/evm/ibc"
	cmn "github.com/cosmos/evm/precompiles/common"
	erc20 "github.com/cosmos/evm/precompiles/erc20"
	erc20types "github.com/cosmos/evm/x/erc20/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

//go:generate go run github.com/yihuang/go-abi/cmd -input abi.json -output werc20.abi.go

var _ vm.PrecompiledContract = &Precompile{}

// Precompile defines the precompiled contract for WERC20.
type Precompile struct {
	*erc20.Precompile
}

const (
	// DepositRequiredGas defines the gas required for the Deposit transaction.
	DepositRequiredGas uint64 = 23_878
	// WithdrawRequiredGas defines the gas required for the Withdraw transaction.
	WithdrawRequiredGas uint64 = 9207
)

// NewPrecompile creates a new WERC20 Precompile instance implementing the
// PrecompiledContract interface. This type wraps around the ERC20 Precompile
// instance to provide additional methods.
func NewPrecompile(
	tokenPair erc20types.TokenPair,
	bankKeeper cmn.BankKeeper,
	erc20Keeper Erc20Keeper,
	transferKeeper ibcutils.TransferKeeper,
) *Precompile {
	erc20Precompile := erc20.NewPrecompile(tokenPair, bankKeeper, erc20Keeper, transferKeeper)

	return &Precompile{
		Precompile: erc20Precompile,
	}
}

// RequiredGas calculates the contract gas use.
func (p Precompile) RequiredGas(input []byte) uint64 {
	// TODO: these values were obtained from Remix using the WEVMOS9.sol.
	// We should execute the transactions from Cosmos EVM testnet
	// to ensure parity in the values.

	// If there is no method ID, then it's the fallback or receive case
	if len(input) < 4 {
		return DepositRequiredGas
	}

	methodID := binary.BigEndian.Uint32(input[:4])

	switch methodID {
	case DepositID:
		return DepositRequiredGas
	case WithdrawID:
		return WithdrawRequiredGas
	default:
		return p.Precompile.RequiredGas(input)
	}
}

func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return p.Execute(ctx, evm.StateDB, contract, readonly)
	})
}

func (p Precompile) Execute(ctx sdk.Context, stateDB vm.StateDB, contract *vm.Contract, readOnly bool) ([]byte, error) {
	methodID, input, err := cmn.ParseMethod(contract.Input, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	var bz []byte

	switch {
	case methodID == 0, // fallback or receive
		methodID == DepositID:
		bz, err = p.Deposit(ctx, contract, stateDB)
	case methodID == WithdrawID:
		return cmn.RunWithStateDB(ctx, p.Withdraw, input, stateDB, contract)
	default:
		// ERC20 transactions and queries
		bz, err = p.Precompile.Execute(ctx, stateDB, contract, readOnly)
	}

	return bz, err
}

// IsTransaction returns true if the given method name correspond to a
// transaction. Returns false otherwise.
func (p Precompile) IsTransaction(methodID uint32) bool {
	txMethodIDs := []uint32{DepositID, WithdrawID}

	if slices.Contains(txMethodIDs, methodID) || methodID == 0 {
		return true
	}

	return p.Precompile.IsTransaction(methodID)
}
