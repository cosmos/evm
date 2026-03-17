package indexer

import (
	"github.com/ethereum/go-ethereum/common"

	abci "github.com/cometbft/cometbft/abci/types"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

const (
	// Event mode values from Cosmos SDK's "mode" attribute
	EventModePreBlock   = ""           // PreBlock events have no mode attribute
	EventModeBeginBlock = "BeginBlock" // BeginBlock events
	EventModeEndBlock   = "EndBlock"   // EndBlock events
)

// getEthTxHash extracts the ethereum tx hash from an ethereum_tx event.
// Returns empty hash if the event doesn't have ethereumTxHash attribute.
// This handles both format1 (single event with all attributes) and format2 (split events)
func getEthTxHash(event abci.Event) common.Hash {
	for _, attr := range event.Attributes {
		if attr.Key == evmtypes.AttributeKeyEthereumTxHash {
			return common.HexToHash(attr.Value)
		}
	}
	return common.Hash{}
}

// getEventMode extracts the "mode" attribute from an event
func getEventMode(event abci.Event) string {
	for _, attr := range event.Attributes {
		if attr.Key == "mode" {
			return attr.Value
		}
	}
	return ""
}

// partitionFinalizeBlockEvents classifies FinalizeBlockEvents by their execution mode.
// Returns three slices: preblockEvents, beginBlockEvents, endBlockEvents
// Events are classified by the "mode" attribute added by Cosmos SDK:
// - PreBlock: mode="" (empty string, no mode attribute)
// - BeginBlock: mode="BeginBlock"
// - EndBlock: mode="EndBlock"
// Each phase generates a single synthetic tx, so eventIndex within each phase is used
// for log ordering only (not for EthTxHash generation).
func partitionFinalizeBlockEvents(events []abci.Event) ([]abci.Event, []abci.Event, []abci.Event) {
	var preblockEvents, beginBlockEvents, endBlockEvents []abci.Event

	for _, event := range events {
		mode := getEventMode(event)
		switch mode {
		case EventModePreBlock:
			preblockEvents = append(preblockEvents, event)
		case EventModeBeginBlock:
			beginBlockEvents = append(beginBlockEvents, event)
		case EventModeEndBlock:
			endBlockEvents = append(endBlockEvents, event)
		}
	}

	return preblockEvents, beginBlockEvents, endBlockEvents
}
