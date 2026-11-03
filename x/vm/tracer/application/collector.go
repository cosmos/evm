package application

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	vmtracer "github.com/cosmos/evm/x/vm/tracer"
)

// Collector retains attempts, raw facts and current call outcomes in one
// execution journal.
type Collector interface {
	vmtracer.Journal
	Observe(vmtracer.Observer) (func(), error)
}

// txTraceCollector owns the execution journal, including reverted attempts.
type txTraceCollector struct {
	events            []vmtracer.Observation
	calls             map[uint64]*observationCall
	stack             []uint64
	subscribers       []observationSubscriber
	nextCall          uint64
	nextObservationID uint64
	nextSubscriber    uint64
	maxEvents         uint64
	code              func(common.Address) []byte
	err               error
	executionErr      error
}

type observationCall struct {
	index  int
	parent uint64
	status vmtracer.CallStatus
}

type observationSubscriber struct {
	id uint64
	fn vmtracer.Observer
}

const DefaultObservationBudget uint64 = 100_000

var (
	erc20TransferSelector     = []byte{0xa9, 0x05, 0x9c, 0xbb}
	erc20TransferFromSelector = []byte{0x23, 0xb8, 0x72, 0xdd}
)

// NewCollector creates the default transaction-trace collector.
func NewCollector() Collector {
	c, _ := NewCollectorWithBudget(DefaultObservationBudget)
	return c
}

func NewCollectorWithBudget(budget uint64) (Collector, error) {
	if budget == 0 {
		return nil, errors.New("collector requires a positive budget")
	}
	return &txTraceCollector{
		calls:     make(map[uint64]*observationCall),
		maxEvents: budget,
	}, nil
}

// Hooks returns the EVM callbacks used by the collector.
func (c *txTraceCollector) Hooks() *tracing.Hooks {
	if c == nil {
		return nil
	}
	return &tracing.Hooks{
		OnEnter: c.onEnter,
		OnExit:  c.onExit,
		OnBalanceChange: func(addr common.Address, prev, next *big.Int, reason tracing.BalanceChangeReason) {
			c.emitObservation(vmtracer.Observation{
				Kind:            vmtracer.BalanceChange,
				Target:          addr,
				PreviousBalance: prev,
				CurrentBalance:  next,
				BalanceReason:   reason,
			})
		},
		OnLog: func(log *ethtypes.Log) {
			c.emitObservation(vmtracer.Observation{
				Kind: vmtracer.Log,
				Log:  log,
			})
		},
	}
}

func (c *txTraceCollector) Error() error {
	if c == nil {
		return nil
	}

	return errors.Join(c.err, c.executionErr)
}

// LatchError retains execution errors across EVM and observer rollback.
func (c *txTraceCollector) LatchError(err error) {
	if c != nil && c.executionErr == nil {
		c.executionErr = err
	}
}

// ExecutionError reports enforcement errors, including incomplete observation
// coverage while a policy observer is installed. Passive tracing errors alone
// do not change ordinary VM execution.
func (c *txTraceCollector) ExecutionError() error {
	if c == nil {
		return nil
	}

	return c.executionErr
}

func (c *txTraceCollector) collectionError(err error) {
	if c == nil || err == nil {
		return
	}
	if c.err == nil {
		c.err = err
	}
	if len(c.subscribers) > 0 {
		c.LatchError(c.err)
	}
}

func (c *txTraceCollector) LastObservationID() uint64 {
	if c == nil {
		return 0
	}

	return c.nextObservationID
}

// SetCodeReader supplies the current StateDB code at call entry. Keeper binds
// it before execution, including state overrides and EIP-7702 authorizations.
func (c *txTraceCollector) SetCodeReader(reader func(common.Address) []byte) func() {
	if c == nil {
		return func() {}
	}

	previous := c.code
	c.code = reader

	return func() { c.code = previous }
}

// status combines the local outcome with any enclosing rollback. Attempts
// remain stored; a later parent revert changes their effective status on read.
func (c *txTraceCollector) status(id uint64) vmtracer.CallStatus {
	f := c.calls[id]
	if f == nil {
		return vmtracer.CallStatus{}
	}

	result := f.status
	for parent := f.parent; parent != 0; parent = c.calls[parent].parent {
		if c.calls[parent].status.Reverted {
			result.Reverted = true
			result.Success = false
			break
		}
	}

	return result
}

// CurrentCall returns the active call's facts without requiring caller-side
// stack reconstruction. It does not infer AA evidence.
func (c *txTraceCollector) CurrentCall() (vmtracer.Observation, bool) {
	if c == nil {
		return vmtracer.Observation{}, false
	}

	f := c.calls[c.currentObservationCall()]
	if f == nil || f.index >= len(c.events) {
		return vmtracer.Observation{}, false
	}

	e := cloneObservation(c.events[f.index])
	e.Status = c.status(e.CallID)

	return e, true
}

// Observe subscribes to the same observations stored in the journal. It can be
// installed at a precompile boundary; no separate collector or identity exists.
func (c *txTraceCollector) Observe(fn vmtracer.Observer) (func(), error) {
	if c == nil || fn == nil {
		return nil, errors.New("observations not enabled or nil observer")
	}

	if c.err != nil {
		c.LatchError(c.err)
		return nil, c.err
	}

	c.nextSubscriber++
	id := c.nextSubscriber
	c.subscribers = append(c.subscribers, observationSubscriber{
		id: id,
		fn: fn,
	})

	return func() {
		for i, sub := range c.subscribers {
			if sub.id == id {
				c.subscribers = append(c.subscribers[:i], c.subscribers[i+1:]...)
				return
			}
		}
	}, nil
}

func (c *txTraceCollector) Observations() []vmtracer.Observation {
	if c == nil {
		return nil
	}

	out := make([]vmtracer.Observation, len(c.events))

	for i, e := range c.events {
		out[i] = cloneObservation(e)
		out[i].Status = c.status(e.CallID)
	}

	return out
}

func (c *txTraceCollector) currentObservationCall() uint64 {
	if c != nil {
		for _, v := range slices.Backward(c.stack) {
			if v != 0 {
				return v
			}
		}
	}

	return 0
}

func (c *txTraceCollector) emitObservation(e vmtracer.Observation) {
	if c == nil {
		return
	}

	c.nextObservationID++
	if uint64(len(c.events)) >= c.maxEvents {
		c.collectionError(errors.New("raw observation budget exceeded"))
		return
	}

	e.ObservationID = c.nextObservationID
	if e.CallID == 0 {
		e.CallID = c.currentObservationCall()
	}

	if f := c.calls[e.CallID]; f != nil && f.index < len(c.events) {
		enter := c.events[f.index]
		e.ParentCallID = f.parent
		e.Depth = enter.Depth
		e.ExecutionAddress = enter.ExecutionAddress
		e.CodeAddress = enter.CodeAddress
		e.CodeAddressKnown = enter.CodeAddressKnown
	}

	e.Status = c.status(e.CallID)
	e = cloneObservation(e)
	c.events = append(c.events, e)
	subscribers := append([]observationSubscriber(nil), c.subscribers...)

	for _, sub := range subscribers {
		c.LatchError(sub.fn(cloneObservation(e)))
	}
}

func (c *txTraceCollector) onEnter(depth int, typ byte, from, to common.Address, input []byte, gas uint64, value *big.Int) {
	if c == nil {
		return
	}

	// geth's SELFDESTRUCT enter/exit pair is synthetic, not a call.
	// Attribute the executed movement to the still-active enclosing call so the
	// exported call tree contains only CALL/CREATE-family entries.
	if typ == byte(vm.SELFDESTRUCT) {
		if c.currentObservationCall() == 0 {
			c.collectionError(errors.New("selfdestruct outside recorded call"))
		}

		if from != to && value != nil && value.Sign() > 0 {
			c.emitObservation(vmtracer.Observation{
				Kind:   vmtracer.NativeTransfer,
				From:   from,
				To:     to,
				Amount: value,
				Opcode: typ,
			})
		}

		c.stack = append(c.stack, 0)
		return
	}

	parent := c.currentObservationCall()
	c.nextCall++
	e := vmtracer.Observation{
		ObservationID:    c.nextObservationID + 1,
		Kind:             vmtracer.CallEnter,
		Depth:            depth,
		CallID:           c.nextCall,
		ParentCallID:     parent,
		Opcode:           typ,
		Caller:           from,
		Target:           to,
		ExecutionAddress: to,
		CodeAddress:      to,
		Input:            input,
		Gas:              gas,
		Value:            value,
	}

	if typ == byte(vm.DELEGATECALL) || typ == byte(vm.CALLCODE) {
		e.ExecutionAddress = from
	}

	switch {
	case typ == byte(vm.CREATE) || typ == byte(vm.CREATE2):
		e.CodeAddressKnown = true

	case c.code != nil:
		e.CodeAddressKnown = true
		if addr, ok := ethtypes.ParseDelegation(c.code(to)); ok {
			e.CodeAddress = addr
		}

	default:
		c.collectionError(errors.New("code reader unavailable at call entry"))
	}

	c.calls[e.CallID] = &observationCall{
		index:  len(c.events),
		parent: parent,
	}
	c.stack = append(c.stack, e.CallID)
	c.emitObservation(e)

	if typ == byte(vm.CALL) && len(input) >= 4 && (bytes.Equal(input[:4], erc20TransferSelector) || bytes.Equal(input[:4], erc20TransferFromSelector)) {
		c.emitObservation(vmtracer.Observation{
			Kind:   vmtracer.ERC20Call,
			Caller: from,
			Target: to,
			Input:  input,
			Opcode: typ,
		})
	}
}

func (c *txTraceCollector) onExit(_ int, output []byte, gas uint64, err error, reverted bool) {
	if c == nil {
		return
	}

	if len(c.stack) > 0 && c.stack[len(c.stack)-1] == 0 {
		c.stack = c.stack[:len(c.stack)-1]
		return
	}

	id := c.currentObservationCall()
	if id == 0 {
		c.collectionError(errors.New("call exit without call"))
		return
	}

	f := c.calls[id]
	f.status = vmtracer.CallStatus{
		Exited:   true,
		Success:  err == nil && !reverted,
		Reverted: reverted,
		Error:    err,
	}

	c.emitObservation(vmtracer.Observation{
		Kind:     vmtracer.CallExit,
		CallID:   id,
		Output:   output,
		Gas:      gas,
		Err:      err,
		Reverted: reverted,
	})

	c.stack = c.stack[:len(c.stack)-1]
}

// RecordNativeTransfer is called only after the EVM Transfer function executes.
// Balance callbacks are separate facts and are never tuple-deduplicated or paired.
func (c *txTraceCollector) RecordNativeTransfer(from, to common.Address, value *big.Int) {
	if c == nil || value == nil || value.Sign() == 0 {
		return
	}

	id := c.currentObservationCall()
	f := c.calls[id]
	if f == nil || f.index >= len(c.events) {
		c.collectionError(errors.New("movement outside recorded call"))
		return
	}

	opcode := c.events[f.index].Opcode
	if opcode != byte(vm.CALL) && opcode != byte(vm.CREATE) && opcode != byte(vm.CREATE2) {
		c.collectionError(fmt.Errorf("movement in unsupported opcode %x", opcode))
		return
	}

	c.emitObservation(vmtracer.Observation{
		Kind:   vmtracer.NativeTransfer,
		From:   from,
		To:     to,
		Amount: value,
		Opcode: opcode,
	})
}

func cloneObservation(e vmtracer.Observation) vmtracer.Observation {
	e.Input = append([]byte(nil), e.Input...)
	e.Output = append([]byte(nil), e.Output...)
	if e.Value != nil {
		e.Value = new(big.Int).Set(e.Value)
	}
	if e.PreviousBalance != nil {
		e.PreviousBalance = new(big.Int).Set(e.PreviousBalance)
	}
	if e.CurrentBalance != nil {
		e.CurrentBalance = new(big.Int).Set(e.CurrentBalance)
	}
	if e.Amount != nil {
		e.Amount = new(big.Int).Set(e.Amount)
	}
	if e.UserOperation != nil {
		u := *e.UserOperation
		e.UserOperation = &u
	}
	if e.Log != nil {
		l := *e.Log
		l.Topics = append([]common.Hash(nil), l.Topics...)
		l.Data = append([]byte(nil), l.Data...)
		e.Log = &l
	}
	return e
}
