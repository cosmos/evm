package statedb_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/holiman/uint256"

	"github.com/cosmos/evm/x/vm/statedb"
	vmtracer "github.com/cosmos/evm/x/vm/tracer"
)

var benchmarkBalanceBefore, benchmarkBalanceAfter *big.Int

func BenchmarkBalanceTracing(b *testing.B) {
	for _, mode := range []string{"NoManager", "EmptyManager", "BalanceHook"} {
		b.Run(mode, func(b *testing.B) {
			for _, operation := range []string{"Add", "Sub", "Set"} {
				b.Run(operation, func(b *testing.B) {
					ctx := newTestCtx()
					if mode != "NoManager" {
						var base *tracing.Hooks
						if mode == "BalanceHook" {
							base = &tracing.Hooks{OnBalanceChange: func(_ common.Address, previous, current *big.Int, _ tracing.BalanceChangeReason) {
								benchmarkBalanceBefore, benchmarkBalanceAfter = previous, current
							}}
						}
						ctx = vmtracer.WithManager(ctx, vmtracer.New(base))
					}
					db := statedb.New(ctx, NewMockKeeper(), emptyTxConfig)
					db.SetBalance(address, uint256.NewInt(100), tracing.BalanceChangeUnspecified)
					amount := uint256.NewInt(7)
					var mutate func()
					switch operation {
					case "Add":
						mutate = func() { db.AddBalance(address, amount, tracing.BalanceChangeTransfer) }
					case "Sub":
						mutate = func() { db.SubBalance(address, amount, tracing.BalanceChangeTransfer) }
					case "Set":
						mutate = func() { db.SetBalance(address, amount, tracing.BalanceChangeUnspecified) }
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						snapshot := db.Snapshot()
						mutate()
						db.RevertToSnapshot(snapshot)
					}
				})
			}
		})
	}
}
