package erc20factory

import (
	"context"

	"github.com/ethereum/go-ethereum/common"

	erc20types "github.com/cosmos/evm/x/erc20/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

type ERC20Keeper interface {
	SetToken(ctx sdk.Context, token erc20types.TokenPair)
	EnableDynamicPrecompiles(ctx sdk.Context, addresses ...common.Address) error
	IsDenomRegistered(ctx sdk.Context, denom string) bool
}

type BankKeeper interface {
	GetDenomMetaData(ctx context.Context, denom string) (banktypes.Metadata, bool)
	SetDenomMetaData(ctx context.Context, denomMetaData banktypes.Metadata)
}
