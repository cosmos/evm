package reserver

import "errors"

// ErrAlreadyReserved is returned if the sender address has a pending transaction
// in a different subpool. For example, this error is returned in response to any
// input transaction of non-blob type when a blob transaction from this sender
// remains pending (and vice-versa).
var ErrAlreadyReserved = errors.New("address already reserved")
