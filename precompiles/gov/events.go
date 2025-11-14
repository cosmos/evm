package gov

import (
	"errors"
	"math"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/yihuang/go-abi"

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
	// Create the event using the generated constructor
	event := NewSubmitProposalEvent(proposerAddress, proposalID)

	// Prepare the event topics and data
	topics, data, err := abi.EncodeEvent(event)
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

	// Prepare the event topics and data
	topics, data, err := abi.EncodeEvent(event)
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

	// Prepare the event topics and data
	topics, data, err := abi.EncodeEvent(event)
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
	if option > math.MaxUint8 {
		return errors.New("vote option exceeds uint8 limit")
	}

	// Create the event using the generated constructor
	event := NewVoteEvent(voterAddress, proposalID, uint8(option)) //nolint:gosec

	// Prepare the event topics data
	topics, data, err := abi.EncodeEvent(event)
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

	// Prepare the event topics and data
	topics, data, err := abi.EncodeEvent(event)
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
