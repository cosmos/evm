package systemtests

import (
	"math/big"
)

type TransferFunc func(
	nodeID string,
	accID string,
	nonce uint64,
	gasPrice *big.Int,
	optionalGasTipCap *big.Int,
) (string, error)

type TestOption struct {
	TxType       string
	TransferFunc TransferFunc
}
