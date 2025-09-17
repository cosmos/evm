package erc20

import (
	"embed"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"

	ibcutils "github.com/cosmos/evm/ibc"
	cmn "github.com/cosmos/evm/precompiles/common"
	erc20types "github.com/cosmos/evm/x/erc20/types"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// abiPath defines the path to the ERC-20 precompile ABI JSON file.
	abiPath = "abi.json"

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

// Embed abi json file to the executable binary. Needed when importing as dependency.
//
//go:embed abi.json
var f embed.FS

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

// LoadABI loads the IERC20Metadata ABI from the embedded abi.json file
// for the erc20 precompile.
func LoadABI() (abi.ABI, error) {
	return cmn.LoadABI(f, abiPath)
}

// NewPrecompile creates a new ERC-20 Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	tokenPair erc20types.TokenPair,
	bankKeeper cmn.BankKeeper,
	erc20Keeper Erc20Keeper,
	transferKeeper ibcutils.TransferKeeper,
	erc20ABI abi.ABI,
) (*Precompile, error) {
	p := &Precompile{
		Precompile: cmn.Precompile{
			ABI:                  erc20ABI,
			KvGasConfig:          storetypes.GasConfig{},
			TransientKVGasConfig: storetypes.GasConfig{},
		},
		tokenPair:      tokenPair,
		BankKeeper:     bankKeeper,
		erc20Keeper:    erc20Keeper,
		transferKeeper: transferKeeper,
	}
	// Address defines the address of the ERC-20 precompile contract.
	p.SetAddress(p.tokenPair.GetERC20Contract())

	// Set the balance handler for the precompile.
	p.SetBalanceHandler(bankKeeper)

	return p, nil
}

// RequiredGas calculates the contract gas used for the
func (p Precompile) RequiredGas(input []byte) uint64 {
	// NOTE: This check avoid panicking when trying to decode the method ID
	if len(input) < 4 {
		return 0
	}

	methodID := input[:4]
	method, err := p.MethodById(methodID)
	if err != nil {
		return 0
	}

	// TODO: these values were obtained from Remix using the ERC20.sol from OpenZeppelin.
	// We should execute the transactions using the ERC20MinterBurnerDecimals.sol from Cosmos EVM testnet
	// to ensure parity in the values.
	switch method.Name {
	// ERC-20 transactions
	case TransferMethod:
		return GasTransfer
	case TransferFromMethod:
		return GasTransferFrom
	case ApproveMethod:
		return GasApprove
	// ERC-20 queries
	case NameMethod:
		return GasName
	case SymbolMethod:
		return GasSymbol
	case DecimalsMethod:
		return GasDecimals
	case TotalSupplyMethod:
		return GasTotalSupply
	case BalanceOfMethod:
		return GasBalanceOf
	case AllowanceMethod:
		return GasAllowance
	default:
		return 0
	}
}

// Run executes the precompiled contract ERC-20 methods defined in the ABI.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readOnly bool) (bz []byte, err error) {
	bz, err = p.run(evm, contract, readOnly)
	if err != nil {
		return cmn.ReturnRevertError(evm, err)
	}

	return bz, nil
}

func (p Precompile) run(evm *vm.EVM, contract *vm.Contract, readOnly bool) (bz []byte, err error) {
	// ERC20 precompiles cannot receive funds because they are not managed by an
	// EOA and will not be possible to recover funds sent to an instance of
	// them.This check is a safety measure because currently funds cannot be
	// received due to the lack of a fallback handler.
	if value := contract.Value(); value.Sign() == 1 {
		return nil, fmt.Errorf(ErrCannotReceiveFunds, contract.Value().String())
	}

	ctx, stateDB, method, initialGas, args, err := p.RunSetup(evm, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	// Start the balance change handler before executing the precompile.
	p.GetBalanceHandler().BeforeBalanceChange(ctx)

	// This handles any out of gas errors that may occur during the execution of a precompile tx or query.
	// It avoids panics and returns the out of gas error so the EVM can continue gracefully.
	defer cmn.HandleGasError(ctx, contract, initialGas, &err)()

	bz, err = p.HandleMethod(ctx, contract, stateDB, method, args)
	if err != nil {
		return nil, err
	}

	cost := ctx.GasMeter().GasConsumed() - initialGas

	if !contract.UseGas(cost, nil, tracing.GasChangeCallPrecompiledContract) {
		return nil, vm.ErrOutOfGas
	}

	// Process the native balance changes after the method execution.
	if err = p.GetBalanceHandler().AfterBalanceChange(ctx, stateDB); err != nil {
		return nil, err
	}

	return bz, nil
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case TransferMethod,
		TransferFromMethod,
		ApproveMethod:
		return true
	default:
		return false
	}
}

// HandleMethod handles the execution of each of the ERC-20 methods.
func (p *Precompile) HandleMethod(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) (bz []byte, err error) {
	switch method.Name {
	// ERC-20 transactions
	case TransferMethod:
		bz, err = p.Transfer(ctx, contract, stateDB, method, args)
	case TransferFromMethod:
		bz, err = p.TransferFrom(ctx, contract, stateDB, method, args)
	case ApproveMethod:
		bz, err = p.Approve(ctx, contract, stateDB, method, args)
	// ERC-20 queries
	case NameMethod:
		bz, err = p.Name(ctx, contract, stateDB, method, args)
	case SymbolMethod:
		bz, err = p.Symbol(ctx, contract, stateDB, method, args)
	case DecimalsMethod:
		bz, err = p.Decimals(ctx, contract, stateDB, method, args)
	case TotalSupplyMethod:
		bz, err = p.TotalSupply(ctx, contract, stateDB, method, args)
	case BalanceOfMethod:
		bz, err = p.BalanceOf(ctx, contract, stateDB, method, args)
	case AllowanceMethod:
		bz, err = p.Allowance(ctx, contract, stateDB, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
}
