package types

import (
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/ethereum/go-ethereum/params"
)

const (
	TracerAccessList = "access_list"
	TracerJSON       = "json"
	TracerStruct     = "struct"
	TracerMarkdown   = "markdown"
)

// NewTracer creates a new Logger tracer to collect execution traces from an
// EVM transaction.
func NewTracer(tracer string, msg core.Message, cfg *params.ChainConfig, height int64, timestamp uint64) *tracing.Hooks {
	// TODO: enable additional log configuration
	logCfg := &logger.Config{}

	switch tracer {
	case TracerAccessList:
		blockAddrs := make(map[common.Address]struct{})
		blockAddrs[msg.From] = struct{}{}
		if msg.To != nil {
			blockAddrs[*msg.To] = struct{}{}
		}
		precompiles := vm.ActivePrecompiles(cfg.Rules(big.NewInt(height), cfg.MergeNetsplitBlock != nil, timestamp))
		for _, addr := range precompiles {
			blockAddrs[addr] = struct{}{}
		}
		return logger.NewAccessListTracer(msg.AccessList, blockAddrs).Hooks()
	case TracerJSON:
		return logger.NewJSONLogger(logCfg, os.Stderr)
	case TracerMarkdown:
		return logger.NewMarkdownLogger(logCfg, os.Stdout).Hooks() // TODO: Stderr ?
	case TracerStruct:
		return logger.NewStructLogger(logCfg).Hooks()
	default:
		return NewNoOpTracer()
	}
}

// TxTraceResult is the result of a single transaction trace during a block trace.
type TxTraceResult struct {
	Result interface{} `json:"result,omitempty"` // Trace results produced by the tracer
	Error  string      `json:"error,omitempty"`  // Trace failure produced by the tracer
}

// NoOpTracer is an empty implementation of vm.Tracer interface
type NoOpTracer struct{}

// NewNoOpTracer creates a no-op vm.Tracer
func NewNoOpTracer() *tracing.Hooks {
	t := NoOpTracer{}
	return &tracing.Hooks{
		OnTxStart:        t.OnTxStart,
		OnTxEnd:          t.OnTxEnd,
		OnEnter:          t.OnEnter,
		OnExit:           t.OnExit,
		OnOpcode:         t.OnOpcode,
		OnFault:          t.OnFault,
		OnGasChange:      t.OnGasChange,
		OnBlockchainInit: t.OnBlockchainInit,
		OnBlockStart:     t.OnBlockStart,
		OnBlockEnd:       t.OnBlockEnd,
		OnSkippedBlock:   t.OnSkippedBlock,
		OnGenesisBlock:   t.OnGenesisBlock,
		OnBalanceChange:  t.OnBalanceChange,
		OnNonceChange:    t.OnNonceChange,
		OnCodeChange:     t.OnCodeChange,
		OnStorageChange:  t.OnStorageChange,
		OnLog:            t.OnLog,
	}
}

func (dt *NoOpTracer) OnOpcode(_ uint64, _ byte, _, _ uint64, _ tracing.OpContext, _ []byte, _ int, _ error) {
}

func (dt *NoOpTracer) OnFault(_ uint64, _ byte, _, _ uint64, _ tracing.OpContext, _ int, _ error) {
}

func (dt *NoOpTracer) OnEnter(_ int, _ byte, _ common.Address, _ common.Address, _ []byte, _ uint64,
	_ *big.Int) {
}

func (dt *NoOpTracer) OnExit(_ int, _ []byte, _ uint64, _ error, _ bool) {
}

func (dt *NoOpTracer) OnTxStart(_ *tracing.VMContext, _ *types.Transaction, _ common.Address) {
}

func (dt *NoOpTracer) OnTxEnd(_ *types.Receipt, _ error) {
}

func (dt *NoOpTracer) OnBlockStart(_ tracing.BlockEvent) {
}

func (dt *NoOpTracer) OnBlockEnd(_ error) {
}

func (dt *NoOpTracer) OnSkippedBlock(_ tracing.BlockEvent) {}

func (dt *NoOpTracer) OnBlockchainInit(_ *params.ChainConfig) {
}

func (dt *NoOpTracer) OnGenesisBlock(_ *types.Block, _ types.GenesisAlloc) {
}

func (dt *NoOpTracer) OnBalanceChange(_ common.Address, _, _ *big.Int, _ tracing.BalanceChangeReason) {
}

func (dt *NoOpTracer) OnNonceChange(_ common.Address, _, _ uint64) {
}

func (dt *NoOpTracer) OnCodeChange(_ common.Address, _ common.Hash, _ []byte, _ common.Hash,
	_ []byte) {
}

func (dt *NoOpTracer) OnStorageChange(_ common.Address, _, _, _ common.Hash) {
}

func (dt *NoOpTracer) OnLog(_ *types.Log) {
}

func (dt *NoOpTracer) OnGasChange(_, _ uint64, _ tracing.GasChangeReason) {
}
