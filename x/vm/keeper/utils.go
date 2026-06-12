package keeper

import (
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// IsContract determines if the given address is a smart contract.
// It checks if the account has associated code and ensures that
// the code is not a delegated contract (EIP-7702).
func (k *Keeper) IsContract(ctx sdk.Context, addr common.Address) bool {
	codeHash := k.GetCodeHash(ctx, addr)
	code := k.GetCode(ctx, codeHash)

	_, delegated := ethtypes.ParseDelegation(code)
	return len(code) > 0 && !delegated
}
