package mempool

import (
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/x/vm/statedb"
	vmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type VMKeeperI interface {
	GetBaseFee(ctx sdk.Context) *big.Int
	GetParams(ctx sdk.Context) (params vmtypes.Params)
	GetAccount(ctx sdk.Context, addr common.Address) *statedb.Account
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
	KVStoreKeys() map[string]*storetypes.KVStoreKey
}

type FeeMarketKeeperI interface {
	GetBlockGasWanted(ctx sdk.Context) uint64
}
