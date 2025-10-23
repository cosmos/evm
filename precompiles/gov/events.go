package gov

import (
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
	cmn "github.com/cosmos/evm/precompiles/common"
)

const (
	// EventTypeVote defines the event type for the gov VoteMethod transaction.
	EventTypeVote = "Vote"
	// EventTypeVoteWeighted defines the event type for the gov VoteWeightedMethod transaction.
	EventTypeVoteWeighted = "VoteWeighted"
	// EventTypeSubmitProposal defines the event type for the gov SubmitProposalMethod transaction.
	EventTypeSubmitProposal = "SubmitProposal"
	// EventTypeCanclelProposal defines the event type for the gov CancelProposalMethod transaction.
	EventTypeCancelProposal = "CancelProposal"
	// EventTypeDeposit defines the event type for the gov DepositMethod transaction.
	EventTypeDeposit = "Deposit"
)

// EmitSubmitProposalEvent creates a new event emitted on a SubmitProposal transaction.
func (p Precompile) EmitSubmitProposalEvent(ctx sdk.Context, stateDB vm.StateDB, proposerAddress common.Address, proposalID uint64) error {
	// Create the event using the generated constructor
	event := NewSubmitProposalEvent(proposerAddress, proposalID)

	// Prepare the event topics
	topics, err := event.SubmitProposalEventIndexed.EncodeTopics()
	if err != nil {
		return err
	}

	// Prepare the event data
	data, err := event.SubmitProposalEventData.Encode()
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}

// EmitCancelProposalEvent creates a new event emitted on a CancelProposal transaction.
func (p Precompile) EmitCancelProposalEvent(ctx sdk.Context, stateDB vm.StateDB, proposerAddress common.Address, proposalID uint64) error {
	// Create the event using the generated constructor
	event := NewCancelProposalEvent(proposerAddress, proposalID)

	// Prepare the event topics
	topics, err := event.CancelProposalEventIndexed.EncodeTopics()
	if err != nil {
		return err
	}

	// Prepare the event data
	data, err := event.CancelProposalEventData.Encode()
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}

// EmitDepositEvent creates a new event emitted on a Deposit transaction.
func (p Precompile) EmitDepositEvent(ctx sdk.Context, stateDB vm.StateDB, depositorAddress common.Address, proposalID uint64, amount []sdk.Coin) error {
	// Create the event using the generated constructor
	event := NewDepositEvent(depositorAddress, proposalID, cmn.NewCoinsResponse(amount))

	// Prepare the event topics
	topics, err := event.DepositEventIndexed.EncodeTopics()
	if err != nil {
		return err
	}

	// Prepare the event data
	data, err := event.DepositEventData.Encode()
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}

// EmitVoteEvent creates a new event emitted on a Vote transaction.
func (p Precompile) EmitVoteEvent(ctx sdk.Context, stateDB vm.StateDB, voterAddress common.Address, proposalID uint64, option int32) error {
	// Create the event using the generated constructor
	event := NewVoteEvent(voterAddress, proposalID, uint8(option))

	// Prepare the event topics
	topics, err := event.VoteEventIndexed.EncodeTopics()
	if err != nil {
		return err
	}

	// Prepare the event data
	data, err := event.VoteEventData.Encode()
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}

// EmitVoteWeightedEvent creates a new event emitted on a VoteWeighted transaction.
func (p Precompile) EmitVoteWeightedEvent(ctx sdk.Context, stateDB vm.StateDB, voterAddress common.Address, proposalID uint64, options WeightedVoteOptions) error {
	// Create the event using the generated constructor
	event := NewVoteWeightedEvent(voterAddress, proposalID, options)

	// Prepare the event topics
	topics, err := event.VoteWeightedEventIndexed.EncodeTopics()
	if err != nil {
		return err
	}

	// Prepare the event data
	data, err := event.VoteWeightedEventData.Encode()
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}
