package application

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	vmtracer "github.com/cosmos/evm/x/vm/tracer"
)

func transferLog(asset, from, to common.Address, amount int64) *ethtypes.Log {
	return &ethtypes.Log{
		Address: asset,
		Topics:  []common.Hash{crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")), common.BytesToHash(from.Bytes()), common.BytesToHash(to.Bytes())},
		Data:    common.LeftPadBytes(big.NewInt(amount).Bytes(), 32),
	}
}

func TestRequestsNativeTransfersAndLogsStayDistinct(t *testing.T) {
	m, c := rawManager(t, 100)
	from, to, token := common.HexToAddress("0xaa"), common.HexToAddress("0xbb"), common.HexToAddress("0xcc")
	input := append([]byte{0xa9, 5, 0x9c, 0xbb}, common.LeftPadBytes(to.Bytes(), 32)...)
	input = append(input, common.LeftPadBytes(big.NewInt(7).Bytes(), 32)...)
	m.Hooks().OnEnter(0, byte(vm.CALL), from, token, input, 100, big.NewInt(7))
	m.RecordNativeTransfer(from, to, big.NewInt(7))
	log := transferLog(token, from, to, 7)
	m.Hooks().OnLog(log)
	m.Hooks().OnLog(log)
	m.Hooks().OnExit(0, nil, 0, vm.ErrExecutionReverted, true)
	facts := c.Observations()
	kinds := []vmtracer.ObservationKind{vmtracer.CallEnter, vmtracer.ERC20Call, vmtracer.NativeTransfer, vmtracer.Log, vmtracer.Log, vmtracer.CallExit}
	require.Len(t, facts, len(kinds))
	for i, e := range facts {
		require.Equal(t, kinds[i], e.Kind)
		require.Equal(t, uint64(i+1), e.ObservationID)
		require.Equal(t, uint64(1), e.CallID)
		require.True(t, e.Status.Reverted)
	}
	require.Equal(t, input, facts[1].Input)
	require.Equal(t, from, facts[1].Caller)
	require.Equal(t, token, facts[1].Target)
	require.Equal(t, from, facts[2].From)
	require.Equal(t, to, facts[2].To)
	require.Equal(t, int64(7), facts[2].Amount.Int64())
	require.NotEqual(t, facts[3].ObservationID, facts[4].ObservationID)
	assertMarooTraceStructure(t, facts)
	facts[1].Input[0] = 0
	facts[2].Amount.SetInt64(99)
	facts[3].Log.Data[0] = 99
	input[0] = 0
	log.Data[0] = 99
	require.Equal(t, byte(0xa9), c.Observations()[1].Input[0])
	require.Equal(t, int64(7), c.Observations()[2].Amount.Int64())
	require.Equal(t, byte(0), c.Observations()[3].Log.Data[0])
	require.NoError(t, c.Error())
}

func TestTokenAnnotationsOnlyBelongToCalls(t *testing.T) {
	for _, op := range []vm.OpCode{vm.CALL, vm.DELEGATECALL, vm.CALLCODE, vm.STATICCALL, vm.CREATE, vm.CREATE2} {
		t.Run(op.String(), func(t *testing.T) {
			m, c := rawManager(t, 100)
			input := []byte{0xa9, 5, 0x9c, 0xbb}
			m.Hooks().OnEnter(0, byte(op), common.Address{}, common.Address{}, input, 100, big.NewInt(7))
			m.Hooks().OnExit(0, nil, 0, nil, false)
			facts := c.Observations()
			count := 2
			if op == vm.CALL {
				count = 3
				require.Equal(t, vmtracer.ERC20Call, facts[1].Kind)
				require.Equal(t, input, facts[1].Input)
			}
			require.Len(t, facts, count)
			require.Empty(t, nativeRecords(facts))
			require.NoError(t, c.Error())
			assertMarooTraceStructure(t, facts)
		})
	}
}

func TestLogsRemainUninterpreted(t *testing.T) {
	m, c := rawManager(t, 100)
	from, to, token := common.HexToAddress("0xaa"), common.HexToAddress("0xbb"), common.HexToAddress("0xcc")
	m.Hooks().OnEnter(0, byte(vm.CALL), from, token, nil, 100, nil)
	malformed := transferLog(token, from, to, 7)
	malformed.Data = []byte{7}
	nonfungible := transferLog(token, from, to, 7)
	nonfungible.Topics = append(nonfungible.Topics, common.HexToHash("0x1"))
	nonfungible.Data = nil
	logs := []*ethtypes.Log{transferLog(token, from, to, 7), malformed, nonfungible, ethtypes.EthTransferLog(from, to, uint256.NewInt(7)), transferLog(token, common.Address{}, to, 5), transferLog(token, to, common.Address{}, 3)}
	for _, log := range logs {
		m.Hooks().OnLog(log)
	}
	m.Hooks().OnExit(0, nil, 0, nil, false)
	facts := c.Observations()
	require.Len(t, facts, len(logs)+2)
	require.Empty(t, nativeRecords(facts))
	for i, log := range logs {
		require.Equal(t, vmtracer.Log, facts[i+1].Kind)
		require.Equal(t, log, facts[i+1].Log)
	}
	require.NoError(t, c.Error())
	assertMarooTraceStructure(t, facts)
}

func TestSelfDestructMovementUsesEnclosingCall(t *testing.T) {
	m, c := rawManager(t, 100)
	from, to := common.HexToAddress("0xaa"), common.HexToAddress("0xbb")
	m.Hooks().OnEnter(0, byte(vm.CALL), to, from, nil, 100, nil)
	m.Hooks().OnEnter(1, byte(vm.SELFDESTRUCT), from, to, nil, 0, big.NewInt(7))
	m.Hooks().OnExit(1, nil, 0, nil, false)
	call, ok := c.CurrentCall()
	require.True(t, ok)
	require.Equal(t, uint64(1), call.CallID)
	m.Hooks().OnExit(0, nil, 0, nil, false)
	facts := c.Observations()
	require.Len(t, facts, 3)
	require.Equal(t, vmtracer.NativeTransfer, facts[1].Kind)
	require.Equal(t, byte(vm.SELFDESTRUCT), facts[1].Opcode)
	require.Equal(t, uint64(1), facts[1].CallID)
	require.Equal(t, from, facts[1].From)
	require.Equal(t, to, facts[1].To)
	require.NoError(t, c.Error())
	assertMarooTraceStructure(t, facts)
}

func TestCollectorErrorsAreSeparateFromEnforcement(t *testing.T) {
	c := NewCollector()
	c.Hooks().OnEnter(0, byte(vm.CALL), common.Address{}, common.Address{}, nil, 100, nil)
	require.ErrorContains(t, c.Error(), "code reader")
	require.NoError(t, c.ExecutionError())
	rejection := vm.ErrExecutionReverted
	c.LatchError(rejection)
	require.ErrorIs(t, c.ExecutionError(), rejection)
	c.Hooks().OnBalanceChange(common.Address{}, big.NewInt(1), big.NewInt(2), tracing.BalanceChangeTransfer)
	c.Hooks().OnExit(0, nil, 0, nil, false)
	require.ErrorIs(t, c.Error(), rejection)
}

func nativeRecords(events []vmtracer.Observation) []vmtracer.Observation {
	var out []vmtracer.Observation
	for _, e := range events {
		if e.Kind == vmtracer.NativeTransfer {
			out = append(out, e)
		}
	}
	return out
}

// Mirror the consumer's structural requirements: one request annotation per
// CALL call, identical calldata, active-call ownership and distinct nonzero
// observation IDs. The adapter can retain every ID instead of synthesizing one.
func assertMarooTraceStructure(t *testing.T, events []vmtracer.Observation) {
	t.Helper()
	calls := map[uint64]vmtracer.Observation{}
	annotated := map[uint64]bool{}
	seen := map[uint64]bool{}
	var stack []uint64
	for _, e := range events {
		require.NotZero(t, e.ObservationID)
		require.False(t, seen[e.ObservationID])
		seen[e.ObservationID] = true
		if e.Kind == vmtracer.BalanceChange {
			continue
		} // diagnostic, not an asset observation
		if e.Kind == vmtracer.CallEnter {
			parent := uint64(0)
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			require.Equal(t, parent, e.ParentCallID)
			require.Contains(t, []byte{byte(vm.CALL), byte(vm.DELEGATECALL), byte(vm.CALLCODE), byte(vm.STATICCALL), byte(vm.CREATE), byte(vm.CREATE2)}, e.Opcode)
			calls[e.CallID] = e
			stack = append(stack, e.CallID)
			continue
		}
		require.NotEmpty(t, stack)
		require.Equal(t, stack[len(stack)-1], e.CallID)
		switch e.Kind {
		case vmtracer.ERC20Call:
			enter := calls[e.CallID]
			require.Equal(t, byte(vm.CALL), enter.Opcode)
			require.False(t, annotated[e.CallID])
			annotated[e.CallID] = true
			require.Equal(t, enter.Caller, e.Caller)
			require.Equal(t, enter.Target, e.Target)
			require.True(t, bytes.Equal(enter.Input, e.Input))
		case vmtracer.NativeTransfer:
			require.NotNil(t, e.Amount)
		case vmtracer.Log:
			require.NotNil(t, e.Log)
		case vmtracer.CallExit:
			stack = stack[:len(stack)-1]
		default:
			t.Fatalf("unsupported input kind %s", e.Kind)
		}
	}
	require.Empty(t, stack)
}
