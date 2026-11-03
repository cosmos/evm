package statedb

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/holiman/uint256"

	vmtracer "github.com/cosmos/evm/x/vm/tracer"
)

// AttachTracerManager makes manager available to precompiles without replacing
// other execution-scoped values in the StateDB contexts. If the cache context
// has already been created, it is updated as well. A nil manager clears any
// manager left by a previous execution.
func (s *StateDB) AttachTracerManager(manager *vmtracer.Manager) {
	if manager == nil {
		if _, ok := vmtracer.FromContext(s.ctx); ok {
			s.ctx = vmtracer.WithoutManager(s.ctx)
		}
		if s.writeCache != nil {
			if _, ok := vmtracer.FromContext(s.cacheCtx); ok {
				s.cacheCtx = vmtracer.WithoutManager(s.cacheCtx)
			}
		}
		return
	}
	s.ctx = vmtracer.WithManager(s.ctx, manager)
	if s.writeCache != nil {
		s.cacheCtx = vmtracer.WithManager(s.cacheCtx, manager)
	}
}

// Rollback mutates the StateDB journal directly and does not emit fresh balance
// or log observations. Call exit observations carry the rollback decision.
func (s *StateDB) tracingHooks() *tracing.Hooks {
	if manager, ok := vmtracer.FromContext(s.ctx); ok {
		return manager.Hooks()
	}
	return nil
}

func (s *StateDB) traceBalance(addr common.Address, previous, current *uint256.Int, reason tracing.BalanceChangeReason) {
	if previous.Eq(current) {
		return
	}
	if hooks := s.tracingHooks(); hooks != nil && hooks.OnBalanceChange != nil {
		hooks.OnBalanceChange(addr, previous.ToBig(), current.ToBig(), reason)
	}
}
