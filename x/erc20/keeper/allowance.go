package keeper

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/x/erc20/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

// GetAllowance returns the allowance of the given owner and spender
// on the given erc20 precompile address.
func (k Keeper) GetAllowance(
	ctx sdk.Context,
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
) (*big.Int, error) {
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
	return k.setAllowance(ctx, erc20, owner, spender, value, false)
}

// DeleteAllowance deletes the allowance of the given owner and spender
// on the given erc20 precompile address.
func (k Keeper) DeleteAllowance(
	ctx sdk.Context,
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
) error {
	return k.setAllowance(ctx, erc20, owner, spender, common.Big0, false)
}

// UnsafeSetAllowance sets the allowance of the given owner and spender with validation.
// It allows setting allowance for disabled token pairs.
// This should only be used in InitGenesis.
func (k Keeper) UnsafeSetAllowance(
	ctx sdk.Context,
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
	value *big.Int,
) error {
	return k.setAllowance(ctx, erc20, owner, spender, value, true)
}

func (k Keeper) setAllowance(
	ctx sdk.Context,
	erc20 common.Address,
	owner common.Address,
	spender common.Address,
	value *big.Int,
	allowDisabledTokenPair bool,
) error {
	// validate existence of token pair
	tokenPairID := k.GetERC20Map(ctx, erc20)
	tokenPair, found := k.GetTokenPair(ctx, tokenPairID)
	if !found {
		return errorsmod.Wrapf(
			types.ErrTokenPairNotFound, "token pair for address '%s' not registered", erc20,
		)
	}
	if !allowDisabledTokenPair && !tokenPair.Enabled {
		return errorsmod.Wrapf(
			types.ErrERC20TokenPairDisabled, "token pair for address '%s' is disabled", erc20,
		)
	}

	// validate addresses
	if (owner == common.Address{}) {
		return errorsmod.Wrapf(errortypes.ErrInvalidAddress, "owner address is empty")
	}
	if (spender == common.Address{}) {
		return errorsmod.Wrapf(errortypes.ErrInvalidAddress, "spender address is empty")
	}

	// validate value
	if value == nil {
		return errorsmod.Wrapf(types.ErrInvalidAllowance, "value is nil")
	}
	if value.Sign() < 0 {
		return errorsmod.Wrapf(types.ErrInvalidAllowance, "value '%s' is less than zero", value)
	}
	if value.BitLen() > 256 {
		return errorsmod.Wrapf(types.ErrInvalidAllowance, "value '%s' is greater than max value of uint256", value)
	}

	// store or delete allowance
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixAllowance)
	allowanceKey := types.AllowanceKey(erc20, owner, spender)

	if value.Sign() == 0 {
		store.Delete(allowanceKey)
	} else {
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

func (k Keeper) deleteAllowances(ctx sdk.Context, erc20 common.Address) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), types.KeyPrefixAllowance)
	iterator := storetypes.KVStorePrefixIterator(store, erc20.Bytes())
	defer iterator.Close()

	var keys [][]byte

	for ; iterator.Valid(); iterator.Next() {
		keys = append(keys, iterator.Key())
	}

	for _, key := range keys {
		store.Delete(key)
	}
}
