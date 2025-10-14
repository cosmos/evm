package types

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// BankKeeper defines the expected interface needed to retrieve account balances.
type BankKeeper interface {
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	IterateAllBalances(ctx context.Context, cb func(address sdk.AccAddress, coin sdk.Coin) (stop bool))
	GetSupply(ctx context.Context, denom string) sdk.Coin
}

// StakingKeeper defines the expected interface needed to retrieve staking information.
type StakingKeeper interface {
	GetAllDelegations(ctx context.Context) ([]stakingtypes.Delegation, error)
	GetAllUnbondingDelegations(ctx context.Context, delegator sdk.AccAddress) ([]stakingtypes.UnbondingDelegation, error)
	BondDenom(ctx context.Context) (string, error)
}

// AccountKeeper defines the expected interface needed to retrieve account information.
type AccountKeeper interface {
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	IterateAccounts(ctx context.Context, cb func(account sdk.AccountI) (stop bool))
}
