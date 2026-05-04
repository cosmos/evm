package keeper

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/statedb"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RegisterERC20Extension creates and adds an ERC20 precompile interface for an IBC Coin.
//
// It derives the ERC-20 address from the token denomination and registers the
// EVM extension as an active dynamic precompile.
//
// CONTRACT: This must ONLY be called if there is no existing token pair for the given denom.
func (k Keeper) RegisterERC20Extension(ctx sdk.Context, denom string) (*types.TokenPair, error) {
	pair, err := k.CreateNewTokenPair(ctx, denom)
	if err != nil {
		return nil, err
	}

	// Add to existing EVM extensions
	if err := k.EnableDynamicPrecompile(ctx, pair.GetERC20Contract()); err != nil {
		return nil, err
	}

	return &pair, err
}

// RegisterERC20CodeHash sets the codehash for the erc20 precompile account
// if the bytecode for the erc20 codehash does not exists, it stores it.
func (k Keeper) RegisterERC20CodeHash(ctx sdk.Context, erc20Addr common.Address) error {
	var (
		// bytecode and codeHash is the same for all IBC coins
		// cause they're all using the same contract
		bytecode = common.FromHex(types.Erc20Bytecode)
		codeHash = crypto.Keccak256(bytecode)
	)
	// check if code was already stored
	code := k.evmKeeper.GetCode(ctx, common.Hash(codeHash))
	if len(code) == 0 {
		k.evmKeeper.SetCode(ctx, codeHash, bytecode)
	}

	var (
		nonce   uint64
		balance = common.U2560
	)
	// keep balance and nonce if account exists
	if acc := k.evmKeeper.GetAccount(ctx, erc20Addr); acc != nil {
		nonce = acc.Nonce
		balance = acc.Balance
	}

	return k.evmKeeper.SetAccount(ctx, erc20Addr, statedb.Account{
		CodeHash: codeHash,
		Nonce:    nonce,
		Balance:  balance,
	})
}

// UnRegisterERC20CodeHash sets the codehash for the account to an empty one
func (k Keeper) UnRegisterERC20CodeHash(ctx sdk.Context, erc20Addr common.Address) error {
	emptyCodeHash := crypto.Keccak256(nil)

	var (
		nonce   uint64
		balance = common.U2560
	)
	// keep balance and nonce if account exists
	if acc := k.evmKeeper.GetAccount(ctx, erc20Addr); acc != nil {
		nonce = acc.Nonce
		balance = acc.Balance
	}

	return k.evmKeeper.SetAccount(ctx, erc20Addr, statedb.Account{
		CodeHash: emptyCodeHash,
		Nonce:    nonce,
		Balance:  balance,
	})
}
