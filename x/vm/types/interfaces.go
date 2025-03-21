package types

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/cosmos/evm/x/vm/core/vm"

	"cosmossdk.io/core/address"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	paramtypes "github.com/cosmos/cosmos-sdk/x/params/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// AccountKeeper defines the expected account keeper interface
type AccountKeeper interface {
	NewAccountWithAddress(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	GetModuleAddress(moduleName string) sdk.AccAddress
	GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	SetAccount(ctx context.Context, account sdk.AccountI)
	RemoveAccount(ctx context.Context, account sdk.AccountI)
	GetParams(ctx context.Context) (params authtypes.Params)
	GetSequence(ctx context.Context, account sdk.AccAddress) (uint64, error)
	AddressCodec() address.Codec
}

// BankKeeper defines the expected interface needed to retrieve account balances.
type BankKeeper interface {
	authtypes.BankKeeper
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
}

// StakingKeeper returns the historical headers kept in store.
type StakingKeeper interface {
	GetHistoricalInfo(ctx context.Context, height int64) (stakingtypes.HistoricalInfo, error)
	GetValidatorByConsAddr(ctx context.Context, consAddr sdk.ConsAddress) (stakingtypes.Validator, error)
	ValidatorAddressCodec() address.Codec
}

// FeeMarketKeeper defines the expected interfaces needed for the feemarket
type FeeMarketKeeper interface {
	GetBaseFee(ctx sdk.Context) math.LegacyDec
	GetParams(ctx sdk.Context) feemarkettypes.Params
	CalculateBaseFee(ctx sdk.Context) math.LegacyDec
}

// Erc20Keeper defines the expected interface needed to instantiate ERC20 precompiles.
type Erc20Keeper interface {
	GetERC20PrecompileInstance(ctx sdk.Context, address common.Address) (contract vm.PrecompiledContract, found bool, err error)
}

type (
	LegacyParams = paramtypes.ParamSet
	// Subspace defines an interface that implements the legacy Cosmos SDK x/params Subspace type.
	// NOTE: This is used solely for migration of the Cosmos SDK x/params managed parameters.
	Subspace interface {
		GetParamSetIfExists(ctx sdk.Context, ps LegacyParams)
	}
)

// BankWrapper defines the methods required by the wrapper around
// the Cosmos SDK x/bank keeper that is used to manage an EVM coin
// with a configurable value for decimals.
type BankWrapper interface {
	BankKeeper

	MintAmountToAccount(ctx context.Context, recipientAddr sdk.AccAddress, amt *big.Int) error
	BurnAmountFromAccount(ctx context.Context, account sdk.AccAddress, amt *big.Int) error
}
