package systemtests

import (
	"math/big"
	"time"
)

type FuncTransfer func(
	nodeID string,
	accID string,
	nonce uint64,
	gasPrice *big.Int,
	optionalGasTipCap *big.Int,
) (string, error)

type FuncWaitForCommit func(
	nodeID string,
	txHash string,
	timeout time.Duration,
) error

type TestOption struct {
	TestType      string
	Transfer      FuncTransfer
	WaitForCommit FuncWaitForCommit
}
