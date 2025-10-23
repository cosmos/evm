package staking

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

const (
	// EventTypeCreateValidator defines the event type for the staking CreateValidator transaction.
	EventTypeCreateValidator = "CreateValidator"
	// EventTypeEditValidator defines the event type for the staking EditValidator transaction.
	EventTypeEditValidator = "EditValidator"
	// EventTypeDelegate defines the event type for the staking Delegate transaction.
	EventTypeDelegate = "Delegate"
	// EventTypeUnbond defines the event type for the staking Undelegate transaction.
	EventTypeUnbond = "Unbond"
	// EventTypeRedelegate defines the event type for the staking Redelegate transaction.
	EventTypeRedelegate = "Redelegate"
	// EventTypeCancelUnbondingDelegation defines the event type for the staking CancelUnbondingDelegation transaction.
	EventTypeCancelUnbondingDelegation = "CancelUnbondingDelegation"
)

// EmitCreateValidatorEvent creates a new create validator event emitted on a CreateValidator transaction.
func (p Precompile) EmitCreateValidatorEvent(ctx sdk.Context, stateDB vm.StateDB, msg *stakingtypes.MsgCreateValidator, validatorAddr common.Address) error {
	// Prepare the event topics
	event := NewCreateValidatorEvent(validatorAddr, msg.Value.Amount.BigInt())
	topics, err := event.EncodeTopics()
	if err != nil {
		return err
	}

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

// EmitEditValidatorEvent creates a new edit validator event emitted on a EditValidator transaction.
func (p Precompile) EmitEditValidatorEvent(ctx sdk.Context, stateDB vm.StateDB, msg *stakingtypes.MsgEditValidator, validatorAddr common.Address) error {
	commissionRate := big.NewInt(DoNotModifyCommissionRate)
	if msg.CommissionRate != nil {
		commissionRate = msg.CommissionRate.BigInt()
	}

	minSelfDelegation := big.NewInt(DoNotModifyMinSelfDelegation)
	if msg.MinSelfDelegation != nil {
		minSelfDelegation = msg.MinSelfDelegation.BigInt()
	}

	event := NewEditValidatorEvent(validatorAddr, commissionRate, minSelfDelegation)
	topics, err := event.EncodeTopics()
	if err != nil {
		return err
	}

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

// EmitDelegateEvent creates a new delegate event emitted on a Delegate transaction.
func (p Precompile) EmitDelegateEvent(ctx sdk.Context, stateDB vm.StateDB, msg *stakingtypes.MsgDelegate, delegatorAddr common.Address) error {
	valAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return err
	}

	// Get the validator to estimate the new shares delegated
	// NOTE: At this point the validator has already been checked, so no need to check again
	validator, _ := p.stakingKeeper.GetValidator(ctx, valAddr)

	// Get only the new shares based on the delegation amount
	newShares, err := validator.SharesFromTokens(msg.Amount.Amount)
	if err != nil {
		return err
	}

	// Prepare the event topics
	event := NewDelegateEvent(
		delegatorAddr,
		common.BytesToAddress(valAddr),
		msg.Amount.Amount.BigInt(),
		newShares.BigInt(),
	)
	topics, err := event.EncodeTopics()
	if err != nil {
		return err
	}

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

// EmitUnbondEvent creates a new unbond event emitted on an Undelegate transaction.
func (p Precompile) EmitUnbondEvent(ctx sdk.Context, stateDB vm.StateDB, msg *stakingtypes.MsgUndelegate, delegatorAddr common.Address, completionTime int64) error {
	valAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return err
	}

	// Prepare the event topics
	event := NewUnbondEvent(
		delegatorAddr,
		common.BytesToAddress(valAddr),
		msg.Amount.Amount.BigInt(),
		big.NewInt(completionTime),
	)
	topics, err := event.EncodeTopics()
	if err != nil {
		return err
	}

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

// EmitRedelegateEvent creates a new redelegate event emitted on a Redelegate transaction.
func (p Precompile) EmitRedelegateEvent(ctx sdk.Context, stateDB vm.StateDB, msg *stakingtypes.MsgBeginRedelegate, delegatorAddr common.Address, completionTime int64) error {
	valSrcAddr, err := sdk.ValAddressFromBech32(msg.ValidatorSrcAddress)
	if err != nil {
		return err
	}

	valDstAddr, err := sdk.ValAddressFromBech32(msg.ValidatorDstAddress)
	if err != nil {
		return err
	}

	// Prepare the event topics
	event := RedelegateEventIndexed{
		DelegatorAddress:    delegatorAddr,
		ValidatorSrcAddress: common.BytesToAddress(valSrcAddr),
		ValidatorDstAddress: common.BytesToAddress(valDstAddr),
	}
	topics, err := event.EncodeTopics()
	if err != nil {
		return err
	}

	data := RedelegateEventData{
		Amount:         msg.Amount.Amount.BigInt(),
		CompletionTime: big.NewInt(completionTime),
	}
	bz, err := data.Encode()
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        bz,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
	})

	return nil
}

// EmitCancelUnbondingDelegationEvent creates a new cancel unbonding delegation event emitted on a CancelUnbondingDelegation transaction.
func (p Precompile) EmitCancelUnbondingDelegationEvent(ctx sdk.Context, stateDB vm.StateDB, msg *stakingtypes.MsgCancelUnbondingDelegation, delegatorAddr common.Address) error {
	valAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		return err
	}

	// Prepare the event topics
	event := CancelUnbondingDelegationEventIndexed{
		DelegatorAddress: delegatorAddr,
		ValidatorAddress: common.BytesToAddress(valAddr),
	}
	topics, err := event.EncodeTopics()
	if err != nil {
		return err
	}

	data := CancelUnbondingDelegationEventData{
		Amount:         msg.Amount.Amount.BigInt(),
		CreationHeight: big.NewInt(int64(msg.CreationHeight)),
	}
	bz, err := data.Encode()
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        bz,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
	})

	return nil
}
