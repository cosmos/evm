// Copyright 2025 The go-ethereum Authors
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

package reserver

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ErrAlreadyReserved is returned if the sender address has a pending transaction
// in a different subpool. For example, this error is returned in response to any
// input transaction of non-blob type when a blob transaction from this sender
// remains pending (and vice-versa).
var ErrAlreadyReserved = fmt.Errorf("address already reserved")

var (
	meter = otel.Meter("github.com/cosmos/evm/mempool/reserver")

	// reservationsGauge is a per-subpool address reservation count, tagged with
	// the subpool id.
	//
	// This is mostly a sanity metric to ensure there's no bug that would make
	// some subpool hog all the reservations due to mis-accounting.
	reservationsGauge metric.Int64UpDownCounter
)

// ReservationTracker is a struct shared between different Subpools. It is used to reserve
// the account and ensure that one address cannot initiate transactions, authorizations,
// and other state-changing behaviors in different pools at the same time.
type ReservationTracker struct {
	// map account => subpool id
	accounts map[common.Address]int
	lock     sync.RWMutex
}

// HandlerOpt is an option for creating a ReservationHandle.
type HandlerOpt func(*ReservationHandle)

// NewReservationTracker initializes the account reservation tracker.
func NewReservationTracker() *ReservationTracker {
	return &ReservationTracker{
		accounts: make(map[common.Address]int),
	}
}

func WithRefCounter() HandlerOpt {
	return func(h *ReservationHandle) {
		h.enableRefCount = true
		h.refsCounter = make(map[common.Address]int)
	}
}

// NewHandle creates a named handle on the ReservationTracker. The handle
// identifies the subpool so ownership of reservations can be determined.
func (r *ReservationTracker) NewHandle(id int, opts ...HandlerOpt) *ReservationHandle {
	h := &ReservationHandle{
		tracker: r,
		id:      id,
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// Reserver is an interface for creating and releasing owned reservations in the
// ReservationTracker struct, which is shared between Subpools.
type Reserver interface {
	// Hold attempts to reserve the specified account address for the given pool.
	// Returns an error if the account is already reserved.
	Hold(addr ...common.Address) error

	// Release attempts to release the reservation for the specified account.
	// Returns an error if the address is not reserved or is reserved by another pool.
	Release(addr ...common.Address) error

	// Has returns a flag indicating if the address has been reserved by a pool
	// other than one with the current Reserver handle.
	Has(address common.Address) bool
}

// ReservationHandle is a named handle on ReservationTracker. It is held by sub-pools
// to make reservations for accounts it is tracking. The id is used to determine
// which sub-pool owns an address and disallows non-owners to hold or release
// addresses it doesn't own.
//
// Additionally, with enableRefCount, the handler will track the refs
// to an address and will only release the address when refs==0
type ReservationHandle struct {
	tracker *ReservationTracker
	id      int

	enableRefCount bool

	// map account => ref count
	refsCounter map[common.Address]int
}

// Hold atomically reserves all addresses or none.
// Ensure addrs have NO duplicates.
// In most cases addrs is a single item.
func (h *ReservationHandle) Hold(addrs ...common.Address) error {
	h.tracker.lock.Lock()
	defer h.tracker.lock.Unlock()

	// dry run
	for _, addr := range addrs {
		if err := h.canHold(addr); err != nil {
			return err
		}
	}

	for _, addr := range addrs {
		// add only if not already owned by us
		if _, exists := h.tracker.accounts[addr]; !exists {
			h.tracker.accounts[addr] = h.id
			h.incReservations(1)
		}

		if h.enableRefCount {
			h.refsCounter[addr]++
		}
	}

	return nil
}

// Release atomically releases all addresses or none.
// Ensure addrs have NO duplicates.
// In most cases addrs is a single item.
func (h *ReservationHandle) Release(addrs ...common.Address) error {
	h.tracker.lock.Lock()
	defer h.tracker.lock.Unlock()

	// dry run
	for _, addr := range addrs {
		if err := h.canRelease(addr); err != nil {
			return err
		}
	}

	for _, addr := range addrs {
		// simple release
		if !h.enableRefCount {
			delete(h.tracker.accounts, addr)
			h.incReservations(-1)
			continue
		}

		// release with refsCounter check
		if refs, exists := h.refsCounter[addr]; !exists || refs <= 1 {
			delete(h.tracker.accounts, addr)
			delete(h.refsCounter, addr)
			h.incReservations(-1)
		} else {
			// just decrease the ref count, don't release yet
			h.refsCounter[addr]--
		}
	}

	return nil
}

// Has checks that address is already reserved by ANOTHER pool.
func (h *ReservationHandle) Has(address common.Address) bool {
	h.tracker.lock.RLock()
	defer h.tracker.lock.RUnlock()

	id, exists := h.tracker.accounts[address]
	return exists && id != h.id
}

func (h *ReservationHandle) canHold(addr common.Address) error {
	owner, exists := h.tracker.accounts[addr]

	if exists && owner != h.id {
		return fmt.Errorf("address %s: %w", addr.String(), ErrAlreadyReserved)
	}

	// doesn't exist or already owned by this pool
	return nil
}

func (h *ReservationHandle) canRelease(addr common.Address) error {
	owner, exists := h.tracker.accounts[addr]
	if !exists {
		return fmt.Errorf("address %s not reserved", addr.String())
	}

	if owner != h.id {
		return fmt.Errorf("address %s not owned by sub-pool %d", addr.String(), h.id)
	}

	return nil
}

func (h *ReservationHandle) incReservations(n int) {
	reservationsGauge.Add(context.Background(), int64(n), metric.WithAttributes(
		attribute.String("subpool_id", strconv.Itoa(h.id))),
	)
}

func init() {
	var err error
	reservationsGauge, err = meter.Int64UpDownCounter(
		"txpool.reservations",
		metric.WithDescription("Number of addresses currently reserved by a subpool"),
	)
	if err != nil {
		panic(err)
	}
}
