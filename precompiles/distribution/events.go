package distribution

import (
	"bytes"
	"fmt"
	"reflect"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

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
	// Prepare the event topics
	event := p.Events[EventTypeClaimRewards]
	topics := make([]common.Hash, 2)

	// The first topic is always the signature of the event.
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(delegatorAddress)
	if err != nil {
		return err
	}

	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return err
	}
	totalAmount := totalCoins.AmountOf(bondDenom)

	// Pack the arguments to be used as the Data field
	arguments := abi.Arguments{event.Inputs[1]}
	packed, err := arguments.Pack(totalAmount.BigInt())
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        packed,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
	})

	return nil
}

// EmitSetWithdrawAddressEvent creates a new event emitted on a SetWithdrawAddressMethod transaction.
func (p Precompile) EmitSetWithdrawAddressEvent(ctx sdk.Context, stateDB vm.StateDB, caller common.Address, withdrawerAddress string) error {
	// Prepare the event topics
	event := p.Events[EventTypeSetWithdrawAddress]
	topics := make([]common.Hash, 2)

	// The first topic is always the signature of the event.
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(caller)
	if err != nil {
		return err
	}

	// Pack the arguments to be used as the Data field
	arguments := abi.Arguments{event.Inputs[1]}
	packed, err := arguments.Pack(withdrawerAddress)
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        packed,
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

	// Prepare the event topics
	event := p.Events[EventTypeWithdrawDelegatorReward]
	topics := make([]common.Hash, 3)

	// The first topic is always the signature of the event.
	topics[0] = event.ID

	topics[1], err = cmn.MakeTopic(delegatorAddress)
	if err != nil {
		return err
	}

	topics[2], err = cmn.MakeTopic(common.BytesToAddress(valAddr.Bytes()))
	if err != nil {
		return err
	}

	// Prepare the event data
	var b bytes.Buffer
	b.Write(cmn.PackNum(reflect.ValueOf(coins[0].Amount.BigInt())))

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        b.Bytes(),
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
	})

	return nil
}

// EmitWithdrawValidatorCommissionEvent creates a new event emitted on a WithdrawValidatorCommission transaction.
func (p Precompile) EmitWithdrawValidatorCommissionEvent(ctx sdk.Context, stateDB vm.StateDB, validatorAddress string, coins sdk.Coins) error {
	// Prepare the event topics
	event := p.Events[EventTypeWithdrawValidatorCommission]
	topics := make([]common.Hash, 2)

	// The first topic is always the signature of the event.
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(validatorAddress)
	if err != nil {
		return err
	}

	// Prepare the event data
	var b bytes.Buffer
	b.Write(cmn.PackNum(reflect.ValueOf(coins[0].Amount.BigInt())))

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        b.Bytes(),
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
	})

	return nil
}

// EmitFundCommunityPoolEvent creates a new event emitted per Coin on a FundCommunityPool transaction.
func (p Precompile) EmitFundCommunityPoolEvent(ctx sdk.Context, stateDB vm.StateDB, depositor common.Address, coins sdk.Coins) error {
	// Prepare the event topics
	event := p.Events[EventTypeFundCommunityPool]

	for _, coin := range coins {
		topics := make([]common.Hash, 2)

		// The first topic is always the signature of the event.
		topics[0] = event.ID

		// Second topic: depositor address
		var err error
		topics[1], err = cmn.MakeTopic(depositor)
		if err != nil {
			return err
		}

		// Encode denom and amount as event data
		// Assuming FundCommunityPool(address,string,uint256)
		data, err := event.Inputs.NonIndexed().Pack(coin.Denom, coin.Amount.BigInt())
		if err != nil {
			return fmt.Errorf("failed to pack event data: %w", err)
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

	// Prepare the event topics
	event := p.Events[EventTypeDepositValidatorRewardsPool]
	for _, coin := range coins {
		topics := make([]common.Hash, 3)

		// The first topic is always the signature of the event.
		topics[0] = event.ID

		// The second topic is depositor address.
		var err error
		topics[1], err = cmn.MakeTopic(depositor)
		if err != nil {
			return err
		}

		// The third topic is validator address.
		topics[2], err = cmn.MakeTopic(common.BytesToAddress(valAddr.Bytes()))
		if err != nil {
			return err
		}

		// Encode denom and amount as event data assuming the event type is
		// DepositValidatorRewardsPool(address, address, string, uint256)
		data, err := event.Inputs.NonIndexed().Pack(coin.Denom, coin.Amount.BigInt())
		if err != nil {
			return fmt.Errorf("failed to pack event data: %w", err)
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
