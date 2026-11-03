package tracer

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

// Manager dispatches EVM tracing events to a base tracer and application tracers.
// A Manager belongs to one execution and is not safe for concurrent use.
type Manager struct {
	hooks       *tracing.Hooks
	nestedHooks *tracing.Hooks
	base        *tracing.Hooks

	// Each OnEnter pushes its VM-reported depth. The stack length determines
	// the logical depth across both native reentry and nested VM instances.
	callDepths     []int
	journal        Journal
	observationErr error

	nextID       uint64
	applications []installedHooks
}

type installedHooks struct {
	id    uint64
	hooks tracing.Hooks
}

// New creates a tracing manager. base is invoked before application
// tracers and may be nil. The returned Hooks pointer remains stable for the
// lifetime of the manager.
func New(base *tracing.Hooks) *Manager {
	m := &Manager{
		hooks:       &tracing.Hooks{},
		nestedHooks: &tracing.Hooks{},
	}
	if base != nil {
		baseCopy := *base
		m.base = &baseCopy
	}
	m.rebuild()
	return m
}

// Hooks returns the stable dispatcher hooks to install in vm.Config.Tracer.
// Depth-bearing callbacks use the logical call tree, including native calls
// that reenter the same VM without incrementing its reported depth.
func (m *Manager) Hooks() *tracing.Hooks {
	if m == nil {
		return nil
	}
	return m.hooks
}

// NestedHooks returns a stable dispatcher for a nested EVM execution. It
// dispatches the same call, opcode, and state events as Hooks, but omits the
// transaction boundary owned by the enclosing execution.
func (m *Manager) NestedHooks() *tracing.Hooks {
	if m == nil {
		return nil
	}
	return m.nestedHooks
}

// HasActiveCall reports whether the manager is tracing an EVM call.
// A manager can remain in a StateDB context after its execution has ended.
func (m *Manager) HasActiveCall() bool {
	return m != nil && len(m.callDepths) > 0
}

func (m *Manager) install(tracer Tracer) func() {
	if m == nil || tracer == nil {
		return func() {}
	}

	if journal, ok := tracer.(Journal); ok {
		if m.HasActiveCall() || m.journal != nil || journal.LastObservationID() != 0 {
			m.LatchError(errors.New("execution journal must be installed once before execution"))
			return func() {}
		}
		m.journal = journal
		journal.LatchError(m.observationErr)
		m.observationErr = nil
	}

	hooks := tracer.Hooks()
	if hooks == nil {
		return func() {}
	}

	m.nextID++
	entry := installedHooks{
		id:    m.nextID,
		hooks: *hooks,
	}
	m.applications = append(m.applications, entry)
	m.rebuild()

	closed := false
	return func() {
		if closed {
			return
		}
		closed = true
		m.remove(entry.id)
		if _, ok := tracer.(Journal); ok {
			m.journal = nil
		}
	}
}

func (m *Manager) remove(id uint64) {
	entries := &m.applications

	for i := range *entries {
		if (*entries)[i].id != id {
			continue
		}
		copy((*entries)[i:], (*entries)[i+1:])
		*entries = (*entries)[:len(*entries)-1]
		m.rebuild()
		return
	}
}

func (m *Manager) rebuild() {
	ordered := make([]tracing.Hooks, 0, 1+len(m.applications))
	if m.base != nil {
		ordered = append(ordered, *m.base)
	}
	for _, entry := range m.applications {
		ordered = append(ordered, entry.hooks)
	}

	composed := m.withCallTracking(compose(ordered))
	*m.hooks = composed
	composed.OnTxStart = nil
	composed.OnTxEnd = nil
	*m.nestedHooks = composed
}

func (m *Manager) withCallTracking(hooks tracing.Hooks) tracing.Hooks {
	out := hooks
	out.OnEnter = func(depth int, typ byte, from, to common.Address, input []byte, gas uint64, value *big.Int) {
		logicalDepth := len(m.callDepths)
		m.callDepths = append(m.callDepths, depth)
		if hooks.OnEnter != nil {
			hooks.OnEnter(logicalDepth, typ, from, to, input, gas, value)
		}
	}
	out.OnExit = func(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
		if m.HasActiveCall() {
			call := len(m.callDepths) - 1
			depth = call
			defer func() { m.callDepths = m.callDepths[:call] }()
		}
		if hooks.OnExit != nil {
			hooks.OnExit(depth, output, gasUsed, err, reverted)
		}
	}
	if hooks.OnOpcode != nil {
		out.OnOpcode = func(pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, returnData []byte, depth int, err error) {
			hooks.OnOpcode(pc, op, gas, cost, scope, returnData, depth+m.callDepthOffset(), err)
		}
	}
	if hooks.OnFault != nil {
		out.OnFault = func(pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, depth int, err error) {
			hooks.OnFault(pc, op, gas, cost, scope, depth+m.callDepthOffset(), err)
		}
	}
	return out
}

// Opcode and fault depths are relative to the active VM call (normally one
// greater than OnEnter's depth). Preserve that relationship in the logical tree.
func (m *Manager) callDepthOffset() int {
	if !m.HasActiveCall() {
		return 0
	}
	call := len(m.callDepths) - 1
	return call - m.callDepths[call]
}

func compose(hooks []tracing.Hooks) tracing.Hooks {
	var out tracing.Hooks

	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnTxStart != nil }) {
		out.OnTxStart = func(vm *tracing.VMContext, tx *ethtypes.Transaction, from common.Address) {
			for i := range hooks {
				if fn := hooks[i].OnTxStart; fn != nil {
					fn(vm, tx, from)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnTxEnd != nil }) {
		out.OnTxEnd = func(receipt *ethtypes.Receipt, err error) {
			for i := range hooks {
				if fn := hooks[i].OnTxEnd; fn != nil {
					fn(receipt, err)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnEnter != nil }) {
		out.OnEnter = func(depth int, typ byte, from, to common.Address, input []byte, gas uint64, value *big.Int) {
			for i := range hooks {
				if fn := hooks[i].OnEnter; fn != nil {
					fn(depth, typ, from, to, input, gas, value)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnExit != nil }) {
		out.OnExit = func(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
			for i := range hooks {
				if fn := hooks[i].OnExit; fn != nil {
					fn(depth, output, gasUsed, err, reverted)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnOpcode != nil }) {
		out.OnOpcode = func(pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, returnData []byte, depth int, err error) {
			for i := range hooks {
				if fn := hooks[i].OnOpcode; fn != nil {
					fn(pc, op, gas, cost, scope, returnData, depth, err)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnFault != nil }) {
		out.OnFault = func(pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, depth int, err error) {
			for i := range hooks {
				if fn := hooks[i].OnFault; fn != nil {
					fn(pc, op, gas, cost, scope, depth, err)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnGasChange != nil }) {
		out.OnGasChange = func(oldGas, newGas uint64, reason tracing.GasChangeReason) {
			for i := range hooks {
				if fn := hooks[i].OnGasChange; fn != nil {
					fn(oldGas, newGas, reason)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnBlockchainInit != nil }) {
		out.OnBlockchainInit = func(config *params.ChainConfig) {
			for i := range hooks {
				if fn := hooks[i].OnBlockchainInit; fn != nil {
					fn(config)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnClose != nil }) {
		out.OnClose = func() {
			for i := range hooks {
				if fn := hooks[i].OnClose; fn != nil {
					fn()
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnBlockStart != nil }) {
		out.OnBlockStart = func(event tracing.BlockEvent) {
			for i := range hooks {
				if fn := hooks[i].OnBlockStart; fn != nil {
					fn(event)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnBlockEnd != nil }) {
		out.OnBlockEnd = func(err error) {
			for i := range hooks {
				if fn := hooks[i].OnBlockEnd; fn != nil {
					fn(err)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnSkippedBlock != nil }) {
		out.OnSkippedBlock = func(event tracing.BlockEvent) {
			for i := range hooks {
				if fn := hooks[i].OnSkippedBlock; fn != nil {
					fn(event)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnGenesisBlock != nil }) {
		out.OnGenesisBlock = func(genesis *ethtypes.Block, alloc ethtypes.GenesisAlloc) {
			for i := range hooks {
				if fn := hooks[i].OnGenesisBlock; fn != nil {
					fn(genesis, alloc)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnSystemCallStart != nil }) {
		out.OnSystemCallStart = func() {
			for i := range hooks {
				if fn := hooks[i].OnSystemCallStart; fn != nil {
					fn()
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnSystemCallStartV2 != nil }) {
		out.OnSystemCallStartV2 = func(vm *tracing.VMContext) {
			for i := range hooks {
				if fn := hooks[i].OnSystemCallStartV2; fn != nil {
					fn(vm)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnSystemCallEnd != nil }) {
		out.OnSystemCallEnd = func() {
			for i := range hooks {
				if fn := hooks[i].OnSystemCallEnd; fn != nil {
					fn()
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnStateUpdate != nil }) {
		out.OnStateUpdate = func(update *tracing.StateUpdate) {
			for i := range hooks {
				if fn := hooks[i].OnStateUpdate; fn != nil {
					fn(update)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnBalanceChange != nil }) {
		out.OnBalanceChange = func(address common.Address, previous, current *big.Int, reason tracing.BalanceChangeReason) {
			for i := range hooks {
				if fn := hooks[i].OnBalanceChange; fn != nil {
					fn(address, previous, current, reason)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnNonceChange != nil }) {
		out.OnNonceChange = func(address common.Address, previous, current uint64) {
			for i := range hooks {
				if fn := hooks[i].OnNonceChange; fn != nil {
					fn(address, previous, current)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnNonceChangeV2 != nil }) {
		out.OnNonceChangeV2 = func(address common.Address, previous, current uint64, reason tracing.NonceChangeReason) {
			for i := range hooks {
				if fn := hooks[i].OnNonceChangeV2; fn != nil {
					fn(address, previous, current, reason)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnCodeChange != nil }) {
		out.OnCodeChange = func(address common.Address, previousHash common.Hash, previousCode []byte, hash common.Hash, code []byte) {
			for i := range hooks {
				if fn := hooks[i].OnCodeChange; fn != nil {
					fn(address, previousHash, previousCode, hash, code)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnCodeChangeV2 != nil }) {
		out.OnCodeChangeV2 = func(address common.Address, previousHash common.Hash, previousCode []byte, hash common.Hash, code []byte, reason tracing.CodeChangeReason) {
			for i := range hooks {
				if fn := hooks[i].OnCodeChangeV2; fn != nil {
					fn(address, previousHash, previousCode, hash, code, reason)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnStorageChange != nil }) {
		out.OnStorageChange = func(address common.Address, slot, previous, current common.Hash) {
			for i := range hooks {
				if fn := hooks[i].OnStorageChange; fn != nil {
					fn(address, slot, previous, current)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnLog != nil }) {
		out.OnLog = func(log *ethtypes.Log) {
			for i := range hooks {
				if fn := hooks[i].OnLog; fn != nil {
					fn(log)
				}
			}
		}
	}
	if anyHook(hooks, func(h tracing.Hooks) bool { return h.OnBlockHashRead != nil }) {
		out.OnBlockHashRead = func(blockNumber uint64, hash common.Hash) {
			for i := range hooks {
				if fn := hooks[i].OnBlockHashRead; fn != nil {
					fn(blockNumber, hash)
				}
			}
		}
	}

	return out
}

func anyHook(hooks []tracing.Hooks, predicate func(tracing.Hooks) bool) bool {
	for i := range hooks {
		if predicate(hooks[i]) {
			return true
		}
	}
	return false
}

// Journal returns the application execution journal, or nil when none is installed.
func (m *Manager) Journal() Journal {
	if m == nil {
		return nil
	}
	return m.journal
}

func (m *Manager) LatchError(err error) {
	if m == nil {
		return
	}
	if m.journal != nil {
		m.journal.LatchError(err)
	} else if m.observationErr == nil {
		m.observationErr = err
	}
}

func (m *Manager) ExecutionError() error {
	if m == nil {
		return nil
	}
	if m.observationErr != nil {
		return m.observationErr
	}
	if m.journal != nil {
		return m.journal.ExecutionError()
	}
	return nil
}

func (m *Manager) SetCodeReader(reader func(common.Address) []byte) func() {
	if m.Journal() == nil {
		return func() {}
	}
	return m.journal.SetCodeReader(reader)
}

func (m *Manager) RecordNativeTransfer(from, to common.Address, value *big.Int) {
	if m.Journal() != nil {
		m.journal.RecordNativeTransfer(from, to, value)
	}
}
