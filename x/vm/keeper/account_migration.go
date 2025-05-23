package keeper

import (
	"cosmossdk.io/math"
	"cosmossdk.io/x/feegrant"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	vesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/exported"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// ===============
// === Staking ===
// ===============
func (k *Keeper) migrateDelegations(ctx sdk.Context, originalAddress sdk.AccAddress, maxValidators uint16, newAddress sdk.AccAddress) error {
	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return err
	}

	// Check if the account is a vesting account
	acc := k.accountKeeper.GetAccount(ctx, originalAddress)

	// remaining vesting delegation
	var remainingVestingDelegation int64 = 0
	var isVestingAccount = false

	if acc != nil {
		acc, ok := acc.(vesting.VestingAccount)
		if ok {
			isVestingAccount = true
			remainingVestingDelegation = acc.GetDelegatedFree().AmountOf(bondDenom).Int64()
		}
	}

	delegations, err := k.stakingKeeper.GetDelegatorDelegations(ctx, originalAddress, maxValidators)

	for _, delegation := range delegations {
		validatorAddress, err := sdk.ValAddressFromBech32(delegation.ValidatorAddress)
		if err != nil {
			return err
		}

		// Seems like we don't need to go through the whole unbond -> send to user -> send to new address -> bond
		// and that we can remove / reset the delegation instead. Tokens aren't moving anywhere anyways
		sharesToUnbond := delegation.GetShares()

		if isVestingAccount {
			// if there are no unlocked tokens remaining, we return from the function and terminate delegation migration
			if remainingVestingDelegation <= 0 {
				return nil
			}

			if remainingVestingDelegation < sharesToUnbond.TruncateInt64() {
				sharesToUnbond = math.LegacyNewDec(remainingVestingDelegation)
				remainingVestingDelegation = 0
			}
		}

		_, err = k.stakingKeeper.Unbond(ctx, originalAddress, validatorAddress, sharesToUnbond)
		if err != nil {
			return err
		}

		err = k.stakingKeeper.SetDelegation(ctx, stakingtypes.Delegation{
			DelegatorAddress: newAddress.String(),
			ValidatorAddress: delegation.ValidatorAddress,
			Shares:           sharesToUnbond,
		})
		if err != nil {
			return err
		}

	}
	return nil
}

// ============
// === Bank ===
// ============
func (k *Keeper) migrateBankTokens(err error, ctx sdk.Context, originalAddress sdk.AccAddress, maxTokens uint64, newAddress sdk.AccAddress) error {
	balancesResponse, err := k.bankWrapper.SpendableBalances(ctx, &banktypes.QuerySpendableBalancesRequest{
		Address: originalAddress.String(),
		Pagination: &query.PageRequest{
			Key:        nil,
			Offset:     0,
			Limit:      maxTokens,
			CountTotal: false,
			Reverse:    false,
		},
	})
	if err != nil {
		return err
	}

	balances := balancesResponse.Balances

	err = k.bankWrapper.SendCoins(ctx, originalAddress, newAddress, balances)
	if err != nil {
		return err
	}
	return nil
}

// ================
// === Feegrant ===
// ================
func (k *Keeper) migrateFeeGrants(ctx sdk.Context, originalAddress sdk.AccAddress, maxFeeGrants uint64, newAddress sdk.AccAddress) error {
	allowancesResponse, err := k.feegrantKeeper.AllowancesByGranter(ctx, &feegrant.QueryAllowancesByGranterRequest{
		Granter: originalAddress.String(),
		Pagination: &query.PageRequest{
			Key:        nil,
			Offset:     0,
			Limit:      maxFeeGrants,
			CountTotal: false,
			Reverse:    false,
		},
	})
	if err != nil {
		return err
	}

	allowances := allowancesResponse.Allowances

	for _, grant := range allowances {
		//todo: when keeper call is introduced for revoking, replace this call with it: https://github.com/cosmos/cosmos-sdk/issues/24773
		msgRevoke := feegrant.MsgRevokeAllowance{
			Granter: grant.Granter,
			Grantee: grant.Grantee,
		}
		k.Router().Handler(&msgRevoke)

		granteeAddress, err := sdk.AccAddressFromBech32(grant.Grantee)
		if err != nil {
			return err
		}

		var allowance feegrant.FeeAllowanceI

		// todo: test this
		err = k.cdc.UnpackAny(grant.Allowance, &allowance)
		if err != nil {
			return fmt.Errorf("unknown message type: %s", grant.Allowance.TypeUrl)
		}

		err = k.feegrantKeeper.GrantAllowance(ctx, newAddress, granteeAddress, allowance)
		if err != nil {
			return err
		}
	}
	return nil
}

// =============
// === Authz ===
// =============
func (k *Keeper) migrateAuthzGrants(ctx sdk.Context, originalAddress sdk.AccAddress, maxAuthzGrants uint64) error {
	authzGrantsResponse, err := k.authzKeeper.GranterGrants(ctx, &authz.QueryGranterGrantsRequest{
		Granter: originalAddress.String(),
		Pagination: &query.PageRequest{
			Key:        nil,
			Offset:     0,
			Limit:      maxAuthzGrants,
			CountTotal: false,
			Reverse:    false,
		},
	})
	if err != nil {
		return err
	}

	authzGrants := authzGrantsResponse.Grants

	for _, grant := range authzGrants {
		granteeAddress, err := sdk.AccAddressFromBech32(grant.Granter)
		if err != nil {
			return err
		}

		granterAddress, err := sdk.AccAddressFromBech32(grant.Granter)
		if err != nil {
			return err
		}

		// todo: is the message type string the same as the type url?
		err = k.authzKeeper.DeleteGrant(ctx, granteeAddress, granterAddress, grant.Authorization.TypeUrl)
		if err != nil {
			return err
		}

		var authorization authz.Authorization

		// todo: test this
		err = k.cdc.UnpackAny(grant.Authorization, &authorization)
		if err != nil {
			return fmt.Errorf("unknown message type: %s", grant.Authorization.TypeUrl)
		}

		err = k.authzKeeper.SaveGrant(ctx, granteeAddress, granterAddress, authorization, grant.Expiration)
		if err != nil {
			return err
		}
	}
	return nil
}
