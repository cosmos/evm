package types

import (
	"context"
	"cosmossdk.io/x/feegrant"
	"github.com/cosmos/cosmos-sdk/x/authz"
	"github.com/cosmos/cosmos-sdk/x/bank/types"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"

	"cosmossdk.io/core/address"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
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
	UnorderedTransactionsEnabled() bool
	RemoveExpiredUnorderedNonces(ctx sdk.Context) error
	TryAddUnorderedNonce(ctx sdk.Context, sender []byte, timestamp time.Time) error
}

// BankKeeper defines the expected interface needed to retrieve account balances.
type BankKeeper interface {
	authtypes.BankKeeper
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins
	SpendableBalances(ctx context.Context, req *types.QuerySpendableBalancesRequest) (*types.QuerySpendableBalancesResponse, error)
	BlockedAddr(addr sdk.AccAddress) bool
	SendCoinsFromModuleToAccount(ctx context.Context, senderModule string, recipientAddr sdk.AccAddress, amt sdk.Coins) error
	SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error
	MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
	BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
}

// StakingKeeper returns the historical headers kept in store.
type StakingKeeper interface {
	GetHistoricalInfo(ctx context.Context, height int64) (stakingtypes.HistoricalInfo, error)
	GetValidatorByConsAddr(ctx context.Context, consAddr sdk.ConsAddress) (stakingtypes.Validator, error)
	ValidatorAddressCodec() address.Codec
	GetDelegatorDelegations(ctx context.Context, delegator sdk.AccAddress, maxRetrieve uint16) (delegations []stakingtypes.Delegation, err error)
	Unbond(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress, shares math.LegacyDec) (amount math.Int, err error)
	GetValidator(ctx context.Context, addr sdk.ValAddress) (validator stakingtypes.Validator, err error)
	BondDenom(ctx context.Context) (string, error)
	SetDelegation(ctx context.Context, delegation stakingtypes.Delegation) error
	RemoveDelegation(ctx context.Context, delegation stakingtypes.Delegation) error
}

type FeegrantKeeper interface {
	AllowancesByGranter(c context.Context, req *feegrant.QueryAllowancesByGranterRequest) (*feegrant.QueryAllowancesByGranterResponse, error)
	GrantAllowance(ctx context.Context, granter, grantee sdk.AccAddress, feeAllowance feegrant.FeeAllowanceI) error
}

type AuthzKeeper interface {
	GranterGrants(ctx context.Context, req *authz.QueryGranterGrantsRequest) (*authz.QueryGranterGrantsResponse, error)
	DeleteGrant(ctx context.Context, grantee, granter sdk.AccAddress, msgType string) error
	SaveGrant(ctx context.Context, grantee, granter sdk.AccAddress, authorization authz.Authorization, expiration *time.Time) error
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

// EvmHooks event hooks for evm tx processing
type EvmHooks interface {
	// Must be called after tx is processed successfully, if return an error, the whole transaction is reverted.
	PostTxProcessing(ctx sdk.Context, sender common.Address, msg core.Message, receipt *ethtypes.Receipt) error
}

// MigrateAccountHooks TODO
type MigrateAccountHooks interface {
	BeforeAll(ctx sdk.Context, originalAddress sdk.AccAddress, newAddress sdk.AccAddress) error
	AfterMigrateDelegations(ctx sdk.Context, originalAddress sdk.AccAddress, newAddress sdk.AccAddress) error
	AfterMigrateBankTokens(ctx sdk.Context, originalAddress sdk.AccAddress, newAddress sdk.AccAddress) error
	AfterMigrateFeeGrants(ctx sdk.Context, originalAddress sdk.AccAddress, newAddress sdk.AccAddress) error
	AfterAll(ctx sdk.Context, originalAddress sdk.AccAddress, newAddress sdk.AccAddress) error
}

// BankWrapper defines the methods required by the wrapper around
// the Cosmos SDK x/bank keeper that is used to manage an EVM coin
// with a configurable value for decimals.
type BankWrapper interface {
	BankKeeper

	MintAmountToAccount(ctx context.Context, recipientAddr sdk.AccAddress, amt *big.Int) error
	BurnAmountFromAccount(ctx context.Context, account sdk.AccAddress, amt *big.Int) error
}
