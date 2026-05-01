package types

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AccountKeeper defines the contract required for account APIs.
type AccountKeeper interface {
	NewAccountWithAddress(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	SetAccount(ctx context.Context, acc sdk.AccountI)
	HasAccount(ctx context.Context, addr sdk.AccAddress) bool
}

// EVMKeeper defines the expected EVM keeper interface used on erc20
type EVMKeeper interface {
	CallEVM(ctx sdk.Context, stateDB *statedb.StateDB, abi abi.ABI, from, contract common.Address, commit bool, callFromPrecompile bool, gasCap *big.Int, method string, args ...interface{}) (*evmtypes.MsgEthereumTxResponse, error)
	CallEVMWithData(ctx sdk.Context, stateDB *statedb.StateDB, from common.Address, contract *common.Address, data []byte, commit bool, callFromPrecompile bool, gasCap *big.Int) (*evmtypes.MsgEthereumTxResponse, error)
	GetAccountOrEmpty(ctx sdk.Context, addr common.Address) statedb.Account
	GetAccount(ctx sdk.Context, addr common.Address) *statedb.Account
	IsContract(ctx sdk.Context, addr common.Address) bool
	GetState(ctx sdk.Context, addr common.Address, key common.Hash) common.Hash
	GetCode(ctx sdk.Context, codeHash common.Hash) []byte
	GetCodeHash(ctx sdk.Context, addr common.Address) common.Hash
	ForEachStorage(ctx sdk.Context, addr common.Address, cb func(key common.Hash, value common.Hash) bool)
	SetAccount(ctx sdk.Context, addr common.Address, account statedb.Account) error
	DeleteState(ctx sdk.Context, addr common.Address, key common.Hash)
	SetState(ctx sdk.Context, addr common.Address, key common.Hash, value []byte)
	DeleteCode(ctx sdk.Context, codeHash []byte)
	SetCode(ctx sdk.Context, codeHash []byte, code []byte)
	DeleteAccount(ctx sdk.Context, addr common.Address) error
	KVStoreKeys() map[string]storetypes.StoreKey
	TransientStoreKeys() map[string]*storetypes.TransientStoreKey
}

type ERC20Keeper interface {
	GetTokenPairID(ctx sdk.Context, token string) []byte
	GetTokenPair(ctx sdk.Context, id []byte) (types.TokenPair, bool)
	SetAllowance(ctx sdk.Context, erc20 common.Address, owner common.Address, spender common.Address, value *big.Int) error
	BalanceOf(ctx sdk.Context, abi abi.ABI, contract, account common.Address) *big.Int
}
