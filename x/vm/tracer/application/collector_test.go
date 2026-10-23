package application

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func erc20Input(selector []byte, words ...[]byte) []byte {
	input := append([]byte(nil), selector...)
	for _, word := range words {
		input = append(input, common.LeftPadBytes(word, common.HashLength)...)
	}
	return input
}

func TestCollectorCollectsSuccessfulTouchesAndTransfers(t *testing.T) {
	collector := NewCollector()
	hooks := collector.Hooks()
	from := common.HexToAddress("0x1")
	to := common.HexToAddress("0x2")
	rootValue := big.NewInt(10)

	hooks.OnEnter(0, byte(vm.CALL), from, to, nil, 0, rootValue)
	rootValue.SetInt64(99)

	hooks.OnEnter(1, byte(vm.DELEGATECALL), to, from, nil, 0, big.NewInt(7))
	hooks.OnExit(1, nil, 0, nil, false)

	hooks.OnEnter(1, byte(vm.CREATE), to, common.Address{}, nil, 0, big.NewInt(3))
	hooks.OnEnter(2, byte(vm.CALL), to, from, nil, 0, big.NewInt(4))
	hooks.OnExit(2, nil, 0, nil, false)
	hooks.OnExit(1, nil, 0, nil, true)
	hooks.OnExit(0, nil, 0, nil, false)

	touches := collector.Touches()
	require.Len(t, touches, 2)
	require.Equal(t, byte(vm.CALL), touches[0].CallType)
	require.Equal(t, byte(vm.DELEGATECALL), touches[1].CallType)

	transfers := collector.Transfers()
	require.Len(t, transfers, 1)
	require.Equal(t, int64(10), transfers[0].Value.Int64())

	transfers[0].Value.SetInt64(123)
	require.Equal(t, int64(10), collector.Transfers()[0].Value.Int64())
	touches[0].Depth = 42
	require.Equal(t, 0, collector.Touches()[0].Depth)
}

func TestCollectorDropsRevertedRoot(t *testing.T) {
	collector := NewCollector()
	hooks := collector.Hooks()

	hooks.OnEnter(0, byte(vm.CALL), common.Address{}, common.Address{}, nil, 0, big.NewInt(1))
	hooks.OnEnter(1, byte(vm.STATICCALL), common.Address{}, common.Address{}, nil, 0, nil)
	hooks.OnExit(1, nil, 0, nil, false)
	hooks.OnExit(0, nil, 0, nil, true)

	require.Empty(t, collector.Touches())
	require.Empty(t, collector.Transfers())
	require.Empty(t, collector.ERC20Transfers())
}

func TestCollectorCollectsSuccessfulERC20Calls(t *testing.T) {
	collector := NewCollector()
	hooks := collector.Hooks()
	caller := common.HexToAddress("0x1001")
	token := common.HexToAddress("0x2002")
	from := common.HexToAddress("0x3003")
	to := common.HexToAddress("0x4004")
	input := erc20Input(erc20TransferFromSelector, from.Bytes(), to.Bytes(), big.NewInt(9).Bytes())

	// A false ABI return does not represent a successful transfer.
	hooks.OnEnter(0, byte(vm.CALL), caller, token, input, 0, big.NewInt(0))
	hooks.OnExit(0, make([]byte, common.HashLength), 0, nil, false)
	require.Empty(t, collector.ERC20Transfers())

	// A successful nested call is discarded when the enclosing frame reverts.
	hooks.OnEnter(0, byte(vm.CALL), caller, common.HexToAddress("0x5005"), nil, 0, big.NewInt(0))
	hooks.OnEnter(1, byte(vm.CALL), caller, token, input, 0, big.NewInt(0))
	hooks.OnExit(1, nil, 0, nil, false)
	hooks.OnExit(0, nil, 0, nil, true)
	require.Empty(t, collector.ERC20Transfers())

	// Empty return data is accepted for ERC20 implementations that do not return a bool.
	hooks.OnEnter(0, byte(vm.CALL), caller, token, input, 0, big.NewInt(0))
	hooks.OnExit(0, nil, 0, nil, false)
	transfers := collector.ERC20Transfers()
	require.Len(t, transfers, 1)
	require.Equal(t, caller, transfers[0].Caller)
	require.Equal(t, token, transfers[0].Token)
	require.Equal(t, from, transfers[0].From)
	require.Equal(t, to, transfers[0].To)
	require.Equal(t, int64(9), transfers[0].Value.Int64())

	transfers[0].Value.SetInt64(99)
	require.Equal(t, int64(9), collector.ERC20Transfers()[0].Value.Int64())
}

func TestCollectorNilReceiver(t *testing.T) {
	var collector *txTraceCollector
	require.Nil(t, collector.Hooks())
	require.Nil(t, collector.Touches())
	require.Nil(t, collector.Transfers())
	require.Nil(t, collector.ERC20Transfers())
}
