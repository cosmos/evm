package mempool

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/x/vm/statedb"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NotifiedMempool is the set of methods that a mempool must implement in order
// to be notified of new blocks by this Keeper via the EndBlocker.
type NotifiedMempool interface {
	// HasEventBus returns true if the mempool has an event bus configured to
	// get cometbft events
	HasEventBus() bool

	// GetBlockchain returns the mempools blockchain representation.
	GetBlockchain() *Blockchain
}

type VMKeeperI interface {
	GetBaseFee(ctx sdk.Context) *big.Int
	GetParams(ctx sdk.Context) (params vmtypes.Params)
	GetEvmCoinInfo(ctx sdk.Context) (coinInfo vmtypes.EvmCoinInfo)
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
	KVStoreKeys() map[string]storetypes.StoreKey
}

type FeeMarketKeeperI interface {
	GetBlockGasWanted(ctx sdk.Context) uint64
}
