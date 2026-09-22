package tracer

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

// Manager dispatches EVM tracing events to a base tracer and application-level
// tracers. A Manager belongs to one EVM execution and is not safe for concurrent
// use, matching vm.EVM's concurrency guarantees.
type Manager struct {
	hooks              *tracing.Hooks
	nestedHooks        *tracing.Hooks
	base               *tracing.Hooks
	currentDepth       int
	nestedDepthOffsets []int

	nextID       uint64
	applications []installedHooks
}

type installedHooks struct {
	id    uint64
	hooks tracing.Hooks
}

// New creates a tracing manager. base is invoked before application tracers and
// may be nil. The returned Hooks pointers remain stable for the manager's
// lifetime.
func New(base *tracing.Hooks) *Manager {
	m := &Manager{
		hooks:        &tracing.Hooks{},
		nestedHooks:  &tracing.Hooks{},
		currentDepth: -1,
	}
	if base != nil {
		baseCopy := *base
		m.base = &baseCopy
	}
	m.rebuild()
	return m
}

// Hooks returns the stable dispatcher hooks to install in vm.Config.Tracer.
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

func (m *Manager) install(tracer Tracer) func() {
	if m == nil || tracer == nil {
		return func() {}
	}

	hooks := tracer.Hooks()
	if hooks == nil {
		return func() {}
	}

	m.nextID++
	entry := installedHooks{id: m.nextID, hooks: *hooks}
	m.applications = append(m.applications, entry)
	m.rebuild()

	closed := false
	return func() {
		if closed {
			return
		}
		closed = true
		m.remove(entry.id)
	}
}

func (m *Manager) remove(id uint64) {
	for i := range m.applications {
		if m.applications[i].id != id {
			continue
		}
		copy(m.applications[i:], m.applications[i+1:])
		m.applications = m.applications[:len(m.applications)-1]
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

	composed := compose(ordered)
	*m.hooks = m.withDepthTracking(composed)
	composed.OnTxStart = nil
	composed.OnTxEnd = nil
	*m.nestedHooks = m.withNestedDepthOffset(composed)
}

func (m *Manager) withDepthTracking(hooks tracing.Hooks) tracing.Hooks {
	out := hooks
	out.OnEnter = func(depth int, typ byte, from, to common.Address, input []byte, gas uint64, value *big.Int) {
		m.currentDepth = depth
		if hooks.OnEnter != nil {
			hooks.OnEnter(depth, typ, from, to, input, gas, value)
		}
	}
	out.OnExit = func(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
		if hooks.OnExit != nil {
			hooks.OnExit(depth, output, gasUsed, err, reverted)
		}
		m.currentDepth = depth - 1
	}
	return out
}

func (m *Manager) withNestedDepthOffset(hooks tracing.Hooks) tracing.Hooks {
	out := hooks
	out.OnEnter = func(depth int, typ byte, from, to common.Address, input []byte, gas uint64, value *big.Int) {
		if depth == 0 {
			m.nestedDepthOffsets = append(m.nestedDepthOffsets, m.currentDepth+1)
		}
		depth += m.nestedDepthOffset()
		m.currentDepth = depth
		if hooks.OnEnter != nil {
			hooks.OnEnter(depth, typ, from, to, input, gas, value)
		}
	}
	out.OnExit = func(depth int, output []byte, gasUsed uint64, err error, reverted bool) {
		nestedRoot := depth == 0
		depth += m.nestedDepthOffset()
		if hooks.OnExit != nil {
			hooks.OnExit(depth, output, gasUsed, err, reverted)
		}
		m.currentDepth = depth - 1
		if nestedRoot && len(m.nestedDepthOffsets) > 0 {
			m.nestedDepthOffsets = m.nestedDepthOffsets[:len(m.nestedDepthOffsets)-1]
		}
	}
	if hooks.OnOpcode != nil {
		out.OnOpcode = func(pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, returnData []byte, depth int, err error) {
			hooks.OnOpcode(pc, op, gas, cost, scope, returnData, depth+m.nestedDepthOffset(), err)
		}
	}
	if hooks.OnFault != nil {
		out.OnFault = func(pc uint64, op byte, gas, cost uint64, scope tracing.OpContext, depth int, err error) {
			hooks.OnFault(pc, op, gas, cost, scope, depth+m.nestedDepthOffset(), err)
		}
	}
	return out
}

func (m *Manager) nestedDepthOffset() int {
	if len(m.nestedDepthOffsets) == 0 {
		return 0
	}
	return m.nestedDepthOffsets[len(m.nestedDepthOffsets)-1]
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
