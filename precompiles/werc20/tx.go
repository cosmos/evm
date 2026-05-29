package werc20

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// DepositMethod defines the ABI method name for the IWERC20 deposit
	// transaction.
	DepositMethod = "deposit"
	// WithdrawMethod defines the ABI method name for the IWERC20 withdraw
	// transaction.
	WithdrawMethod = "withdraw"
)

// Deposit handles the payable deposit function. It retrieves the deposited amount
// and sends it back to the sender using the bank keeper.
func (p Precompile) Deposit(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
) ([]byte, error) {
	caller := contract.Caller()
	depositedAmount := contract.Value()

	callerAccAddress := sdk.AccAddress(caller.Bytes())
	precompileAccAddr := sdk.AccAddress(p.Address().Bytes())

	// Send the coins back to the sender
	if err := p.BankKeeper.SendCoins(
		ctx,
		precompileAccAddr,
		callerAccAddress,
		sdk.NewCoins(sdk.Coin{
			Denom:  evmtypes.GetEVMCoinExtendedDenom(),
			Amount: math.NewIntFromBigInt(depositedAmount.ToBig()),
		}),
	); err != nil {
		return nil, err
	}

	if err := p.EmitDepositEvent(ctx, stateDB, caller, depositedAmount.ToBig()); err != nil {
		return nil, err
	}

	return nil, nil
}

// Withdraw is a no-op and mock function that provides the same interface as the
// WETH contract to support equality between the native coin and its wrapped
// ERC-20 (e.g. ATOM and WEVMOS).
func (p Precompile) Withdraw(ctx sdk.Context, contract *vm.Contract, stateDB vm.StateDB, args []interface{}) ([]byte, error) {
	amount, ok := args[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("invalid argument type: %T", args[0])
	}
	amountInt := math.NewIntFromBigInt(amount)

	caller := contract.Caller()
	callerAccAddress := sdk.AccAddress(caller.Bytes())
	nativeBalance := p.BankKeeper.SpendableCoin(ctx, callerAccAddress, evmtypes.GetEVMCoinDenom())
	if nativeBalance.Amount.Mul(types.ConversionFactor()).LT(amountInt) {
		return nil, fmt.Errorf("account balance %v is lower than withdraw balance %v", nativeBalance.Amount, amountInt)
	}

	if err := p.EmitWithdrawalEvent(ctx, stateDB, caller, amount); err != nil {
		return nil, err
	}
	return nil, nil
}
