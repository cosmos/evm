package ibc

import "errors"

var (
	ErrNoIBCVoucherDenom = errors.New("denom is not an IBC voucher")
	ErrDenomNotFound     = errors.New("denom not found")
	ErrInvalidBaseDenom  = errors.New("invalid base denomination")
)
