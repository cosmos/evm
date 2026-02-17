package mempool

import "errors"

var (
	ErrNoMessages                  = errors.New("transaction has no messages")
	ErrExpectedOneMessage          = errors.New("expected 1 message")
	ErrExpectedOneError            = errors.New("expected 1 error")
	ErrNotEVMTransaction           = errors.New("transaction is not an EVM transaction")
	ErrMultiMsgEthereumTransaction = errors.New("transaction contains multiple messages with an EVM msg")
	ErrNonceGap                    = errors.New("tx nonce is higher than account nonce")
	ErrNonceLow                    = errors.New("tx nonce is lower than account nonce")
	ErrQueueFull                   = errors.New("queue full")
)
