package statedb

import (
	"bytes"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"

	"github.com/cosmos/evm/x/vm/types"
)

// Account is the Ethereum consensus representation of accounts.
// These objects are stored in the storage of auth module.
type Account struct {
	Nonce    uint64
	Balance  *uint256.Int
	CodeHash []byte
}

// NewEmptyAccount returns an empty account.
func NewEmptyAccount() *Account {
	return &Account{
		Balance:  new(uint256.Int),
		CodeHash: types.EmptyCodeHash,
	}
}

// IsContract returns if the account contains contract code.
func (acct Account) HasCodeHash() bool {
	return !types.IsEmptyCodeHash(acct.CodeHash)
}

// Storage represents in-memory cache/buffer of contract storage.
type Storage map[common.Hash]common.Hash

func (s Storage) Copy() Storage {
	cpy := make(Storage, len(s))
	for key, value := range s {
		cpy[key] = value
	}
	return cpy
}

// SortedKeys sort the keys for deterministic iteration
func (s Storage) SortedKeys() []common.Hash {
	keys := make([]common.Hash, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return bytes.Compare(keys[i].Bytes(), keys[j].Bytes()) < 0
	})
	return keys
}

// stateObject is the state of an account
type stateObject struct {
	db *StateDB

	account Account
	code    []byte

	// state storage
	originStorage Storage
	dirtyStorage  Storage
	// overridden state, when not nil, replace the whole committed state,
	// mainly to support the stateOverrides in eth_call.
	overrideStorage Storage

	address common.Address

	// flags
	dirtyCode      bool
	selfDestructed bool
	newContract    bool
}

// newObject creates a state object.
func newObject(db *StateDB, address common.Address, account Account) *stateObject {
	if account.Balance == nil {
		account.Balance = new(uint256.Int)
	}

	if account.CodeHash == nil {
		account.CodeHash = types.EmptyCodeHash
	}

	return &stateObject{
		db:            db,
		address:       address,
		account:       account,
		originStorage: make(Storage),
		dirtyStorage:  make(Storage),
	}
}

// empty returns whether the account is considered empty.
func (s *stateObject) empty() bool {
	return s.account.Nonce == 0 &&
		s.account.Balance.Sign() == 0 &&
		types.IsEmptyCodeHash(s.account.CodeHash)
}

func (s *stateObject) markSelfDestructed() {
	s.selfDestructed = true
}

// AddBalance adds amount to s's balance.
// It is used to add funds to the destination account of a transfer.
func (s *stateObject) AddBalance(amount *uint256.Int) uint256.Int {
	if amount.IsZero() {
		return *(s.Balance())
	}
	return s.SetBalance(new(uint256.Int).Add(s.Balance(), amount))
}

// SubBalance removes amount from s's balance.
// It is used to remove funds from the origin account of a transfer.
// Returns the previous balance
func (s *stateObject) SubBalance(amount *uint256.Int) uint256.Int {
	if amount.IsZero() {
		return *(s.Balance())
	}
	return s.SetBalance(new(uint256.Int).Sub(s.Balance(), amount))
}

// SetBalance updates account balance.
// Returns the previous value.
func (s *stateObject) SetBalance(amount *uint256.Int) uint256.Int {
	prev := *s.account.Balance
	s.db.journal.append(balanceChange{
		account: &s.address,
		prev:    new(uint256.Int).Set(s.account.Balance),
	})
	s.setBalance(amount)
	return prev
}

func (s *stateObject) setBalance(amount *uint256.Int) {
	s.account.Balance = amount
}

//
// Attribute accessors
//

// Returns the address of the contract/account
func (s *stateObject) Address() common.Address {
	return s.address
}

// Code returns the contract code associated with this object, if any.
func (s *stateObject) Code() []byte {
	if s.code != nil {
		return s.code
	}

	if types.IsEmptyCodeHash(s.CodeHash()) {
		return nil
	}

	code := s.db.keeper.GetCode(s.db.ctx, common.BytesToHash(s.CodeHash()))
	s.code = code

	return code
}

// CodeSize returns the size of the contract code associated with this object,
// or zero if none.
func (s *stateObject) CodeSize() int {
	return len(s.Code())
}

// SetCode set contract code to account
func (s *stateObject) SetCode(codeHash common.Hash, code []byte) {
	prevcode := s.Code()
	s.db.journal.append(codeChange{
		account:  &s.address,
		prevhash: s.CodeHash(),
		prevcode: prevcode,
	})
	s.setCode(codeHash, code)
}

func (s *stateObject) setCode(codeHash common.Hash, code []byte) {
	s.code = code
	s.account.CodeHash = codeHash[:]
	s.dirtyCode = true
}

// SetCode set nonce to account
func (s *stateObject) SetNonce(nonce uint64) {
	s.db.journal.append(nonceChange{
		account: &s.address,
		prev:    s.account.Nonce,
	})
	s.setNonce(nonce)
}

func (s *stateObject) setNonce(nonce uint64) {
	s.account.Nonce = nonce
}

// CodeHash returns the code hash of account
func (s *stateObject) CodeHash() []byte {
	return s.account.CodeHash
}

// Balance returns the balance of account
func (s *stateObject) Balance() *uint256.Int {
	return s.account.Balance
}

// Nonce returns the nonce of account
func (s *stateObject) Nonce() uint64 {
	return s.account.Nonce
}

// GetCommittedState query the committed state
func (s *stateObject) GetCommittedState(key common.Hash) common.Hash {
	if s.overrideStorage != nil {
		if value, ok := s.overrideStorage[key]; ok {
			return value
		}
		return common.Hash{}
	}

	if value, cached := s.originStorage[key]; cached {
		return value
	}
	// If no live objects are available, load it from keeper
	value := s.db.keeper.GetState(s.db.ctx, s.Address(), key)
	s.originStorage[key] = value
	return value
}

// GetState query the current state (including dirty state)
func (s *stateObject) GetState(key common.Hash) common.Hash {
	if value, dirty := s.dirtyStorage[key]; dirty {
		return value
	}
	return s.GetCommittedState(key)
}

// SetState sets the contract state
// It returns the previous value
func (s *stateObject) SetState(key common.Hash, value common.Hash) common.Hash {
	// If the new value is the same as old, don't set
	prev := s.GetState(key)
	if prev == value {
		return prev
	}
	origin := s.GetCommittedState(key)
	// New value is different, update and journal the change
	s.db.journal.append(storageChange{
		account:   &s.address,
		key:       key,
		preValue:  prev,
		origValue: origin,
	})
	s.setState(key, value, origin)
	return prev
}

// SetStorage overrides the entire contract storage for this state object.
// This replaces the committed state with the provided storage map, clearing
// any previous origin and dirty storage.
func (s *stateObject) SetStorage(storage Storage) {
	s.overrideStorage = storage
	s.originStorage = make(Storage)
	s.dirtyStorage = make(Storage)
}

// setState updates a value in account dirty storage. The dirtiness will be
// removed if the value being set equals to the original value.
func (s *stateObject) setState(key, value, origin common.Hash) {
	// Storage slot is set back to its original value, undo the dirty marker
	if value == origin {
		delete(s.dirtyStorage, key)
		return
	}
	s.dirtyStorage[key] = value
}

// SetStateOverride installs the provided storage value as part of the base state used for
// simulations. Subsequent reads treat the slot as if it were committed on-chain.
func (s *stateObject) SetStateOverride(key, value common.Hash) {
	if s.originStorage == nil {
		s.originStorage = make(Storage)
	}
	s.originStorage[key] = value
	delete(s.dirtyStorage, key)
	if s.overrideStorage != nil {
		s.overrideStorage[key] = value
	}
}
