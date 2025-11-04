package statedb

import (
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/stateless"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/trie/utils"
	"github.com/holiman/uint256"

	"github.com/cosmos/evm/x/vm/store/snapshotmulti"
	vmstoretypes "github.com/cosmos/evm/x/vm/store/types"
	"github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// revision is the identifier of a version of state.
// it consists of an auto-increment id and a journal index.
// it's safer to use than using journal index alone.
type revision struct {
	id           int
	journalIndex int
	events       sdk.Events
}

var _ vm.StateDB = &StateDB{}

// StateDB structs within the ethereum protocol are used to store anything
// within the merkle trie. StateDBs take care of caching and storing
// nested states. It's the general query interface to retrieve:
// * Contracts
// * Accounts
type StateDB struct {
	keeper Keeper
	ctx    sdk.Context
	// cacheCtx is used on precompile calls. It allows to commit the current journal
	// entries to get the updated state in for the precompile call.
	cacheCtx sdk.Context
	// writeCache function contains all the changes related to precompile calls.
	writeCache func()
	// snapshotter is used for snapshot creation and revert
	// this snapshot is used for precompile call
	snapshotter vmstoretypes.Snapshotter

	// Transient storage
	transientStorage transientStorage

	// Journal of state modifications. This is the backbone of
	// Snapshot and RevertToSnapshot.
	journal        *journal
	validRevisions []revision
	nextRevisionID int

	stateObjects map[common.Address]*stateObject

	txConfig TxConfig

	// The refund counter, also used by state transitioning.
	refund uint64

	// Per-transaction logs
	logs []*ethtypes.Log

	// Per-transaction access list
	accessList *accessList

	// The count of calls to precompiles
	precompileCallsCounter uint8
}

func (s *StateDB) CreateContract(address common.Address) {
	obj := s.getStateObject(address)
	if !obj.newContract {
		obj.newContract = true
		s.journal.append(createContractChange{
			&address,
		})
	}
}

// GetStorageRoot calculates the hash of the trie root by iterating through all storage objects for a given account
func (s *StateDB) GetStorageRoot(addr common.Address) common.Hash {
	sr := trie.NewStackTrie(nil)
	s.keeper.ForEachStorage(s.ctx, addr, func(key, value common.Hash) bool {
		if err := sr.Update(key.Bytes(), value.Bytes()); err != nil {
			s.ctx.Logger().Error("failed adding state during storage root hash", "err", err.Error())
			return false
		}
		return true
	})
	return sr.Hash()
}

func (s *StateDB) IsStorageEmpty(addr common.Address) bool {
	empty := true
	s.keeper.ForEachStorage(s.ctx, addr, func(key, value common.Hash) bool {
		empty = false
		return false
	})
	return empty
}

/*
	PointCache, Witness, and AccessEvents are all utilized for verkle trees.
	For now, we just return nil and verkle trees are not supported.
*/

func (s *StateDB) PointCache() *utils.PointCache {
	return nil
}

func (s *StateDB) Witness() *stateless.Witness {
	// TODO support verkle tries?
	return nil
}

func (s *StateDB) AccessEvents() *state.AccessEvents {
	return nil
}

func (s *StateDB) Finalise(deleteEmptyObjects bool) {
	for addr := range s.journal.dirties {
		obj, exist := s.stateObjects[addr]
		if !exist {
			continue
		}
		if obj.selfDestructed || (deleteEmptyObjects && obj.empty()) {
			delete(s.stateObjects, obj.address)
		}
	}
	// Invalidate journal because reverting across transactions is not allowed.
	s.clearJournalAndRefund()
}

// New creates a new state from a given trie.
func New(ctx sdk.Context, keeper Keeper, txConfig TxConfig) *StateDB {
	return &StateDB{
		keeper:           keeper,
		ctx:              ctx,
		stateObjects:     make(map[common.Address]*stateObject),
		journal:          newJournal(),
		accessList:       newAccessList(),
		transientStorage: newTransientStorage(),
		txConfig:         txConfig,
	}
}

// Keeper returns the underlying `Keeper`
func (s *StateDB) Keeper() Keeper {
	return s.keeper
}

// GetContext returns the transaction Context.
func (s *StateDB) GetContext() sdk.Context {
	return s.ctx
}

// GetCacheContext returns the stateDB CacheContext.
func (s *StateDB) GetCacheContext() (sdk.Context, error) {
	if s.writeCache == nil {
		err := s.cache()
		if err != nil {
			return s.ctx, err
		}
	}
	return s.cacheCtx, nil
}

// MultiStoreSnapshot snapshots stateDB CacheMultiStore
// and returns snapshot index
func (s *StateDB) MultiStoreSnapshot() int {
	return s.snapshotter.Snapshot()
}

func (s *StateDB) RevertMultiStore(snapshot int, events sdk.Events) {
	s.snapshotter.RevertToSnapshot(snapshot)
	s.writeCache = func() {
		s.ctx.EventManager().EmitEvents(events)
		s.cacheCtx.MultiStore().(storetypes.CacheMultiStore).Write()
	}
}

// cache creates the stateDB cache context
func (s *StateDB) cache() error {
	if s.ctx.MultiStore() == nil {
		return errors.New("ctx has no multi store")
	}
	s.cacheCtx, _ = s.ctx.CacheContext()

	// Get KVStores for modules wired to app
	cms := s.cacheCtx.MultiStore()
	storeKeys := s.keeper.KVStoreKeys()

	// Create and set snapshot store to stateDB
	snapshotStore := snapshotmulti.NewStore(cms, storeKeys)
	s.snapshotter = snapshotStore
	s.cacheCtx = s.cacheCtx.WithMultiStore(snapshotStore)
	s.writeCache = func() {
		s.ctx.EventManager().EmitEvents(s.cacheCtx.EventManager().Events())
		s.cacheCtx.MultiStore().(storetypes.CacheMultiStore).Write()
	}

	return nil
}

// AddLog adds a log, called by evm.
func (s *StateDB) AddLog(log *ethtypes.Log) {
	s.journal.append(addLogChange{})

	log.TxIndex = s.txConfig.TxIndex
	log.Index = uint(len(s.logs))
	s.logs = append(s.logs, log)
}

// Logs returns the logs of current transaction.
func (s *StateDB) Logs() []*ethtypes.Log {
	return s.logs
}

// AddRefund adds gas to the refund counter
func (s *StateDB) AddRefund(gas uint64) {
	s.journal.append(refundChange{prev: s.refund})
	s.refund += gas
}

// SubRefund removes gas from the refund counter.
// This method will panic if the refund counter goes below zero
func (s *StateDB) SubRefund(gas uint64) {
	s.journal.append(refundChange{prev: s.refund})
	if gas > s.refund {
		panic(fmt.Sprintf("Refund counter below zero (gas: %d > refund: %d)", gas, s.refund))
	}
	s.refund -= gas
}

// Exist reports whether the given account address exists in the state.
// Notably this also returns true for self-destructed accounts.
func (s *StateDB) Exist(addr common.Address) bool {
	return s.getStateObject(addr) != nil
}

// Empty returns whether the state object is either non-existent
// or empty according to the EIP161 specification (balance = nonce = code = 0)
func (s *StateDB) Empty(addr common.Address) bool {
	so := s.getStateObject(addr)
	return so == nil || so.empty()
}

// GetBalance retrieves the balance from the given address or 0 if object not found
func (s *StateDB) GetBalance(addr common.Address) *uint256.Int {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Balance()
	}
	return common.U2560
}

// GetNonce returns the nonce of account, 0 if not exists.
func (s *StateDB) GetNonce(addr common.Address) uint64 {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Nonce()
	}

	return 0
}

// GetCode returns the code of account, nil if not exists.
func (s *StateDB) GetCode(addr common.Address) []byte {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Code()
	}
	return nil
}

// GetCodeSize returns the code size of account.
func (s *StateDB) GetCodeSize(addr common.Address) int {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.CodeSize()
	}
	return 0
}

// GetCodeHash returns the code hash of account.
func (s *StateDB) GetCodeHash(addr common.Address) common.Hash {
	stateObject := s.getStateObject(addr)
	if stateObject == nil {
		return common.Hash{}
	}
	return common.BytesToHash(stateObject.CodeHash())
}

// GetState retrieves a value from the given account's storage trie.
func (s *StateDB) GetState(addr common.Address, hash common.Hash) common.Hash {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetState(hash)
	}
	return common.Hash{}
}

// GetCommittedState retrieves a value from the given account's committed storage trie.
func (s *StateDB) GetCommittedState(addr common.Address, hash common.Hash) common.Hash {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetCommittedState(hash)
	}
	return common.Hash{}
}

// GetStateAndCommittedState returns the current value and the original value.
func (s *StateDB) GetStateAndCommittedState(addr common.Address, hash common.Hash) (common.Hash, common.Hash) {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetState(hash), stateObject.GetCommittedState(hash)
	}
	return common.Hash{}, common.Hash{}
}

// SetStateOverride installs the provided storage value as part of the base state used by simulations.
func (s *StateDB) SetStateOverride(addr common.Address, key, value common.Hash) {
	stateObject := s.getOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetStateOverride(key, value)
	}
}

func (s *StateDB) clearJournalAndRefund() {
	if s.journal == nil {
		s.journal = newJournal()
	} else {
		s.journal.reset()
	}
	s.validRevisions = nil
	s.nextRevisionID = 0
	s.refund = 0
}

// GetRefund returns the current value of the refund counter.
func (s *StateDB) GetRefund() uint64 {
	return s.refund
}

// AddPreimage records a SHA3 preimage seen by the VM.
// AddPreimage performs a no-op since the EnablePreimageRecording flag is disabled
// on the vm.Config during state transitions. No store trie preimages are written
// to the database.
func (s *StateDB) AddPreimage(_ common.Hash, _ []byte) {}

// getStateObject retrieves a state object given by the address, returning nil if
// the object is not found.
func (s *StateDB) getStateObject(addr common.Address) *stateObject {
	// Prefer live objects if any is available
	if obj := s.stateObjects[addr]; obj != nil {
		return obj
	}
	// If no live objects are available, load it from keeper
	account := s.keeper.GetAccount(s.ctx, addr)
	if account == nil {
		return nil
	}
	// Insert into the live set
	obj := newObject(s, addr, *account)
	s.setStateObject(obj)
	return obj
}

// getOrNewStateObject retrieves a state object or create a new state object if nil.
func (s *StateDB) getOrNewStateObject(addr common.Address) *stateObject {
	stateObject := s.getStateObject(addr)
	if stateObject == nil {
		stateObject, _ = s.createObject(addr)
	}
	return stateObject
}

// createObject creates a new state object. If there is an existing account with
// the given address, it is overwritten and returned as the second return value.
func (s *StateDB) createObject(addr common.Address) (newobj, prev *stateObject) {
	prev = s.getStateObject(addr)

	newobj = newObject(s, addr, Account{})
	if prev == nil {
		s.journal.append(createObjectChange{account: &addr})
	} else {
		s.journal.append(resetObjectChange{prev: prev})
	}
	s.setStateObject(newobj)
	if prev != nil {
		return newobj, prev
	}
	return newobj, nil
}

// CreateAccount explicitly creates a state object. If a state object with the address
// already exists the balance is carried over to the new account.
//
// CreateAccount is called during the EVM CREATE operation. The situation might arise that
// a contract does the following:
//
// 1. sends funds to sha(account ++ (nonce + 1))
// 2. tx_create(sha(account ++ nonce)) (note that this gets the address of 1)
//
// Carrying over the balance ensures that Ether doesn't disappear.
func (s *StateDB) CreateAccount(addr common.Address) {
	newObj, prev := s.createObject(addr)
	if prev != nil {
		newObj.setBalance(prev.account.Balance)
	}
}

// ForEachStorage iterate the contract storage, the iteration order is not defined.
func (s *StateDB) ForEachStorage(addr common.Address, cb func(key, value common.Hash) bool) error {
	so := s.getStateObject(addr)
	if so == nil {
		return nil
	}
	s.keeper.ForEachStorage(s.ctx, addr, func(key, value common.Hash) bool {
		if value, dirty := so.dirtyStorage[key]; dirty {
			return cb(key, value)
		}
		if len(value) > 0 {
			return cb(key, value)
		}
		return true
	})
	return nil
}

func (s *StateDB) setStateObject(object *stateObject) {
	s.stateObjects[object.Address()] = object
}

/*
 * SETTERS
 */

// AddPrecompileFn adds a precompileCall journal entry
// with a snapshot of the multi-store and events previous
// to the precompile call.
func (s *StateDB) AddPrecompileFn(snapshot int, events sdk.Events) error {
	s.journal.append(precompileCallChange{
		snapshot: snapshot,
		events:   events,
	})
	s.precompileCallsCounter++
	if s.precompileCallsCounter > types.MaxPrecompileCalls {
		return fmt.Errorf("max calls to precompiles (%d) reached", types.MaxPrecompileCalls)
	}
	return nil
}

// AddBalance adds amount to the account associated with addr.
func (s *StateDB) AddBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	stateObject := s.getOrNewStateObject(addr)
	if stateObject == nil {
		return uint256.Int{}
	}
	return stateObject.AddBalance(amount)
}

// SubBalance subtracts amount from the account associated with addr.
func (s *StateDB) SubBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	stateObject := s.getOrNewStateObject(addr)
	if stateObject == nil {
		return uint256.Int{}
	}
	if amount.IsZero() {
		return *(stateObject.Balance())
	}
	return stateObject.SubBalance(amount)
}

// SetNonce sets the nonce of account.
func (s *StateDB) SetNonce(addr common.Address, nonce uint64, reason tracing.NonceChangeReason) {
	stateObject := s.getOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetNonce(nonce)
	}
}

// SetCode sets the code of account.
func (s *StateDB) SetCode(addr common.Address, code []byte) []byte {
	stateObject := s.getOrNewStateObject(addr)
	var prev []byte
	if stateObject != nil {
		prev = slices.Clone(stateObject.code)
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
	}
	return prev
}

// SetState sets the contract state.
func (s *StateDB) SetState(addr common.Address, key, value common.Hash) common.Hash {
	if stateObject := s.getOrNewStateObject(addr); stateObject != nil {
		return stateObject.SetState(key, value)
	}
	return common.Hash{}
}

// SetBalance sets the balance of account associated with addr to amount.
func (s *StateDB) SetBalance(addr common.Address, amount *uint256.Int, reason tracing.BalanceChangeReason) {
	stateObject := s.getOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetBalance(amount)
	}
}

// SetStorage replaces the entire storage for the specified account with given
// storage. This function should only be used for debugging and the mutations
// must be discarded afterwards.
func (s *StateDB) SetStorage(addr common.Address, storage Storage) {
	stateObject := s.getOrNewStateObject(addr)
	stateObject.SetStorage(storage)
}

// SelfDestruct marks the given account as self-destructed.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after SelfDestruct.
func (s *StateDB) SelfDestruct(addr common.Address) uint256.Int {
	stateObject := s.getStateObject(addr)
	var prevBalance uint256.Int
	if stateObject == nil {
		return prevBalance
	}
	prevBalance = *(stateObject.Balance())
	s.journal.append(selfDestructChange{
		account:     &addr,
		prev:        stateObject.selfDestructed,
		prevbalance: new(uint256.Int).Set(stateObject.Balance()),
	})
	stateObject.markSelfDestructed()
	stateObject.account.Balance = new(uint256.Int)
	return prevBalance
}

func (s *StateDB) SelfDestruct6780(addr common.Address) (uint256.Int, bool) {
	stateObject := s.getStateObject(addr)
	if stateObject == nil {
		return uint256.Int{}, false
	}

	if stateObject.newContract {
		return s.SelfDestruct(addr), true
	}
	return *(stateObject.Balance()), false
}

// HasSelfDestructed returns if the contract is self-destructed in current transaction.
func (s *StateDB) HasSelfDestructed(addr common.Address) bool {
	stateObject := s.getStateObject(addr)
	if stateObject != nil {
		return stateObject.selfDestructed
	}
	return false
}

// SetTransientState sets transient storage for a given account. It
// adds the change to the journal so that it can be rolled back
// to its previous value if there is a revert.
func (s *StateDB) SetTransientState(addr common.Address, key, value common.Hash) {
	prev := s.GetTransientState(addr, key)
	if prev == value {
		return
	}
	s.journal.append(transientStorageChange{
		account:  &addr,
		key:      key,
		prevalue: prev,
	})
	s.setTransientState(addr, key, value)
}

// setTransientState is a lower level setter for transient storage. It
// is called during a revert to prevent modifications to the journal.
func (s *StateDB) setTransientState(addr common.Address, key, value common.Hash) {
	s.transientStorage.Set(addr, key, value)
}

// GetTransientState gets transient storage for a given account.
func (s *StateDB) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	return s.transientStorage.Get(addr, key)
}

// Prepare handles the preparatory steps for executing a state transition with.
// This method must be invoked before state transition.
//
// Berlin fork:
// - Add sender to access list (2929)
// - Add destination to access list (2929)
// - Add precompiles to access list (2929)
// - Add the contents of the optional tx access list (2930)
//
// Potential EIPs:
// - Reset access list (Berlin)
// - Add coinbase to access list (EIP-3651)
// - Reset transient storage (EIP-1153)
func (s *StateDB) Prepare(rules params.Rules, sender, coinbase common.Address, dst *common.Address,
	precompiles []common.Address, list ethtypes.AccessList,
) {
	if rules.IsEIP2929 && rules.IsEIP4762 {
		panic("eip2929 and eip4762 are both activated")
	}
	if rules.IsEIP2929 {
		// Clear out any leftover from previous executions
		al := newAccessList()
		s.accessList = al

		al.AddAddress(sender)
		if dst != nil {
			al.AddAddress(*dst)
			// If it's a create-tx, the destination will be added inside evm.create
		}
		for _, addr := range precompiles {
			al.AddAddress(addr)
		}
		for _, el := range list {
			al.AddAddress(el.Address)
			for _, key := range el.StorageKeys {
				al.AddSlot(el.Address, key)
			}
		}
		if rules.IsShanghai { // EIP-3651: warm coinbase
			al.AddAddress(coinbase)
		}
	}
	// Reset transient storage at the beginning of transaction execution
	s.transientStorage = newTransientStorage()
}

// AddAddressToAccessList adds the given address to the access list
func (s *StateDB) AddAddressToAccessList(addr common.Address) {
	if s.accessList.AddAddress(addr) {
		s.journal.append(accessListAddAccountChange{&addr})
	}
}

// AddSlotToAccessList adds the given (address, slot)-tuple to the access list
func (s *StateDB) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	addrMod, slotMod := s.accessList.AddSlot(addr, slot)
	if addrMod {
		// In practice, this should not happen, since there is no way to enter the
		// scope of 'address' without having the 'address' become already added
		// to the access list (via call-variant, create, etc).
		// Better safe than sorry, though
		s.journal.append(accessListAddAccountChange{&addr})
	}
	if slotMod {
		s.journal.append(accessListAddSlotChange{
			address: &addr,
			slot:    &slot,
		})
	}
}

// AddressInAccessList returns true if the given address is in the access list.
func (s *StateDB) AddressInAccessList(addr common.Address) bool {
	return s.accessList.ContainsAddress(addr)
}

// SlotInAccessList returns true if the given (address, slot)-tuple is in the access list.
func (s *StateDB) SlotInAccessList(addr common.Address, slot common.Hash) (addressPresent bool, slotPresent bool) {
	return s.accessList.Contains(addr, slot)
}

// Snapshot returns an identifier for the current revision of the state.
func (s *StateDB) Snapshot() int {
	id := s.nextRevisionID
	s.nextRevisionID++
	s.validRevisions = append(s.validRevisions, revision{id, s.journal.length(), s.ctx.EventManager().Events()})
	return id
}

// RevertToSnapshot reverts all state changes made since the given revision.
func (s *StateDB) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(s.validRevisions), func(i int) bool {
		return s.validRevisions[i].id >= revid
	})
	if idx == len(s.validRevisions) || s.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := s.validRevisions[idx].journalIndex

	// revert back to snapshotted events
	eventManager := sdk.NewEventManager()
	eventManager.EmitEvents(s.validRevisions[idx].events)
	s.ctx = s.ctx.WithEventManager(eventManager)

	// Replay the journal to undo changes and remove invalidated snapshots
	s.journal.Revert(s, snapshot)
	s.validRevisions = s.validRevisions[:idx]
}

// Commit writes the dirty states to keeper
// the StateDB object should be discarded after committed.
func (s *StateDB) Commit() error {
	// writeCache func will exist only when there's a call to a precompile.
	// It applies all the store updates preformed by precompile calls.
	if s.writeCache != nil {
		s.writeCache()
	}
	return s.commitWithCtx(s.ctx, false)
}

// CommitWithCacheCtx writes the dirty states to keeper using the cacheCtx.
// This function is used before any precompile call to make sure the cacheCtx
// is updated with the latest changes within the tx (StateDB's journal entries).
func (s *StateDB) CommitWithCacheCtx() error {
	return s.commitWithCtx(s.cacheCtx, true)
}

// commitWithCtx writes the dirty states to keeper
// using the provided context
func (s *StateDB) commitWithCtx(ctx sdk.Context, keepDirty bool) error {
	for _, addr := range s.journal.sortedDirties() {
		obj := s.stateObjects[addr]
		if obj.selfDestructed {
			if err := s.keeper.DeleteAccount(ctx, obj.Address()); err != nil {
				return errorsmod.Wrapf(err, "failed to delete account %s", obj.Address())
			}
		} else {
			if obj.code != nil && obj.dirtyCode {
				if len(obj.code) == 0 {
					s.keeper.DeleteCode(ctx, obj.CodeHash())
				} else {
					s.keeper.SetCode(ctx, obj.CodeHash(), obj.code)
				}
			}
			if err := s.keeper.SetAccount(ctx, obj.Address(), obj.account); err != nil {
				return errorsmod.Wrap(err, "failed to set account")
			}

			for _, key := range obj.dirtyStorage.SortedKeys() {
				valueBytes := obj.dirtyStorage[key].Bytes()
				if len(valueBytes) == 0 {
					s.keeper.DeleteState(ctx, obj.Address(), key)
				} else {
					s.keeper.SetState(ctx, obj.Address(), key, valueBytes)
				}

				// Track the persisted value as the new baseline so later writes compare
				// against the most recently flushed state. In go-ethereum the same happens
				// during commitStorage: originStorage follows the latest flush and act as
				// the reference for future dirty detection. The actual revert path still
				// consults the journal's origvalue, so keeping this cache in sync during
				// cacheCtx flushes is safe even if a precompile subsequently reverts.
				if obj.originStorage == nil {
					obj.originStorage = make(Storage)
				}
				if len(valueBytes) == 0 {
					delete(obj.originStorage, key)
				} else {
					obj.originStorage[key] = obj.dirtyStorage[key]
				}
				// Only the final Commit against the root context should clear dirtyStorage.
				// During precompile execution we pass keepDirty=true so that the slots remain
				// marked dirty after cacheCtx flushes, letting the outer transaction still
				// persist those writes (or revert them via the journal) once execution finishes.
				//
				// This is essential after SetState started syncing originStorage: the cacheCtx
				// flush must leave the dirty slots intact so the final Commit() can push the
				// same updates into the keeper context.
				if !keepDirty {
					delete(obj.dirtyStorage, key)
				}
			}
		}
	}
	return nil
}
