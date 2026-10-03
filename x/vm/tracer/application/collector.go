package application

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"

	vmtracer "github.com/cosmos/evm/x/vm/tracer"
)

// TxCallTouch records a call frame observed during EVM execution.
type TxCallTouch struct {
	From     common.Address
	To       common.Address
	Depth    int
	CallType byte
}

// TxValueTransfer records a native balance transfer observed during EVM execution.
type TxValueTransfer struct {
	From     common.Address
	To       common.Address
	Depth    int
	CallType byte
	Value    *big.Int
}

// TxERC20Transfer records a successful standard ERC20 transfer or transferFrom
// call. It represents the call arguments, not a token-contract-specific
// balance delta.
type TxERC20Transfer struct {
	Caller common.Address
	Token  common.Address
	From   common.Address
	To     common.Address
	Value  *big.Int
}

// Collector is the standard transaction-trace contract. Applications can
// provide their own collection policy while retaining GetTxTrace compatibility.
type Collector interface {
	vmtracer.Tracer
	Touches() []TxCallTouch
	Transfers() []TxValueTransfer
	ERC20Transfers() []TxERC20Transfer
}

// txTraceCollector records transaction-wide call touches, successful native
// balance transfers, and successful standard ERC20 transfer calls. Entries
// created inside reverted frames are discarded.
type txTraceCollector struct {
	touches        []TxCallTouch
	transfers      []TxValueTransfer
	erc20Transfers []TxERC20Transfer

	touchesFrameStart   []int
	transfersFrameStart []int
	erc20Frames         []erc20Frame
}

type erc20Frame struct {
	start    int
	transfer *TxERC20Transfer
}

var (
	erc20TransferSelector     = []byte{0xa9, 0x05, 0x9c, 0xbb}
	erc20TransferFromSelector = []byte{0x23, 0xb8, 0x72, 0xdd}
	zeroABIWord               [common.HashLength]byte
)

// NewCollector creates the default transaction-trace collector.
func NewCollector() Collector {
	return &txTraceCollector{}
}

// Hooks returns the EVM callbacks used by the collector.
func (c *txTraceCollector) Hooks() *tracing.Hooks {
	if c == nil {
		return nil
	}
	return &tracing.Hooks{
		OnEnter: c.onEnter,
		OnExit:  c.onExit,
	}
}

// Touches returns a copy of the collected call touches.
func (c *txTraceCollector) Touches() []TxCallTouch {
	if c == nil {
		return nil
	}
	return append([]TxCallTouch(nil), c.touches...)
}

// Transfers returns a deep copy of the collected native value transfers.
func (c *txTraceCollector) Transfers() []TxValueTransfer {
	if c == nil {
		return nil
	}
	transfers := make([]TxValueTransfer, len(c.transfers))
	for i, transfer := range c.transfers {
		transfers[i] = transfer
		if transfer.Value != nil {
			transfers[i].Value = new(big.Int).Set(transfer.Value)
		}
	}
	return transfers
}

// ERC20Transfers returns a deep copy of collected standard ERC20 transfer calls.
func (c *txTraceCollector) ERC20Transfers() []TxERC20Transfer {
	if c == nil {
		return nil
	}
	transfers := make([]TxERC20Transfer, len(c.erc20Transfers))
	for i, transfer := range c.erc20Transfers {
		transfers[i] = transfer
		if transfer.Value != nil {
			transfers[i].Value = new(big.Int).Set(transfer.Value)
		}
	}
	return transfers
}

func isNativeBalanceTransfer(typ byte, value *big.Int) bool {
	if value == nil || value.Sign() <= 0 {
		return false
	}
	// Only CALL/CREATE/CREATE2 can move native balance between accounts.
	// DELEGATECALL, STATICCALL, and CALLCODE can surface a value in tracing
	// callbacks without transferring it to the callee.
	return typ == byte(vm.CALL) || typ == byte(vm.CREATE) || typ == byte(vm.CREATE2)
}

func (c *txTraceCollector) onEnter(depth int, typ byte, from, to common.Address, input []byte, _ uint64, value *big.Int) {
	c.touchesFrameStart = append(c.touchesFrameStart, len(c.touches))
	c.transfersFrameStart = append(c.transfersFrameStart, len(c.transfers))
	c.erc20Frames = append(c.erc20Frames, erc20Frame{
		start:    len(c.erc20Transfers),
		transfer: parseERC20Transfer(typ, from, to, input),
	})

	c.touches = append(c.touches, TxCallTouch{
		From:     from,
		To:       to,
		Depth:    depth,
		CallType: typ,
	})

	if isNativeBalanceTransfer(typ, value) {
		c.transfers = append(c.transfers, TxValueTransfer{
			From:     from,
			To:       to,
			Depth:    depth,
			CallType: typ,
			Value:    new(big.Int).Set(value),
		})
	}
}

func (c *txTraceCollector) onExit(_ int, output []byte, _ uint64, _ error, reverted bool) {
	if reverted {
		if n := len(c.touchesFrameStart); n > 0 {
			start := c.touchesFrameStart[n-1]
			if start >= 0 && start <= len(c.touches) {
				c.touches = c.touches[:start]
			}
		}
		if n := len(c.transfersFrameStart); n > 0 {
			start := c.transfersFrameStart[n-1]
			if start >= 0 && start <= len(c.transfers) {
				c.transfers = c.transfers[:start]
			}
		}
		if n := len(c.erc20Frames); n > 0 {
			start := c.erc20Frames[n-1].start
			if start >= 0 && start <= len(c.erc20Transfers) {
				c.erc20Transfers = c.erc20Transfers[:start]
			}
		}
	} else if n := len(c.erc20Frames); n > 0 {
		frame := c.erc20Frames[n-1]
		if frame.transfer != nil && !isFalseERC20Return(output) {
			c.erc20Transfers = append(c.erc20Transfers, *frame.transfer)
		}
	}

	if n := len(c.touchesFrameStart); n > 0 {
		c.touchesFrameStart = c.touchesFrameStart[:n-1]
	}
	if n := len(c.transfersFrameStart); n > 0 {
		c.transfersFrameStart = c.transfersFrameStart[:n-1]
	}
	if n := len(c.erc20Frames); n > 0 {
		c.erc20Frames = c.erc20Frames[:n-1]
	}
}

func parseERC20Transfer(typ byte, caller, token common.Address, input []byte) *TxERC20Transfer {
	if typ != byte(vm.CALL) || len(input) < 4 {
		return nil
	}

	switch {
	case bytes.Equal(input[:4], erc20TransferSelector):
		if len(input) < 4+32+32 {
			return nil
		}
		to, ok := abiAddress(input[4 : 4+32])
		if !ok {
			return nil
		}
		return &TxERC20Transfer{
			Caller: caller,
			Token:  token,
			From:   caller,
			To:     to,
			Value:  new(big.Int).SetBytes(input[4+32 : 4+64]),
		}
	case bytes.Equal(input[:4], erc20TransferFromSelector):
		if len(input) < 4+32+32+32 {
			return nil
		}
		from, ok := abiAddress(input[4 : 4+32])
		if !ok {
			return nil
		}
		to, ok := abiAddress(input[4+32 : 4+64])
		if !ok {
			return nil
		}
		return &TxERC20Transfer{
			Caller: caller,
			Token:  token,
			From:   from,
			To:     to,
			Value:  new(big.Int).SetBytes(input[4+64 : 4+96]),
		}
	default:
		return nil
	}
}

func abiAddress(word []byte) (common.Address, bool) {
	if len(word) != common.HashLength || !bytes.Equal(word[:12], zeroABIWord[:12]) {
		return common.Address{}, false
	}
	return common.BytesToAddress(word[12:]), true
}

func isFalseERC20Return(output []byte) bool {
	return len(output) >= common.HashLength && bytes.Equal(output[:common.HashLength], zeroABIWord[:])
}
