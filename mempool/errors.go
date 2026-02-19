package mempool

import (
	"errors"

	"github.com/cosmos/evm/mempool/internal/queue"
)

var (
	ErrNoMessages                  = errors.New("transaction has no messages")
	ErrExpectedOneMessage          = errors.New("expected 1 message")
	ErrExpectedOneError            = errors.New("expected 1 error")
	ErrNotEVMTransaction           = errors.New("transaction is not an EVM transaction")
	ErrMultiMsgEthereumTransaction = errors.New("transaction contains multiple messages with an EVM msg")
	ErrNonceGap                    = errors.New("tx nonce is higher than account nonce")
	ErrNonceLow                    = errors.New("tx nonce is lower than account nonce")
	// ErrQueueFull is aliased from the internal queue package so that external
	// packages (e.g. evmd) can check for this error without importing internal/.
	ErrQueueFull = queue.ErrQueueFull
)
