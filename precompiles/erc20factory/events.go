package erc20factory

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// EventTypeCreate is the event type for the Create event.
	EventTypeCreate = "Create"
)

// EmitCreateEvent emits the Create event.
func (p Precompile) EmitCreateEvent(ctx sdk.Context, stateDB vm.StateDB, tokenAddress common.Address, salt [32]uint8, name string, symbol string, decimals uint8, minter common.Address, premintedSupply *big.Int) error {
	event := p.Events[EventTypeCreate]
	topics := make([]common.Hash, 2) // Only 2 topics: event ID + tokenAddress

	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(tokenAddress)
	if err != nil {
		return err
	}

	// Pack the non-indexed event parameters into the data field
	arguments := abi.Arguments{
		event.Inputs[1], // salt
		event.Inputs[2], // name
		event.Inputs[3], // symbol
		event.Inputs[4], // decimals
		event.Inputs[5], // minter
		event.Inputs[6], // premintedSupply
	}
	packed, err := arguments.Pack(salt, name, symbol, decimals, minter, premintedSupply)
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        packed,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // block height won't exceed uint64
	})

	return nil
}
