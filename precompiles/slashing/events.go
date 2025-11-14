package slashing

import (
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/yihuang/go-abi"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EmitValidatorUnjailedEvent emits the ValidatorUnjailed event
func (p Precompile) EmitValidatorUnjailedEvent(ctx sdk.Context, stateDB vm.StateDB, validator common.Address) error {
	// Create the event using the generated constructor
	event := NewValidatorUnjailedEvent(validator)

	// Prepare the event topics
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
