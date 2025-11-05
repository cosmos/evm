package bank

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

type BankMsgServer interface {
	// Send defines a method for sending coins from one account to another account.
	Send(context.Context, *banktypes.MsgSend) (*banktypes.MsgSendResponse, error)
}

type BankKeeper interface {
	GetSupply(ctx context.Context, denom string) sdk.Coin
	GetDenomMetaData(ctx context.Context, denom string) (banktypes.Metadata, bool)
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	IterateAccountBalances(ctx context.Context, addr sdk.AccAddress, cb func(sdk.Coin) bool)
	IterateTotalSupply(ctx context.Context, cb func(sdk.Coin) bool)
}
