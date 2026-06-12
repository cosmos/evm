package erc20

import (
	"context"

	cmn "github.com/cosmos/evm/precompiles/common"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

type MsgServer struct {
	cmn.BankKeeper
}

// NewMsgServerImpl returns an implementation of the bank MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper cmn.BankKeeper) *MsgServer {
	return &MsgServer{
		BankKeeper: keeper,
	}
}

func (m MsgServer) Send(goCtx context.Context, msg *banktypes.MsgSend) error {
	switch keeper := m.BankKeeper.(type) {
	// have cases for both pointer and non-pointer to cover how different apps could be storing the keeper
	case bankkeeper.BaseKeeper:
		msgSrv := bankkeeper.NewMsgServerImpl(keeper)
		if _, err := msgSrv.Send(goCtx, msg); err != nil {
			// This should return an error to avoid the contract from being executed and an event being emitted
			return ConvertErrToERC20Error(err)
		}
	case *bankkeeper.BaseKeeper:
		msgSrv := bankkeeper.NewMsgServerImpl(keeper)
		if _, err := msgSrv.Send(goCtx, msg); err != nil {
			// This should return an error to avoid the contract from being executed and an event being emitted
			return ConvertErrToERC20Error(err)
		}
	case precisebankkeeper.Keeper:
		if _, err := keeper.Send(goCtx, msg); err != nil {
			// This should return an error to avoid the contract from being executed and an event being emitted
			return ConvertErrToERC20Error(err)
		}
	case *precisebankkeeper.Keeper:
		if _, err := keeper.Send(goCtx, msg); err != nil {
			// This should return an error to avoid the contract from being executed and an event being emitted
			return ConvertErrToERC20Error(err)
		}
	default:
		return sdkerrors.ErrInvalidRequest.Wrapf("invalid keeper type: %T", m.BankKeeper)
	}
	return nil
}
