package erc20

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/vm"

	_ "embed"

	ibcutils "github.com/cosmos/evm/ibc"
	cmn "github.com/cosmos/evm/precompiles/common"
	erc20types "github.com/cosmos/evm/x/erc20/types"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

//go:generate go run ../cmd -input abi.json -output erc20.abi.go

const (
	// NOTE: These gas values have been derived from tests that have been concluded on a testing branch, which
	// is not being merged to the main branch. The reason for this was to not clutter the repository with the
	// necessary tests for this use case.
	//
	// The results can be inspected here:
	// https://github.com/evmos/evmos/blob/malte/erc20-gas-tests/precompiles/erc20/plot_gas_values.ipynb

	GasTransfer     = 9_000
	GasTransferFrom = 30_500
	GasApprove      = 8_100
	GasName         = 3_421
	GasSymbol       = 3_464
	GasDecimals     = 427
	GasTotalSupply  = 2_480
	GasBalanceOf    = 2_870
	GasAllowance    = 3_225
)

var _ vm.PrecompiledContract = &Precompile{}

// Precompile defines the precompiled contract for ERC-20.
type Precompile struct {
	cmn.Precompile

	tokenPair      erc20types.TokenPair
	transferKeeper ibcutils.TransferKeeper
	erc20Keeper    Erc20Keeper
	// BankKeeper is a public field so that the werc20 precompile can use it.
	BankKeeper cmn.BankKeeper
}

// NewPrecompile creates a new ERC-20 Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	tokenPair erc20types.TokenPair,
	bankKeeper cmn.BankKeeper,
	erc20Keeper Erc20Keeper,
	transferKeeper ibcutils.TransferKeeper,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:           storetypes.GasConfig{},
			TransientKVGasConfig:  storetypes.GasConfig{},
			ContractAddress:       tokenPair.GetERC20Contract(),
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		tokenPair:      tokenPair,
		BankKeeper:     bankKeeper,
		erc20Keeper:    erc20Keeper,
		transferKeeper: transferKeeper,
	}
}

// RequiredGas calculates the contract gas used for the
func (p Precompile) RequiredGas(input []byte) uint64 {
	methodID, input, err := cmn.SplitMethodID(input)
	if err != nil {
		return 0
	}

	// TODO: these values were obtained from Remix using the ERC20.sol from OpenZeppelin.
	// We should execute the transactions using the ERC20MinterBurnerDecimals.sol from Cosmos EVM testnet
	// to ensure parity in the values.
	switch methodID {
	// ERC-20 transactions
	case TransferID:
		return GasTransfer
	case TransferFromID:
		return GasTransferFrom
	case ApproveID:
		return GasApprove
	// ERC-20 queries
	case NameID:
		return GasName
	case SymbolID:
		return GasSymbol
	case DecimalsID:
		return GasDecimals
	case TotalSupplyID:
		return GasTotalSupply
	case BalanceOfID:
		return GasBalanceOf
	case AllowanceID:
		return GasAllowance
	default:
		return 0
	}
}

func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return p.Execute(ctx, evm.StateDB, contract, readonly)
	})
}

func (p Precompile) Execute(ctx sdk.Context, stateDB vm.StateDB, contract *vm.Contract, readOnly bool) ([]byte, error) {
	// ERC20 precompiles cannot receive funds because they are not managed by an
	// EOA and will not be possible to recover funds sent to an instance of
	// them.This check is a safety measure because currently funds cannot be
	// received due to the lack of a fallback handler.
	if value := contract.Value(); value.Sign() == 1 {
		return nil, fmt.Errorf(ErrCannotReceiveFunds, contract.Value().String())
	}

	methodID, input, err := cmn.ParseMethod(contract.Input, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	switch methodID {
	// ERC-20 transactions
	case TransferID:
		return cmn.RunWithStateDB(ctx, p.Transfer, input, stateDB, contract)
	case TransferFromID:
		return cmn.RunWithStateDB(ctx, p.TransferFrom, input, stateDB, contract)
	case ApproveID:
		return cmn.RunWithStateDB(ctx, p.Approve, input, stateDB, contract)
	// ERC-20 queries
	case NameID:
		return cmn.Run(ctx, p.Name, input)
	case SymbolID:
		return cmn.Run(ctx, p.Symbol, input)
	case DecimalsID:
		return cmn.Run(ctx, p.Decimals, input)
	case TotalSupplyID:
		return cmn.Run(ctx, p.TotalSupply, input)
	case BalanceOfID:
		return cmn.Run(ctx, p.BalanceOf, input)
	case AllowanceID:
		return cmn.Run(ctx, p.Allowance, input)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, methodID)
	}
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
func (Precompile) IsTransaction(methodID uint32) bool {
	switch methodID {
	case TransferID,
		TransferFromID,
		ApproveID:
		return true
	default:
		return false
	}
}
