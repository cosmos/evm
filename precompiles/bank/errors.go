package bank

import "errors"

var (
	ErrDenomNotFound = errors.New("denom not found")
	ErrUnauthorized  = errors.New("unauthorized")
)
