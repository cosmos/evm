package common

import (
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/yihuang/go-abi"
)

//go:generate go run github.com/yihuang/go-abi/cmd -var=CommonABI -output common.abi.go

var CommonABI = []string{
	"struct Coin {string denom; uint256 amount;}",
	"struct DecCoin {string denom; uint256 amount; uint8 precision;}",
	"struct Dec {uint256 value; uint8 precision;}",
	"struct Height { uint64 revisionNumber; uint64 revisionHeight; }",
	"struct PageRequest { bytes key; uint64 offset; uint64 limit; bool countTotal; bool reverse; }",
	"struct PageResponse { bytes nextKey; uint64 total; }",
	"struct ICS20Allocation { string sourcePort; string sourceChannel; Coin[] spendLimit; string[] allowList; string[] allowedPacketData; }",

	// there's no dedicated tyeps for structs in ABI,
	// the dummy function to keep them in the ABI
	"function dummy(Coin a, DecCoin b, Dec c, Height d, PageRequest e, PageResponse f, ICS20Allocation g)",
}

// UnpackLog unpacks a retrieved log into the provided output structure.
func UnpackLog(event abi.Event, log ethtypes.Log) error {
	if _, err := event.Decode(log.Data); err != nil {
		return err
	}
	return event.DecodeTopics(log.Topics)
}
