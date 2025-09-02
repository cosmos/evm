package systemtests

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/tests/systemtests/clients"
)

type TransferFunc func(
	ethClient *clients.EthClient,
	nodeID string,
	accID string,
	nonce uint64,
	gasPrice *big.Int,
	optionalGasTipCap *big.Int,
) (common.Hash, error)

type TestOptions struct {
	TxType       string
	TransferFunc TransferFunc
}
