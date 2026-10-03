package tracer

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ExecutionInfo identifies one top-level EVM execution.
type ExecutionInfo struct {
	Message core.Message
	TxHash  common.Hash
	TxIndex uint64
	Commit  bool
}

// ApplicationTracerFactory creates a tracer for one top-level EVM execution.
// It may return a derived context to expose per-execution state to post-tx
// hooks. A new tracer is created for every traced execution.
type ApplicationTracerFactory func(ctx sdk.Context, execution ExecutionInfo) (sdk.Context, Tracer)

// InstallApplication installs a tracer that observes the whole EVM execution.
// Application tracers normally live until the Manager is discarded.
func (m *Manager) InstallApplication(tracer Tracer) func() {
	return m.install(tracer)
}
