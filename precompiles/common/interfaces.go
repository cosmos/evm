package common

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type BankKeeper interface {
	IterateAccountBalances(ctx context.Context, account sdk.AccAddress, cb func(coin sdk.Coin) bool)
	IterateTotalSupply(ctx context.Context, cb func(coin sdk.Coin) bool)
	GetSupply(ctx context.Context, denom string) sdk.Coin
}
