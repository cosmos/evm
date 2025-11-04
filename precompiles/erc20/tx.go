package erc20

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	"cosmossdk.io/math"

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
	args TransferCall,
	stateDB vm.StateDB,
	contract *vm.Contract,
) (*TransferReturn, error) {
	from := contract.Caller()
	success, err := p.transfer(ctx, stateDB, contract, from, args.To, args.Amount)
	if err != nil {
		return nil, err
	}
	return &TransferReturn{success}, nil
}

// TransferFrom executes a transfer on behalf of the specified from address in
// the call data to the destination address.
func (p *Precompile) TransferFrom(
	ctx sdk.Context,
	args TransferFromCall,
	stateDB vm.StateDB,
	contract *vm.Contract,
) (*TransferFromReturn, error) {
	success, err := p.transfer(ctx, stateDB, contract, args.From, args.To, args.Amount)
	if err != nil {
		return nil, err
	}
	return &TransferFromReturn{success}, nil
}

// transfer is a common function that handles transfers for the ERC-20 Transfer
// and TransferFrom methods. It executes a bank Send message. If the spender isn't
// the sender of the transfer, it checks the allowance and updates it accordingly.
func (p *Precompile) transfer(
	ctx sdk.Context,
	stateDB vm.StateDB,
	contract *vm.Contract,
	from, to common.Address,
	amount *big.Int,
) (bool, error) {
	coins := sdk.Coins{{Denom: p.tokenPair.Denom, Amount: math.NewIntFromBigInt(amount)}}

	msg := banktypes.NewMsgSend(from.Bytes(), to.Bytes(), coins)

	if err := msg.Amount.Validate(); err != nil {
		return false, err
	}

	isTransferFrom := from != contract.Caller()
	spenderAddr := contract.Caller()
	newAllowance := big.NewInt(0)

	if isTransferFrom {
		prevAllowance, err := p.erc20Keeper.GetAllowance(ctx, p.Address(), from, spenderAddr)
		if err != nil {
			return false, ConvertErrToERC20Error(err)
		}

		newAllowance = new(big.Int).Sub(prevAllowance, amount)
		if newAllowance.Sign() < 0 {
			return false, ErrInsufficientAllowance
		}

		if newAllowance.Sign() == 0 {
			// If the new allowance is 0, we need to delete it from the store.
			err = p.erc20Keeper.DeleteAllowance(ctx, p.Address(), from, spenderAddr)
		} else {
			// If the new allowance is not 0, we need to set it in the store.
			err = p.erc20Keeper.SetAllowance(ctx, p.Address(), from, spenderAddr, newAllowance)
		}
		if err != nil {
			return false, ConvertErrToERC20Error(err)
		}
	}

	msgSrv := NewMsgServerImpl(p.BankKeeper)
	if err := msgSrv.Send(ctx, msg); err != nil {
		// This should return an error to avoid the contract from being executed and an event being emitted
		return false, ConvertErrToERC20Error(err)
	}

	if err := p.EmitTransferEvent(ctx, stateDB, from, to, amount); err != nil {
		return false, err
	}

	// NOTE: if it's a direct transfer, we return here but if used through transferFrom,
	// we need to emit the approval event with the new allowance.
	if isTransferFrom {
		if err := p.EmitApprovalEvent(ctx, stateDB, from, spenderAddr, newAllowance); err != nil {
			return false, err
		}
	}

	return true, nil
}
