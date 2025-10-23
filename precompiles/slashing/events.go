package slashing

import (
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EmitValidatorUnjailedEvent emits the ValidatorUnjailed event
func (p Precompile) EmitValidatorUnjailedEvent(ctx sdk.Context, stateDB vm.StateDB, validator common.Address) error {
	// Create the event using the generated constructor
	event := NewValidatorUnjailedEvent(validator)

	// Prepare the event topics
	topics, err := event.ValidatorUnjailedEventIndexed.EncodeTopics()
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        []byte{},                  // No data fields for this event
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}
