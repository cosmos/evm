package mempool

import "errors"

var (
	ErrNoMessages         = errors.New("transaction has no messages")
	ErrExpectedOneMessage = errors.New("expected 1 message")
	ErrExpectedOneError   = errors.New("expected 1 error")
	ErrNotEVMTransaction  = errors.New("transaction is not an EVM transaction")
	ErrNonceGap           = errors.New("tx nonce is higher than account nonce")
	ErrNonceLow           = errors.New("tx nonce is lower than account nonce")
	ErrInvalidTx          = errors.New("tx is invalid")
	ErrMempoolFull        = errors.New("mempool is full")
	ErrAlreadyKnown       = errors.New("tx already known")
)
