package application

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"

	vmtracer "github.com/cosmos/evm/x/vm/tracer"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestTxTraceContext(t *testing.T) {
	parent := sdk.Context{}.WithContext(context.Background())
	collector := NewCollector()
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

	touches, transfers, erc20Transfers := GetTxTrace(ctx, 7)
	require.Len(t, touches, 1)
	require.Len(t, transfers, 1)
	require.Empty(t, erc20Transfers)

	touches, transfers, erc20Transfers = GetTxTrace(ctx, 8)
	require.Nil(t, touches)
	require.Nil(t, transfers)
	require.Nil(t, erc20Transfers)

	touches, transfers, erc20Transfers = GetTxTrace(parent, 7)
	require.Nil(t, touches)
	require.Nil(t, transfers)
	require.Nil(t, erc20Transfers)
}

func TestTxTraceContextWithNilCollector(t *testing.T) {
	ctx := sdk.Context{}.WithContext(context.Background())
	ctx = WithCollector(ctx, 0, nil)

	touches, transfers, erc20Transfers := GetTxTrace(ctx, 0)
	require.Nil(t, touches)
	require.Nil(t, transfers)
	require.Nil(t, erc20Transfers)
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

	touches, transfers, erc20Transfers := GetTxTrace(ctx, 3)
	require.Len(t, touches, 1)
	require.Len(t, transfers, 1)
	require.Empty(t, erc20Transfers)
}
