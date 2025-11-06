// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package statedb

import (
	"bytes"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// JournalEntry is a modification entry in the state change journal that can be
// Reverted on demand.
type JournalEntry interface {
	// Revert undoes the changes introduced by this journal entry.
	Revert(*StateDB)

	// Dirtied returns the Ethereum address modified by this journal entry.
	Dirtied() *common.Address
}

// journal contains the list of state modifications applied since the last state
// commit. These are tracked to be able to be reverted in the case of an execution
// exception or request for reversal.
type journal struct {
	entries []JournalEntry         // Current changes tracked by the journal
	dirties map[common.Address]int // Dirty accounts and the number of changes
}

// newJournal creates a new initialized journal.
func newJournal() *journal {
	return &journal{
		dirties: make(map[common.Address]int),
	}
}

// reset clears the journal so it can be reused without reallocating.
func (j *journal) reset() {
	j.entries = j.entries[:0]
	for _, k := range j.sortedDirties() {
		delete(j.dirties, k)
	}
}

// sortedDirties sort the dirty addresses for deterministic iteration
func (j *journal) sortedDirties() []common.Address {
	keys := make([]common.Address, 0, len(j.dirties))
	for k := range j.dirties {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return bytes.Compare(keys[i].Bytes(), keys[j].Bytes()) < 0
	})
	return keys
}

// append inserts a new modification entry to the end of the change journal.
func (j *journal) append(entry JournalEntry) {
	j.entries = append(j.entries, entry)
	if addr := entry.Dirtied(); addr != nil {
		j.dirties[*addr]++
	}
}

// Revert undoes a batch of journaled modifications along with any Reverted
// dirty handling too.
func (j *journal) Revert(statedb *StateDB, snapshot int) {
	for i := len(j.entries) - 1; i >= snapshot; i-- {
		// Undo the changes made by the operation
		j.entries[i].Revert(statedb)

		// Drop any dirty tracking induced by the change
		if addr := j.entries[i].Dirtied(); addr != nil {
			if j.dirties[*addr]--; j.dirties[*addr] == 0 {
				delete(j.dirties, *addr)
			}
		}
	}
	j.entries = j.entries[:snapshot]
}

// length returns the current number of entries in the journal.
func (j *journal) length() int {
	return len(j.entries)
}

type (
	// Changes to the account trie.
	createObjectChange struct {
		account *common.Address
	}
	resetObjectChange struct {
		prev *stateObject
	}
	selfDestructChange struct {
		account     *common.Address
		prev        bool // whether account had already self-destructed
		prevbalance *uint256.Int
	}

	// Changes to individual accounts.
	balanceChange struct {
		account *common.Address
		prev    *uint256.Int
	}
	nonceChange struct {
		account *common.Address
		prev    uint64
	}
	storageChange struct {
		account   *common.Address
		key       common.Hash
		preValue  common.Hash
		origValue common.Hash
	}
	transientStorageChange struct {
		account       *common.Address
		key, prevalue common.Hash
	}
	codeChange struct {
		account            *common.Address
		prevcode, prevhash []byte
	}

	// Changes to other state values.
	refundChange struct {
		prev uint64
	}
	addLogChange struct{}

	// Changes to the access list
	accessListAddAccountChange struct {
		address *common.Address
	}
	accessListAddSlotChange struct {
		address *common.Address
		slot    *common.Hash
	}
	precompileCallChange struct {
		snapshot int
		events   sdk.Events
	}
	createContractChange struct {
		account *common.Address
	}
)

var (
	_ JournalEntry = createContractChange{}
	_ JournalEntry = createObjectChange{}
	_ JournalEntry = resetObjectChange{}
	_ JournalEntry = selfDestructChange{}
	_ JournalEntry = balanceChange{}
	_ JournalEntry = nonceChange{}
	_ JournalEntry = storageChange{}
	_ JournalEntry = transientStorageChange{}
	_ JournalEntry = codeChange{}
	_ JournalEntry = refundChange{}
	_ JournalEntry = addLogChange{}
	_ JournalEntry = accessListAddAccountChange{}
	_ JournalEntry = accessListAddSlotChange{}
	_ JournalEntry = precompileCallChange{}
)

func (ch createContractChange) Revert(s *StateDB) {
	s.getStateObject(*ch.account).newContract = false
}

func (ch createContractChange) Dirtied() *common.Address {
	return nil
}

func (pc precompileCallChange) Revert(s *StateDB) {
	// rollback multi store from cache ctx to the previous
	// state stored in the snapshot
	s.RevertMultiStore(pc.snapshot, pc.events)
}

func (pc precompileCallChange) Dirtied() *common.Address {
	return nil
}

func (ch createObjectChange) Revert(s *StateDB) {
	delete(s.stateObjects, *ch.account)
}

func (ch createObjectChange) Dirtied() *common.Address {
	return ch.account
}

func (ch resetObjectChange) Revert(s *StateDB) {
	s.setStateObject(ch.prev)
}

func (ch resetObjectChange) Dirtied() *common.Address {
	return nil
}

func (ch selfDestructChange) Revert(s *StateDB) {
	obj := s.getStateObject(*ch.account)
	if obj != nil {
		obj.selfDestructed = ch.prev
		obj.setBalance(ch.prevbalance)
	}
}

func (ch selfDestructChange) Dirtied() *common.Address {
	return ch.account
}

func (ch balanceChange) Revert(s *StateDB) {
	s.getStateObject(*ch.account).setBalance(ch.prev)
}

func (ch balanceChange) Dirtied() *common.Address {
	return ch.account
}

func (ch nonceChange) Revert(s *StateDB) {
	s.getStateObject(*ch.account).setNonce(ch.prev)
}

func (ch nonceChange) Dirtied() *common.Address {
	return ch.account
}

func (ch codeChange) Revert(s *StateDB) {
	s.getStateObject(*ch.account).setCode(common.BytesToHash(ch.prevhash), ch.prevcode)
}

func (ch codeChange) Dirtied() *common.Address {
	return ch.account
}

func (ch storageChange) Revert(s *StateDB) {
	s.getStateObject(*ch.account).setState(ch.key, ch.preValue, ch.origValue)
}

func (ch storageChange) Dirtied() *common.Address {
	return ch.account
}

func (ch transientStorageChange) Revert(s *StateDB) {
	s.setTransientState(*ch.account, ch.key, ch.prevalue)
}

func (ch transientStorageChange) Dirtied() *common.Address {
	return nil
}

func (ch refundChange) Revert(s *StateDB) {
	s.refund = ch.prev
}

func (ch refundChange) Dirtied() *common.Address {
	return nil
}

func (ch addLogChange) Revert(s *StateDB) {
	s.logs = s.logs[:len(s.logs)-1]
}

func (ch addLogChange) Dirtied() *common.Address {
	return nil
}

func (ch accessListAddAccountChange) Revert(s *StateDB) {
	/*
		One important invariant here, is that whenever a (addr, slot) is added, if the
		addr is not already present, the add causes two journal entries:
		- one for the address,
		- one for the (address,slot)
		Therefore, when unrolling the change, we can always blindly delete the
		(addr) at this point, since no storage adds can remain when come upon
		a single (addr) change.
	*/
	s.accessList.DeleteAddress(*ch.address)
}

func (ch accessListAddAccountChange) Dirtied() *common.Address {
	return nil
}

func (ch accessListAddSlotChange) Revert(s *StateDB) {
	s.accessList.DeleteSlot(*ch.address, *ch.slot)
}

func (ch accessListAddSlotChange) Dirtied() *common.Address {
	return nil
}
