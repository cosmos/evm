package ics20

import (
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EmitIBCTransferEvent creates a new IBC transfer event emitted on a Transfer transaction.
func EmitIBCTransferEvent(
	ctx sdk.Context,
	stateDB vm.StateDB,
	precompileAddr, senderAddr common.Address,
	receiver string,
	sourcePort, sourceChannel string,
	token sdk.Coin,
	memo string,
) error {
	// Create the event using the generated constructor
	event := NewIBCTransferEvent(senderAddr, receiver, sourcePort, sourceChannel, token.Denom, token.Amount.BigInt(), memo)

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
		Address:     precompileAddr,
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115 // won't exceed uint64
	})

	return nil
}
