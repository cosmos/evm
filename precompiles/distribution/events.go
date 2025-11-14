package distribution

import (
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/yihuang/go-abi"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// EventTypeSetWithdrawAddress defines the event type for the distribution SetWithdrawAddressMethod transaction.
	EventTypeSetWithdrawAddress = "SetWithdrawerAddress"
	// EventTypeWithdrawDelegatorReward defines the event type for the distribution WithdrawDelegatorRewardMethod transaction.
	EventTypeWithdrawDelegatorReward = "WithdrawDelegatorReward"
	// EventTypeWithdrawValidatorCommission defines the event type for the distribution WithdrawValidatorCommissionMethod transaction.
	EventTypeWithdrawValidatorCommission = "WithdrawValidatorCommission"
	// EventTypeFundCommunityPool defines the event type for the distribution FundCommunityPoolMethod transaction.
	EventTypeFundCommunityPool = "FundCommunityPool"
	// EventTypeClaimRewards defines the event type for the distribution ClaimRewardsMethod transaction.
	EventTypeClaimRewards = "ClaimRewards"
	// EventTypeDepositValidatorRewardsPool defines the event type for the distribution DepositValidatorRewardsPoolMethod transaction.
	EventTypeDepositValidatorRewardsPool = "DepositValidatorRewardsPool"
)

// EmitClaimRewardsEvent creates a new event emitted on a ClaimRewards transaction.
func (p Precompile) EmitClaimRewardsEvent(ctx sdk.Context, stateDB vm.StateDB, delegatorAddress common.Address, totalCoins sdk.Coins) error {
	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return err
	}
	totalAmount := totalCoins.AmountOf(bondDenom)

	// Prepare the event
	event := NewClaimRewardsEvent(
		delegatorAddress,
		totalAmount.BigInt(),
	)
	topics, err := event.EncodeTopics()
	if err != nil {
		return err
	}

	// Prepare the event data
	data, err := event.Encode()
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
	})

	return nil
}

// EmitSetWithdrawAddressEvent creates a new event emitted on a SetWithdrawAddressMethod transaction.
func (p Precompile) EmitSetWithdrawAddressEvent(ctx sdk.Context, stateDB vm.StateDB, caller common.Address, withdrawerAddress string) error {
	// Prepare the event
	event := NewSetWithdrawerAddressEvent(
		caller,
		withdrawerAddress,
	)
	topics, err := event.EncodeTopics()
	if err != nil {
		return err
	}

	// Prepare the event data
	data, err := event.Encode()
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
	})

	return nil
}

// EmitWithdrawDelegatorRewardEvent creates a new event emitted on a WithdrawDelegatorReward transaction.
func (p Precompile) EmitWithdrawDelegatorRewardEvent(ctx sdk.Context, stateDB vm.StateDB, delegatorAddress common.Address, validatorAddress string, coins sdk.Coins) error {
	valAddr, err := sdk.ValAddressFromBech32(validatorAddress)
	if err != nil {
		return err
	}

	// Prepare the event
	event := NewWithdrawDelegatorRewardEvent(
		delegatorAddress,
		common.BytesToAddress(valAddr.Bytes()),
		coins[0].Amount.BigInt(),
	)
	topics, err := event.EncodeTopics()
	if err != nil {
		return err
	}

	// Prepare the event data
	data, err := event.Encode()
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
	})

	return nil
}

// EmitWithdrawValidatorCommissionEvent creates a new event emitted on a WithdrawValidatorCommission transaction.
func (p Precompile) EmitWithdrawValidatorCommissionEvent(ctx sdk.Context, stateDB vm.StateDB, validatorAddress string, coins sdk.Coins) error {
	// Prepare the event
	event := NewWithdrawValidatorCommissionEvent(
		validatorAddress,
		coins[0].Amount.BigInt(),
	)
	topics, data, err := abi.EncodeEvent(event)
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
	})

	return nil
}

// EmitFundCommunityPoolEvent creates a new event emitted per Coin on a FundCommunityPool transaction.
func (p Precompile) EmitFundCommunityPoolEvent(ctx sdk.Context, stateDB vm.StateDB, depositor common.Address, coins sdk.Coins) error {
	for _, coin := range coins {
		// Prepare the event
		event := NewFundCommunityPoolEvent(
			depositor,
			coin.Denom,
			coin.Amount.BigInt(),
		)
		topics, err := event.EncodeTopics()
		if err != nil {
			return err
		}

		// Prepare the event data
		data, err := event.Encode()
		if err != nil {
			return err
		}

		// Emit log for each coin
		stateDB.AddLog(&ethtypes.Log{
			Address:     p.Address(),
			Topics:      topics,
			Data:        data,
			BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
		})
	}

	return nil
}

// EmitDepositValidatorRewardsPoolEvent creates a new event emitted on a DepositValidatorRewardsPool transaction.
func (p Precompile) EmitDepositValidatorRewardsPoolEvent(ctx sdk.Context, stateDB vm.StateDB, depositor common.Address, validatorAddress string, coins sdk.Coins) error {
	valAddr, err := sdk.ValAddressFromBech32(validatorAddress)
	if err != nil {
		return err
	}

	for _, coin := range coins {
		// Prepare the event
		event := NewDepositValidatorRewardsPoolEvent(
			depositor,
			common.BytesToAddress(valAddr.Bytes()),
			coin.Denom,
			coin.Amount.BigInt(),
		)
		topics, err := event.EncodeTopics()
		if err != nil {
			return err
		}

		// Prepare the event data
		data, err := event.Encode()
		if err != nil {
			return err
		}

		// Emit log for each coin
		stateDB.AddLog(&ethtypes.Log{
			Address:     p.Address(),
			Topics:      topics,
			Data:        data,
			BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
		})
	}

	return nil
}
