package erc20

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	"cosmossdk.io/math"

	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const (
	// TransferMethod defines the ABI method name for the ERC-20 transfer
	// transaction.
	TransferMethod = "transfer"
	// TransferFromMethod defines the ABI method name for the ERC-20 transferFrom
	// transaction.
	TransferFromMethod = "transferFrom"
	// ApproveMethod defines the ABI method name for ERC-20 Approve
	// transaction.
	ApproveMethod = "approve"
)

// Transfer executes a direct transfer from the caller address to the
// destination address.
func (p *Precompile) Transfer(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	from := contract.Caller()
	to, amount, err := ParseTransferArgs(args)
	if err != nil {
		return nil, err
	}

	return p.transfer(ctx, contract, stateDB, method, from, to, amount)
}

// TransferFrom executes a transfer on behalf of the specified from address in
// the call data to the destination address.
func (p *Precompile) TransferFrom(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	from, to, amount, err := ParseTransferFromArgs(args)
	if err != nil {
		return nil, err
	}

	return p.transfer(ctx, contract, stateDB, method, from, to, amount)
}

// transfer is a common function that handles transfers for the ERC-20 Transfer
// and TransferFrom methods. It executes a bank Send message. If the spender isn't
// the sender of the transfer, it checks the allowance and updates it accordingly.
func (p *Precompile) transfer(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	from, to common.Address,
	amount *big.Int,
) (data []byte, err error) {
	coins := sdk.Coins{{Denom: p.tokenPair.Denom, Amount: math.NewIntFromBigInt(amount)}}

	msg := banktypes.NewMsgSend(from.Bytes(), to.Bytes(), coins)

	if err = msg.Amount.Validate(); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrInvalidAmount, err.Error())
	}

	isTransferFrom := method.Name == TransferFromMethod
	spenderAddr := contract.Caller()
	newAllowance := big.NewInt(0)

	if isTransferFrom {
		prevAllowance, err := p.erc20Keeper.GetAllowance(ctx, p.Address(), from, spenderAddr)
		if err != nil {
			return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrQueryFailed, TransferFromMethod, err.Error())
		}

		newAllowance = new(big.Int).Sub(prevAllowance, amount)
		if newAllowance.Sign() < 0 {
			return nil, cmn.NewRevertWithSolidityError(p.ABI, SolidityErrERC20InsufficientAllowance, spenderAddr, prevAllowance, amount)
		}

		if newAllowance.Sign() == 0 {
			err = p.erc20Keeper.DeleteAllowance(ctx, p.Address(), from, spenderAddr)
		} else {
			err = p.erc20Keeper.SetAllowance(ctx, p.Address(), from, spenderAddr, newAllowance)
		}
		if err != nil {
			return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrQueryFailed, TransferFromMethod, err.Error())
		}
	}

	msgSrv := NewMsgServerImpl(p.BankKeeper)
	if err = msgSrv.Send(ctx, msg); err != nil {
		spendable := p.BankKeeper.SpendableCoin(ctx, from.Bytes(), p.tokenPair.Denom)
		bal := spendable.Amount.BigInt()
		if amount.Cmp(bal) > 0 {
			return nil, cmn.NewRevertWithSolidityError(p.ABI, SolidityErrERC20InsufficientBalance, from, bal, amount)
		}
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, method.Name, err.Error())
	}

	if err = p.EmitTransferEvent(ctx, stateDB, from, to, amount); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, method.Name, err.Error())
	}

	if isTransferFrom {
		if err = p.EmitApprovalEvent(ctx, stateDB, from, spenderAddr, newAllowance); err != nil {
			return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, method.Name, err.Error())
		}
	}

	return method.Outputs.Pack(true)
}
