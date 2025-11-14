package erc20

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EmitTransferEvent creates a new Transfer event emitted on transfer and transferFrom transactions.
func (p Precompile) EmitTransferEvent(ctx sdk.Context, stateDB vm.StateDB, from, to common.Address, value *big.Int) error {
	// Create the event using the generated constructor
	event := NewTransferEvent(from, to, value)

	// Prepare the event topics
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
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // block height won't exceed uint64
	})

	return nil
}

// EmitApprovalEvent creates a new approval event emitted on Approve transactions.
func (p Precompile) EmitApprovalEvent(ctx sdk.Context, stateDB vm.StateDB, owner, spender common.Address, value *big.Int) error {
	// Create the event using the generated constructor
	event := NewApprovalEvent(owner, spender, value)

	// Prepare the event topics
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
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // block height won't exceed uint64
	})

	return nil
}
