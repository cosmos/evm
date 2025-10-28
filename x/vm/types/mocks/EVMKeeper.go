package mocks

import (
	"errors"
	"maps"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	_             statedb.Keeper = &EVMKeeper{}
	ErrAddress    common.Address = common.BigToAddress(big.NewInt(100))
	EmptyCodeHash                = crypto.Keccak256(nil)
)

type Account struct {
	account statedb.Account
	states  statedb.Storage
}

type EVMKeeper struct {
	accounts  map[common.Address]Account
	codes     map[common.Hash][]byte
	storeKeys map[string]*storetypes.KVStoreKey
}

func NewEVMKeeper() *EVMKeeper {
	return &EVMKeeper{
		accounts:  make(map[common.Address]Account),
		codes:     make(map[common.Hash][]byte),
		storeKeys: make(map[string]*storetypes.KVStoreKey),
	}
}

func (k EVMKeeper) GetAccount(_ sdk.Context, addr common.Address) *statedb.Account {
	acct, ok := k.accounts[addr]
	if !ok {
		return nil
	}
	return &acct.account
}

func (k EVMKeeper) GetState(_ sdk.Context, addr common.Address, key common.Hash) common.Hash {
	return k.accounts[addr].states[key]
}

func (k EVMKeeper) GetCode(_ sdk.Context, codeHash common.Hash) []byte {
	return k.codes[codeHash]
}

func (k EVMKeeper) ForEachStorage(_ sdk.Context, addr common.Address, cb func(key, value common.Hash) bool) {
	if acct, ok := k.accounts[addr]; ok {
		for k, v := range acct.states {
			if !cb(k, v) {
				return
			}
		}
	}
}

func (k EVMKeeper) SetAccount(_ sdk.Context, addr common.Address, account statedb.Account) error {
	if addr == ErrAddress {
		return errors.New("mock db error")
	}
	acct, exists := k.accounts[addr]
	if exists {
		// update
		acct.account = account
		k.accounts[addr] = acct
	} else {
		k.accounts[addr] = Account{account: account, states: make(statedb.Storage)}
	}
	return nil
}

func (k EVMKeeper) SetState(_ sdk.Context, addr common.Address, key common.Hash, value []byte) {
	if acct, ok := k.accounts[addr]; ok {
		acct.states[key] = common.BytesToHash(value)
	}
}

func (k EVMKeeper) DeleteState(_ sdk.Context, addr common.Address, key common.Hash) {
	if acct, ok := k.accounts[addr]; ok {
		delete(acct.states, key)
	}
}

func (k EVMKeeper) SetCode(_ sdk.Context, codeHash []byte, code []byte) {
	k.codes[common.BytesToHash(codeHash)] = code
}

func (k EVMKeeper) DeleteCode(_ sdk.Context, codeHash []byte) {
	delete(k.codes, common.BytesToHash(codeHash))
}

func (k EVMKeeper) DeleteAccount(_ sdk.Context, addr common.Address) error {
	if addr == ErrAddress {
		return errors.New("mock db error")
	}
	old := k.accounts[addr]
	delete(k.accounts, addr)
	if !types.IsEmptyCodeHash(old.account.CodeHash) {
		delete(k.codes, common.BytesToHash(old.account.CodeHash))
	}
	return nil
}

func (k EVMKeeper) Clone() *EVMKeeper {
	accounts := maps.Clone(k.accounts)
	codes := maps.Clone(k.codes)
	storeKeys := maps.Clone(k.storeKeys)
	return &EVMKeeper{accounts, codes, storeKeys}
}

func (k EVMKeeper) KVStoreKeys() map[string]storetypes.StoreKey {
	result := make(map[string]storetypes.StoreKey, len(k.storeKeys))
	for k, v := range k.storeKeys {
		result[k] = v
	}
	return result
}

func (k EVMKeeper) GetCodeHash(_ sdk.Context, _ common.Address) common.Hash {
	return common.Hash{}
}
