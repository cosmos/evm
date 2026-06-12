package gov

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
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
	// Prepare the event topics
	event := p.Events[EventTypeSubmitProposal]
	topics := make([]common.Hash, 2)

	// The first topic is always the signature of the event.
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(proposerAddress)
	if err != nil {
		return err
	}

	// Prepare the event data
	arguments := abi.Arguments{event.Inputs[1]}
	packed, err := arguments.Pack(proposalID)
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        packed,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}

// EmitCancelProposalEvent creates a new event emitted on a CancelProposal transaction.
func (p Precompile) EmitCancelProposalEvent(ctx sdk.Context, stateDB vm.StateDB, proposerAddress common.Address, proposalID uint64) error {
	// Prepare the event topics
	event := p.Events[EventTypeCancelProposal]
	topics := make([]common.Hash, 2)

	// The first topic is always the signature of the event.
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(proposerAddress)
	if err != nil {
		return err
	}

	// Prepare the event data
	arguments := abi.Arguments{event.Inputs[1]}
	packed, err := arguments.Pack(proposalID)
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        packed,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}

// EmitDepositEvent creates a new event emitted on a Deposit transaction.
func (p Precompile) EmitDepositEvent(ctx sdk.Context, stateDB vm.StateDB, depositorAddress common.Address, proposalID uint64, amount []sdk.Coin) error {
	// Prepare the event topics
	event := p.Events[EventTypeDeposit]
	topics := make([]common.Hash, 2)

	// The first topic is always the signature of the event.
	topics[0] = event.ID
	var err error
	topics[1], err = cmn.MakeTopic(depositorAddress)
	if err != nil {
		return err
	}

	// Prepare the event data
	arguments := abi.Arguments{event.Inputs[1], event.Inputs[2]}
	packed, err := arguments.Pack(proposalID, cmn.NewCoinsResponse(amount))
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        packed,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}

// EmitVoteEvent creates a new event emitted on a Vote transaction.
func (p Precompile) EmitVoteEvent(ctx sdk.Context, stateDB vm.StateDB, voterAddress common.Address, proposalID uint64, option int32) error {
	// Prepare the event topics
	event := p.Events[EventTypeVote]
	topics := make([]common.Hash, 2)

	// The first topic is always the signature of the event.
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(voterAddress)
	if err != nil {
		return err
	}

	// Prepare the event data
	arguments := abi.Arguments{event.Inputs[1], event.Inputs[2]}
	packed, err := arguments.Pack(proposalID, uint8(option)) //nolint:gosec // G115
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        packed,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}

// EmitVoteWeightedEvent creates a new event emitted on a VoteWeighted transaction.
func (p Precompile) EmitVoteWeightedEvent(ctx sdk.Context, stateDB vm.StateDB, voterAddress common.Address, proposalID uint64, options WeightedVoteOptions) error {
	// Prepare the event topics
	event := p.Events[EventTypeVoteWeighted]
	topics := make([]common.Hash, 2)

	// The first topic is always the signature of the event.
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(voterAddress)
	if err != nil {
		return err
	}

	// Prepare the event data
	arguments := abi.Arguments{event.Inputs[1], event.Inputs[2]}
	packed, err := arguments.Pack(proposalID, options)
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        packed,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}
