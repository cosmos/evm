package testutil

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	app "github.com/cosmos/evm/example_chain"
	"github.com/cosmos/evm/testutil/constants"
	testutiltx "github.com/cosmos/evm/testutil/tx"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	teststaking "github.com/cosmos/cosmos-sdk/x/staking/testutil"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// PrepareAccountsForDelegationRewards prepares the test suite for testing to withdraw delegation rewards.
//
// Balance is the amount of tokens that will be left in the account after the setup is done.
// For each defined reward, a validator is created and tokens are allocated to it using the distribution keeper,
// such that the given amount of tokens is outstanding as a staking reward for the account.
//
// The setup is done in the following way:
//   - Fund the account with the given address with the given balance.
//   - If the given balance is zero, the account will be created with zero balance.
//
// For every reward defined in the rewards argument, the following steps are executed:
//   - Set up a validator with zero commission and delegate to it -> the account delegation will be 50% of the total delegation.
//   - Allocate rewards to the validator.
//
// The function returns the updated context along with a potential error.
func PrepareAccountsForDelegationRewards(t *testing.T, ctx sdk.Context, app *app.ExampleChain, addr sdk.AccAddress, balance math.Int, rewards ...math.Int) (sdk.Context, error) {
	// Calculate the necessary amount of tokens to fund the account in order for the desired residual balance to
	// be left after creating validators and delegating to them.
	totalRewards := math.ZeroInt()
	for _, reward := range rewards {
		totalRewards = totalRewards.Add(reward)
	}
	totalNeededBalance := balance.Add(totalRewards)

	if totalNeededBalance.IsZero() {
		app.AccountKeeper.SetAccount(ctx, app.AccountKeeper.NewAccountWithAddress(ctx, addr))
	} else {
		// Fund account with enough tokens to stake them
		err := FundAccountWithBaseDenom(ctx, app.BankKeeper, addr, totalNeededBalance.Int64())
		if err != nil {
			return sdk.Context{}, fmt.Errorf("failed to fund account: %s", err.Error())
		}
	}

	if totalRewards.IsZero() {
		return ctx, nil
	}

	// reset historical count in distribution keeper which is necessary
	// for the delegation rewards to be calculated correctly
	app.DistrKeeper.DeleteAllValidatorHistoricalRewards(ctx)

	// set distribution module account balance which pays out the rewards
	distrAcc := app.DistrKeeper.GetDistributionAccount(ctx)
	err := FundModuleAccount(ctx, app.BankKeeper, distrAcc.GetName(), sdk.NewCoins(sdk.NewCoin(constants.ExampleAttoDenom, totalRewards)))
	if err != nil {
		return sdk.Context{}, fmt.Errorf("failed to fund distribution module account: %s", err.Error())
	}
	app.AccountKeeper.SetModuleAccount(ctx, distrAcc)

	for _, reward := range rewards {
		if reward.IsZero() {
			continue
		}

		// Set up validator and delegate to it
		privKey := ed25519.GenPrivKey()
		addr2, _ := testutiltx.NewAccAddressAndKey()
		err := FundAccountWithBaseDenom(ctx, app.BankKeeper, addr2, reward.Int64())
		if err != nil {
			return sdk.Context{}, fmt.Errorf("failed to fund validator account: %s", err.Error())
		}

		zeroDec := math.LegacyZeroDec()
		stakingParams, err := app.StakingKeeper.GetParams(ctx)
		if err != nil {
			return sdk.Context{}, fmt.Errorf("failed to get staking params: %s", err.Error())
		}
		stakingParams.BondDenom = constants.ExampleAttoDenom
		stakingParams.MinCommissionRate = zeroDec
		err = app.StakingKeeper.SetParams(ctx, stakingParams)
		require.NoError(t, err)

		stakingHelper := teststaking.NewHelper(t, ctx, app.StakingKeeper)
		stakingHelper.Commission = stakingtypes.NewCommissionRates(zeroDec, zeroDec, zeroDec)
		stakingHelper.Denom = constants.ExampleAttoDenom

		valAddr := sdk.ValAddress(addr2.Bytes())
		// self-delegate the same amount of tokens as the delegate address also stakes
		// this ensures, that the delegation rewards are 50% of the total rewards
		stakingHelper.CreateValidator(valAddr, privKey.PubKey(), reward, true)
		stakingHelper.Delegate(addr, valAddr, reward)

		// end block to bond validator and increase block height
		// Not using Commit() here because code panics due to invalid block height
		_, err = app.StakingKeeper.EndBlocker(ctx)
		require.NoError(t, err)

		// allocate rewards to validator (of these 50% will be paid out to the delegator)
		validator, err := app.StakingKeeper.Validator(ctx, valAddr)
		if err != nil {
			return sdk.Context{}, fmt.Errorf("failed to get validator: %s", err.Error())
		}
		allocatedRewards := sdk.NewDecCoins(sdk.NewDecCoin(constants.ExampleAttoDenom, reward.Mul(math.NewInt(2))))
		if err = app.DistrKeeper.AllocateTokensToValidator(ctx, validator, allocatedRewards); err != nil {
			return sdk.Context{}, fmt.Errorf("failed to allocate tokens to validator: %s", err.Error())
		}
	}

	// Increase block height in ctx for the rewards calculation
	// NOTE: this will only work for unit tests that use the context
	// returned by this function
	currentHeight := ctx.BlockHeight()
	return ctx.WithBlockHeight(currentHeight + 1), nil
}
