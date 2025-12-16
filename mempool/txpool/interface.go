package txpool

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type VMKeeper interface {
	GetBaseFee(ctx sdk.Context) *big.Int
}
