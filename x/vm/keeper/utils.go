package keeper

import (
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// IsContract determines if the given address is a smart contract.
func (k *Keeper) IsContract(ctx sdk.Context, addr common.Address) bool {
	codeHash := k.GetCodeHash(ctx, addr)
	code := k.GetCode(ctx, codeHash)

	_, delegated := ethtypes.ParseDelegation(code)
	return len(code) > 0 && !delegated
}
