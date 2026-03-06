package legacypool

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/core/types"
)

type nopRechecker struct{}

func newNopRechecker() nopRechecker {
	return nopRechecker{}
}

func (nr nopRechecker) Recheck(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
	return sdk.Context{}, nil
}

func (nr nopRechecker) GetContext() (sdk.Context, func()) {
	return sdk.Context{}, func() {}
}

func (nr nopRechecker) Update(chain BlockChain, header *types.Header) {
}
