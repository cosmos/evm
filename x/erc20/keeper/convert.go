package keeper

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/hashicorp/go-metrics"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/statedb"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

// ConvertERC20IntoCoinsForNativeToken handles the erc20 conversion for a native erc20 token pair.
// This function is used by both the msg server and precompiles (like ICS20).
// It performs the following operations:
//   - validates the token pair and checks if conversion is enabled
//   - removes the token pair if the contract is suicided
//   - escrows tokens on module account
//   - mints coins on bank module
//   - sends minted coins to the receiver
//   - checks if coin balance increased by amount
//   - checks if token balance decreased by amount
//   - checks for unexpected `Approval` event in logs
func (k Keeper) ConvertERC20IntoCoinsForNativeToken(ctx sdk.Context, stateDB *statedb.StateDB, contract common.Address, amount math.Int, receiver sdk.AccAddress, sender common.Address, commit bool, callFromPrecompile bool) (*types.MsgConvertERC20Response, error) {
	// Validate and get token pair
	pair, err := k.MintingEnabled(ctx, receiver, contract.Hex())
	if err != nil {
		return nil, err
	}

	// Check that this is a native ERC20 token
	if !pair.IsNativeERC20() {
		if pair.IsNativeCoin() {
			return nil, types.ErrNativeConversionDisabled
		}
		return nil, types.ErrUndefinedOwner
	}

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
	erc20 := contracts.ERC20MinterBurnerDecimalsContract.ABI
	erc20Contract := pair.GetERC20Contract()
	balanceCoin := k.bankKeeper.GetBalance(ctx, receiver, pair.Denom)
	balanceToken := k.BalanceOf(ctx, erc20, erc20Contract, types.ModuleAddress)
	if balanceToken == nil {
		return nil, errors.Wrap(types.ErrEVMCall, "failed to retrieve balance")
	}

	// Escrow tokens on module account
	transferData, err := erc20.Pack("transfer", types.ModuleAddress, amount.BigInt())
	if err != nil {
		return nil, err
	}

	res, err := k.evmKeeper.CallEVMWithData(ctx, stateDB, sender, &erc20Contract, transferData, commit, callFromPrecompile, nil)
	if err != nil {
		return nil, err
	}

	// Check evm call response
	var unpackedRet types.ERC20BoolResponse
	if len(res.Ret) == 0 {
		// if the token does not return a value, check for the transfer event in logs
		if err := validateTransferEventExists(res.Logs, erc20Contract); err != nil {
			return nil, err
		}
	} else {
		if err := erc20.UnpackIntoInterface(&unpackedRet, "transfer", res.Ret); err != nil {
			return nil, err
		}
		if !unpackedRet.Value {
			return nil, errors.Wrap(errortypes.ErrLogic, "failed to execute transfer")
		}
	}

	// Check expected escrow balance after transfer execution
	coins := sdk.Coins{sdk.Coin{Denom: pair.Denom, Amount: amount}}
	tokens := coins[0].Amount.BigInt()
	balanceTokenAfter := k.BalanceOf(ctx, erc20, erc20Contract, types.ModuleAddress)
	if balanceTokenAfter == nil {
		return nil, errors.Wrap(types.ErrEVMCall, "failed to retrieve balance")
	}

	expToken := big.NewInt(0).Add(balanceToken, tokens)

	if r := balanceTokenAfter.Cmp(expToken); r != 0 {
		return nil, errors.Wrapf(
			types.ErrBalanceInvariance,
			"invalid token balance - expected: %v, actual: %v",
			expToken, balanceTokenAfter,
		)
	}

	// Mint coins
	if err := k.bankKeeper.MintCoins(ctx, types.ModuleName, coins); err != nil {
		return nil, err
	}

	// Send minted coins to the receiver
	if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, receiver, coins); err != nil {
		return nil, err
	}

	// Check expected receiver balance after transfer
	balanceCoinAfter := k.bankKeeper.GetBalance(ctx, receiver, pair.Denom)
	expCoin := balanceCoin.Add(coins[0])

	if ok := balanceCoinAfter.Equal(expCoin); !ok {
		return nil, errors.Wrapf(
			types.ErrBalanceInvariance,
			"invalid coin balance - expected: %v, actual: %v",
			expCoin, balanceCoinAfter,
		)
	}

	defer func() {
		telemetry.IncrCounterWithLabels(
			[]string{"tx", "msg", "convert", "erc20", "total"},
			1,
			[]metrics.Label{
				telemetry.NewLabel("coin", pair.Denom),
			},
		)

		if amount.IsInt64() {
			telemetry.IncrCounterWithLabels(
				[]string{"tx", "msg", "convert", "erc20", "amount", "total"},
				float32(amount.Int64()),
				[]metrics.Label{
					telemetry.NewLabel("denom", pair.Denom),
				},
			)
		}
	}()

	ctx.EventManager().EmitEvents(
		sdk.Events{
			sdk.NewEvent(
				types.EventTypeConvertERC20,
				sdk.NewAttribute(sdk.AttributeKeySender, sender.Hex()),
				sdk.NewAttribute(types.AttributeKeyReceiver, receiver.String()),
				sdk.NewAttribute(sdk.AttributeKeyAmount, amount.String()),
				sdk.NewAttribute(types.AttributeKeyCosmosCoin, pair.Denom),
				sdk.NewAttribute(types.AttributeKeyERC20Token, contract.Hex()),
			),
		},
	)

	return &types.MsgConvertERC20Response{}, nil
}
