package suite

import (
	"math/big"
)

const (
	TxTypeEVM    = "EVMTx"
	TxTypeCosmos = "CosmosTx"
)

type FuncTransfer func(
	nodeID string,
	accID string,
	nonce uint64,
	gasPrice *big.Int,
	optionalGasTipCap *big.Int,
) (string, error)

type TestOption struct {
	TestType string
	TxType   string
	SendTx   FuncTransfer
}
