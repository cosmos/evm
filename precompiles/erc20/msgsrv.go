package erc20

import (
	"context"

	cmn "github.com/cosmos/evm/precompiles/common"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

type MsgServer struct {
	cmn.BankKeeper
}

// bankSender is the subset of bank keeper behaviour that Send needs. Any
// keeper implementing it works, including wrappers around the SDK
// BaseKeeper (by value or by pointer).
type bankSender interface {
	IsSendEnabledCoins(ctx context.Context, coins ...sdk.Coin) error
	BlockedAddr(addr sdk.AccAddress) bool
	SendCoins(ctx context.Context, fromAddr, toAddr sdk.AccAddress, amt sdk.Coins) error
}

// NewMsgServerImpl returns an implementation of the bank MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper cmn.BankKeeper) *MsgServer {
	return &MsgServer{
		BankKeeper: keeper,
	}
}

// Send performs the same checks as the x/bank MsgServer.Send handler and then
// transfers the coins. It works against bankSender instead of delegating to
// bankkeeper.NewMsgServerImpl, because that handler only accepts a
// bankkeeper.BaseKeeper value and rejects pointers and wrapper keepers.
//
// Any error is returned to avoid the contract from being executed and an
// event being emitted.
func (m MsgServer) Send(goCtx context.Context, msg *banktypes.MsgSend) error {
	keeper, ok := m.BankKeeper.(bankSender)
	if !ok {
		return sdkerrors.ErrInvalidRequest.Wrapf("invalid keeper type: %T", m.BankKeeper)
	}

	from, err := sdk.AccAddressFromBech32(msg.FromAddress)
	if err != nil {
		return ConvertErrToERC20Error(sdkerrors.ErrInvalidAddress.Wrapf("invalid from address: %s", err))
	}
	to, err := sdk.AccAddressFromBech32(msg.ToAddress)
	if err != nil {
		return ConvertErrToERC20Error(sdkerrors.ErrInvalidAddress.Wrapf("invalid to address: %s", err))
	}

	if !msg.Amount.IsValid() || !msg.Amount.IsAllPositive() {
		return ConvertErrToERC20Error(errorsmod.Wrap(sdkerrors.ErrInvalidCoins, msg.Amount.String()))
	}

	if err := keeper.IsSendEnabledCoins(goCtx, msg.Amount...); err != nil {
		return ConvertErrToERC20Error(err)
	}

	if keeper.BlockedAddr(to) {
		return ConvertErrToERC20Error(errorsmod.Wrapf(sdkerrors.ErrUnauthorized, "%s is not allowed to receive funds", msg.ToAddress))
	}

	if err := keeper.SendCoins(goCtx, from, to, msg.Amount); err != nil {
		return ConvertErrToERC20Error(err)
	}

	return nil
}
