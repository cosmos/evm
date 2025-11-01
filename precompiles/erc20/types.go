package erc20

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// EventTransfer defines the event data for the ERC20 Transfer events.
type EventTransfer struct {
	From  common.Address
	To    common.Address
	Value *big.Int
}

// EventApproval defines the event data for the ERC20 Approval events.
type EventApproval struct {
	Owner   common.Address
	Spender common.Address
	Value   *big.Int
}
