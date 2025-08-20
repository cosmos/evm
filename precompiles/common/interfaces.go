package common

import (
	"context"

	ethcommon "github.com/ethereum/go-ethereum/common"

	erc20types "github.com/cosmos/evm/x/erc20/types"
	ibctypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	connectiontypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type BankKeeper interface {
	IterateAccountBalances(ctx context.Context, account sdk.AccAddress, cb func(coin sdk.Coin) bool)
	IterateTotalSupply(ctx context.Context, cb func(coin sdk.Coin) bool)
	GetSupply(ctx context.Context, denom string) sdk.Coin
	GetDenomMetaData(ctx context.Context, denom string) (banktypes.Metadata, bool)
	SetDenomMetaData(ctx context.Context, denomMetaData banktypes.Metadata)
	GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	SendCoins(ctx context.Context, fromAddr sdk.AccAddress, toAddr sdk.AccAddress, amt sdk.Coins) error
	SpendableCoin(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin
	BlockedAddr(addr sdk.AccAddress) bool
}

type TransferKeeper interface {
	Denom(ctx context.Context, req *ibctypes.QueryDenomRequest) (*ibctypes.QueryDenomResponse, error)
	Denoms(ctx context.Context, req *ibctypes.QueryDenomsRequest) (*ibctypes.QueryDenomsResponse, error)
	DenomHash(ctx context.Context, req *ibctypes.QueryDenomHashRequest) (*ibctypes.QueryDenomHashResponse, error)
	Transfer(ctx context.Context, msg *ibctypes.MsgTransfer) (*ibctypes.MsgTransferResponse, error)
}

type ChannelKeeper interface {
	GetChannel(ctx sdk.Context, portID, channelID string) (channeltypes.Channel, bool)
	GetConnection(ctx sdk.Context, connectionID string) (connectiontypes.ConnectionEnd, error)
}

type DistributionKeeper interface {
	WithdrawDelegationRewards(ctx context.Context, delAddr sdk.AccAddress, valAddr sdk.ValAddress) (sdk.Coins, error)
}

type StakingKeeper interface {
	BondDenom(ctx context.Context) (string, error)
	MaxValidators(ctx context.Context) (uint32, error)
	GetDelegatorValidators(ctx context.Context, delegatorAddr sdk.AccAddress, maxRetrieve uint32) (stakingtypes.Validators, error)
	GetRedelegation(ctx context.Context, delAddr sdk.AccAddress, valSrcAddr, valDstAddr sdk.ValAddress) (red stakingtypes.Redelegation, err error)
	GetValidator(ctx context.Context, addr sdk.ValAddress) (validator stakingtypes.Validator, err error)
}

type SlashingKeeper interface {
	Params(ctx context.Context, req *slashingtypes.QueryParamsRequest) (*slashingtypes.QueryParamsResponse, error)
	SigningInfo(ctx context.Context, req *slashingtypes.QuerySigningInfoRequest) (*slashingtypes.QuerySigningInfoResponse, error)
	SigningInfos(ctx context.Context, req *slashingtypes.QuerySigningInfosRequest) (*slashingtypes.QuerySigningInfosResponse, error)
}

type ERC20Keeper interface {
	GetCoinAddress(ctx sdk.Context, denom string) (ethcommon.Address, error)
	GetERC20Map(ctx sdk.Context, erc20 ethcommon.Address) []byte
	GetTokenPair(ctx sdk.Context, id []byte) (erc20types.TokenPair, bool)
}
