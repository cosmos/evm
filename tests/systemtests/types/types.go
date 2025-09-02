package systemtests

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type TransferFunc func(
	nodeID string,
	accID string,
	nonce uint64,
	gasPrice *big.Int,
	optionalGasTipCap *big.Int,
) (common.Hash, error)

type TestOption struct {
	TxType       string
	TransferFunc TransferFunc
}
