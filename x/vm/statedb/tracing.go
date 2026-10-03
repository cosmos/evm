package statedb

import vmtracer "github.com/cosmos/evm/x/vm/tracer"

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
