package application

import (
	"context"
	"math/big"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"

	vmtracer "github.com/cosmos/evm/x/vm/tracer"
)

func TestTxTraceContext(t *testing.T) {
	parent := sdk.Context{}.WithContext(context.Background())
	collector := NewCollector()
	collector.SetCodeReader(func(common.Address) []byte { return nil })
	ctx := WithCollector(parent, 7, collector)

	collector.Hooks().OnEnter(
		0,
		byte(vm.CALL),
		common.HexToAddress("0x1"),
		common.HexToAddress("0x2"),
		nil,
		0,
		big.NewInt(10),
	)

	trace, err := GetTxTrace(ctx, 7)
	require.NoError(t, err)
	require.Len(t, trace, 1)

	trace, err = GetTxTrace(ctx, 8)
	require.Nil(t, trace)
	require.NoError(t, err)

	trace, err = GetTxTrace(parent, 7)
	require.Nil(t, trace)
	require.NoError(t, err)
}

func TestTxTraceContextWithNilCollector(t *testing.T) {
	ctx := sdk.Context{}.WithContext(context.Background())
	ctx = WithCollector(ctx, 0, nil)

	trace, err := GetTxTrace(ctx, 0)
	require.Nil(t, trace)
	require.NoError(t, err)
}

func TestTxTraceFactory(t *testing.T) {
	ctx := sdk.Context{}.WithContext(context.Background())
	ctx, tracer := TxTraceFactory(ctx, vmtracer.ExecutionInfo{TxIndex: 3})
	require.NotNil(t, tracer)

	tracer.Hooks().OnEnter(
		0,
		byte(vm.CALL),
		common.HexToAddress("0x1"),
		common.HexToAddress("0x2"),
		nil,
		0,
		big.NewInt(1),
	)

	trace, err := GetTxTrace(ctx, 3)
	require.Len(t, trace, 1)
	require.ErrorContains(t, err, "code reader")
}
