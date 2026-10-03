package tracer

import "github.com/ethereum/go-ethereum/core/tracing"

// Tracer is the contract for application-level tracers managed during one EVM
// execution. Concrete implementations own their collection policy and expose
// only the geth hooks needed by Manager.
type Tracer interface {
	Hooks() *tracing.Hooks
}
