package types

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/ethereum/go-ethereum/params"
)

// AccessListTracerName is the name of the native tracer used by eth_createAccessList
// through the TraceCall query. It records the addresses and storage slots accessed by
// the call, like go-ethereum's access list tracer.
const AccessListTracerName = "accessListTracer"

// AccessListTracerConfig is the tracer config of AccessListTracerName.
type AccessListTracerConfig struct {
	// AccessList is the access list the tracer starts from.
	AccessList ethtypes.AccessList `json:"accessList"`
	// Excludes are the addresses that are never added to the access list
	// (sender, recipient, precompiles, authorities).
	Excludes []common.Address `json:"excludes"`
}

// AccessListTracerResult is the result of AccessListTracerName.
type AccessListTracerResult struct {
	AccessList ethtypes.AccessList `json:"accessList"`
	GasUsed    hexutil.Uint64      `json:"gasUsed"`
	// Error is the EVM error of the call, if any.
	Error string `json:"error,omitempty"`
}

func init() {
	tracers.DefaultDirectory.Register(AccessListTracerName, newAccessListTracer, false)
}

func newAccessListTracer(_ *tracers.Context, cfg json.RawMessage, _ *params.ChainConfig) (*tracers.Tracer, error) {
	var config AccessListTracerConfig
	if len(cfg) > 0 {
		if err := json.Unmarshal(cfg, &config); err != nil {
			return nil, err
		}
	}

	excludes := make(map[common.Address]struct{}, len(config.Excludes))
	for _, addr := range config.Excludes {
		excludes[addr] = struct{}{}
	}

	accessListTracer := logger.NewAccessListTracer(config.AccessList, excludes)
	var result AccessListTracerResult
	hooks := accessListTracer.Hooks()
	hooks.OnTxEnd = func(receipt *ethtypes.Receipt, err error) {
		if receipt != nil {
			result.GasUsed = hexutil.Uint64(receipt.GasUsed)
		}
		if err != nil {
			result.Error = err.Error()
		}
	}

	return &tracers.Tracer{
		Hooks: hooks,
		GetResult: func() (json.RawMessage, error) {
			result.AccessList = accessListTracer.AccessList()
			return json.Marshal(result)
		},
		Stop: func(error) {},
	}, nil
}
