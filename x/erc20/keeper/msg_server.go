package keeper

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/statedb"

	sdkerrors "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ types.MsgServer = &Keeper{}

// ConvertERC20 converts ERC20 tokens into native Cosmos coins for both
// Cosmos-native and ERC20 TokenPair Owners
func (k Keeper) ConvertERC20(
	goCtx context.Context,
	msg *types.MsgConvertERC20,
) (*types.MsgConvertERC20Response, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Parse message
	receiver, err := sdk.AccAddressFromBech32(msg.Receiver)
	if err != nil {
		return nil, err
	}
	sender := common.HexToAddress(msg.Sender)
	contract := common.HexToAddress(msg.ContractAddress)

	// Create stateDB for this transaction
	stateDB := statedb.New(ctx, k.evmKeeper, statedb.NewEmptyTxConfig())

	return k.ConvertERC20IntoCoinsForNativeToken(ctx, stateDB, contract, msg.Amount, receiver, sender, true, false)
}

// ConvertCoin converts native Cosmos coins into ERC20 tokens for both
// Cosmos-native and ERC20 TokenPair Owners
func (k Keeper) ConvertCoin(
	goCtx context.Context,
	msg *types.MsgConvertCoin,
) (*types.MsgConvertCoinResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	// Error checked during msg validation
	sender := sdk.MustAccAddressFromBech32(msg.Sender)
	receiver := common.HexToAddress(msg.Receiver)

	pair, err := k.MintingEnabled(ctx, receiver.Bytes(), msg.Coin.Denom)
	if err != nil {
		return nil, err
	}

	// Check ownership and execute conversion
	switch {
	case pair.IsNativeERC20():
		// Remove token pair if contract is suicided
		acc := k.evmKeeper.GetAccountWithoutBalance(ctx, pair.GetERC20Contract())
		if acc == nil || !acc.HasCodeHash() {
			k.DeleteTokenPair(ctx, pair)
			k.Logger(ctx).Debug(
				"deleting selfdestructed token pair from state",
				"contract", pair.Erc20Address,
			)
			// NOTE: return nil error to persist the changes from the deletion
			return nil, nil
		}

		return nil, k.ConvertCoinNativeERC20(ctx, pair, msg.Coin.Amount, receiver, sender, false)
	case pair.IsNativeCoin():
		return nil, types.ErrNativeConversionDisabled
	}

	return nil, types.ErrUndefinedOwner
}

// ConvertCoinNativeERC20 handles the coin conversion for a native ERC20 token
// pair:
//   - escrow Coins on module account
//   - unescrow Tokens that have been previously escrowed with ConvertERC20 and send to receiver
//   - burn escrowed Coins
//   - check if token balance increased by amount
//   - check for unexpected `Approval` event in logs
func (k Keeper) ConvertCoinNativeERC20(ctx sdk.Context, pair types.TokenPair, amount math.Int, receiver common.Address, sender sdk.AccAddress, callFromPrecompile bool) error {
	if !amount.IsPositive() {
		return sdkerrors.Wrap(types.ErrNegativeToken, "converted coin amount must be positive")
	}

	erc20 := contracts.ERC20MinterBurnerDecimalsContract.ABI
	contract := pair.GetERC20Contract()

	balanceToken := k.BalanceOf(ctx, erc20, contract, receiver)
	if balanceToken == nil {
		return sdkerrors.Wrap(types.ErrEVMCall, "failed to retrieve balance")
	}

	// Escrow Coins on module account
	coins := sdk.Coins{{Denom: pair.Denom, Amount: amount}}
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, coins); err != nil {
		return sdkerrors.Wrap(err, "failed to escrow coins")
	}

	// Unescrow Tokens and send to receiver
	stateDB := statedb.New(ctx, k.evmKeeper, statedb.NewEmptyTxConfig())
	res, err := k.evmKeeper.CallEVM(ctx, stateDB, erc20, types.ModuleAddress, contract, true, callFromPrecompile, nil, "transfer", receiver, amount.BigInt())
	if err != nil {
		return err
	}

	// Check unpackedRet execution
	var unpackedRet types.ERC20BoolResponse
	if len(res.Ret) == 0 {
		// if the token does not return a value, check for the transfer event in logs
		if err := validateTransferEventExists(res.Logs, contract); err != nil {
			return err
		}
	} else {
		if err := erc20.UnpackIntoInterface(&unpackedRet, "transfer", res.Ret); err != nil {
			return err
		}
		if !unpackedRet.Value {
			return sdkerrors.Wrap(errortypes.ErrLogic, "failed to execute unescrow tokens from user")
		}
	}

	// Check expected Receiver balance after transfer execution
	balanceTokenAfter := k.BalanceOf(ctx, erc20, contract, receiver)
	if balanceTokenAfter == nil {
		return sdkerrors.Wrap(types.ErrEVMCall, "failed to retrieve balance")
	}

	exp := big.NewInt(0).Add(balanceToken, amount.BigInt())

	if r := balanceTokenAfter.Cmp(exp); r != 0 {
		return sdkerrors.Wrapf(
			types.ErrBalanceInvariance,
			"invalid token balance - expected: %v, actual: %v", exp, balanceTokenAfter,
		)
	}

	// Burn escrowed Coins
	err = k.bankKeeper.BurnCoins(ctx, types.ModuleName, coins)
	if err != nil {
		return sdkerrors.Wrap(err, "failed to burn coins")
	}

	return nil
}

// UpdateParams implements the gRPC MsgServer interface. After a successful governance vote
// it updates the parameters in the keeper only if the requested authority
// is the Cosmos SDK governance module account, unless an authority address is
// configured via the consensus AuthorityParams, in which case that takes
// precedence.
func (k *Keeper) UpdateParams(goCtx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := k.validateAuthority(ctx, req.Authority); err != nil {
		return nil, err
	}

	if err := k.SetParams(ctx, req.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}

// RegisterERC20 implements the gRPC MsgServer interface. Any account can permissionlessly
// register a native ERC20 contract to map to a Cosmos Coin.
func (k *Keeper) RegisterERC20(goCtx context.Context, req *types.MsgRegisterERC20) (*types.MsgRegisterERC20Response, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	params := k.GetParams(ctx)

	if !params.PermissionlessRegistration {
		if err := k.validateAuthority(ctx, req.Signer); err != nil {
			return nil, err
		}
	}

	// Check if the conversion is globally enabled
	if !k.IsERC20Enabled(ctx) {
		return nil, types.ErrERC20Disabled.Wrap("registration is currently disabled by governance")
	}

	for _, addr := range req.Erc20Addresses {
		if !common.IsHexAddress(addr) {
			return nil, errortypes.ErrInvalidAddress.Wrapf("invalid ERC20 contract address: %s", addr)
		}

		pair, err := k.registerERC20(ctx, common.HexToAddress(addr))
		if err != nil {
			return nil, err
		}

		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeRegisterERC20,
				sdk.NewAttribute(types.AttributeKeyCosmosCoin, pair.Denom),
				sdk.NewAttribute(types.AttributeKeyERC20Token, pair.Erc20Address),
			),
		)
	}

	return &types.MsgRegisterERC20Response{}, nil
}

// ToggleConversion implements the gRPC MsgServer interface.
//
// After a successful governance vote it adjusts the possibility of converting tokens between their
// conversions according to the outcome of the vote.
func (k *Keeper) ToggleConversion(goCtx context.Context, req *types.MsgToggleConversion) (*types.MsgToggleConversionResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	// Check if the conversion is globally enabled
	if !k.IsERC20Enabled(ctx) {
		return nil, types.ErrERC20Disabled.Wrap("toggle conversion is currently disabled by governance")
	}

	if err := k.validateAuthority(ctx, req.Authority); err != nil {
		return nil, err
	}

	pair, err := k.toggleConversion(ctx, req.Token)
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeToggleTokenConversion,
			sdk.NewAttribute(types.AttributeKeyCosmosCoin, pair.Denom),
			sdk.NewAttribute(types.AttributeKeyERC20Token, pair.Erc20Address),
		),
	)

	return &types.MsgToggleConversionResponse{}, nil
}

// validateAuthority is a helper function to validate that the provided authority
// matches the effective authority. It first checks the consensus AuthorityParams
// and falls back to the keeper's authority address if none is configured.
func (k *Keeper) validateAuthority(ctx sdk.Context, authority string) error {
	if _, err := k.addrCodec.StringToBytes(authority); err != nil {
		return errortypes.ErrInvalidAddress.Wrapf("invalid authority address: %s", err)
	}

	return sdk.ValidateAuthority(ctx, k.authority.String(), authority)
}
