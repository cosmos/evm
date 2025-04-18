package keeper

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/x/erc20/types"
)

// GetAllowance returns the allowance of the given owner and spender
// on the given erc20 precompile address.
func (k Keeper) GetAllowance(
	ctx sdk.Context,
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
) (*big.Int, error) {
	if err := k.checkTokenPair(ctx, erc20); err != nil {
		return common.Big0, err
	}

	allowanceKey := types.AllowanceKey(erc20, owner, spender)
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixAllowance)

	var allowance types.Allowance
	bz := store.Get(allowanceKey)
	if bz == nil {
		return common.Big0, nil
	}

	k.cdc.MustUnmarshal(bz, &allowance)

	return allowance.Value.BigInt(), nil
}

// SetAllowance sets the allowance of the given owner and spender
// on the given erc20 precompile address.
func (k Keeper) SetAllowance(
	ctx sdk.Context,
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
	value *big.Int,
) error {
	return k.setAllowance(ctx, erc20, owner, spender, value)
}

// DeleteAllowance deletes the allowance of the given owner and spender
// on the given erc20 precompile address.
func (k Keeper) DeleteAllowance(
	ctx sdk.Context,
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
) error {
	return k.setAllowance(ctx, erc20, owner, spender, common.Big0)
}

func (k Keeper) setAllowance(
	ctx sdk.Context,
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
	value *big.Int,
) error {
	if err := k.checkAddressesNonZero(erc20, owner, spender); err != nil {
		return err
	}

	if err := k.checkTokenPair(ctx, erc20); err != nil {
		return err
	}

	allowanceKey := types.AllowanceKey(erc20, owner, spender)
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixAllowance)
	switch {
	case value == nil || value.Sign() == 0:
		store.Delete(allowanceKey)
	case value.Sign() < 0:
		return errorsmod.Wrapf(types.ErrInvalidAllowance, "value '%s' is less than zero", value)
	default:
		allowance := types.NewAllowance(erc20, owner, spender, value)
		bz := k.cdc.MustMarshal(&allowance)
		store.Set(allowanceKey, bz)
	}

	return nil
}

// GetAllowances returns all allowances stored on the given erc20 precompile address.
func (k Keeper) GetAllowances(
	ctx sdk.Context,
) []types.Allowance {
	allowances := []types.Allowance{}

	k.IterateAllowances(ctx, func(allowance types.Allowance) (stop bool) {
		allowances = append(allowances, allowance)
		return false
	})

	return allowances
}

// IterateAllowances iterates through all allowances stored on the given erc20 precompile address.
func (k Keeper) IterateAllowances(
	ctx sdk.Context,
	cb func(allowance types.Allowance) (stop bool),
) {
	store := ctx.KVStore(k.storeKey)
	iterator := storetypes.KVStorePrefixIterator(store, types.KeyPrefixAllowance)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		var allowance types.Allowance
		k.cdc.MustUnmarshal(iterator.Value(), &allowance)

		if cb(allowance) {
			break
		}
	}
}

func (k Keeper) checkAddressesNonZero(
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
) error {
	if erc20 == (common.Address{}) {
		return errorsmod.Wrapf(errortypes.ErrInvalidAddress, "invalid erc20 address: '%s'", erc20)
	}

	if owner == (common.Address{}) {
		return errorsmod.Wrapf(errortypes.ErrInvalidAddress, "invalid owner address: '%s'", owner)
	}

	if spender == (common.Address{}) {
		return errorsmod.Wrapf(errortypes.ErrInvalidAddress, "invalid spender address: '%s'", spender)
	}

	return nil
}

func (k Keeper) checkTokenPair(ctx sdk.Context, erc20 common.Address) error {
	tokenPairID := k.GetERC20Map(ctx, erc20)
	tokenPair, found := k.GetTokenPair(ctx, tokenPairID)
	if !found {
		return errorsmod.Wrapf(
			types.ErrTokenPairNotFound, "token pair for address '%s' not registered", erc20,
		)
	}

	if !tokenPair.Enabled {
		return errorsmod.Wrapf(
			types.ErrERC20TokenPairDisabled, "token pair for address '%s' is disabled", erc20,
		)
	}

	return nil
}
