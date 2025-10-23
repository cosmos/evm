package nativeburn

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// EventTokenBurned defines the event data for the TokenBurned event
type EventTokenBurned struct {
	Burner common.Address
	Amount *big.Int
}
