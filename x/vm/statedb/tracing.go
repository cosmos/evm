package statedb

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"

	vmtracer "github.com/cosmos/evm/x/vm/tracer"
)

// AttachTracerManager makes manager available to precompiles without replacing
// other execution-scoped values in the StateDB contexts. If the cache context
// has already been created, it is updated as well.
func (s *StateDB) AttachTracerManager(manager *vmtracer.Manager) {
	if manager == nil {
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

func (s *StateDB) traceBalance(addr common.Address, previous, current *big.Int, reason tracing.BalanceChangeReason) {
	if previous.Cmp(current) == 0 {
		return
	}
	if hooks := s.tracingHooks(); hooks != nil && hooks.OnBalanceChange != nil {
		hooks.OnBalanceChange(addr, previous, current, reason)
	}
}
