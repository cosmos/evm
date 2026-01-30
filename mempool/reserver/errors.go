package reserver

import "errors"

var (
	// ErrAlreadyReserved is returned if the sender address has a pending transaction
	// in a different subpool. For example, this error is returned in response to any
	// input transaction of non-blob type when a blob transaction from this sender
	// remains pending (and vice-versa).
	ErrAlreadyReserved = errors.New("address already reserved")
)
