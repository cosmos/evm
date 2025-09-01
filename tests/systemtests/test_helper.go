package systemtests

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/evmos/tests/systemtests/clients"
)

func TransferLegacyTx(
	ethClient *clients.EthClient,
	nodeID string,
	accID string,
	nonce uint64,
	gasPrice *big.Int,
) (common.Hash, error) {
	to := ethClient.Accs["acc3"].Address
	value := big.NewInt(1000)
	gasLimit := uint64(50_000)

	tx := ethtypes.NewTransaction(nonce, to, value, gasLimit, gasPrice, nil)

	return ethClient.SendRawTransaction(nodeID, accID, tx)
}

func TransferDynamicFeeTx(
	ethClient *clients.EthClient,
	nodeID string,
	accID string,
	nonce uint64,
	GasFeeCap *big.Int,
) (common.Hash, error) {
	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   ethClient.ChainID,
		Nonce:     nonce,
		To:        &(ethClient.Accs["acc3"].Address),
		Value:     big.NewInt(1000),
		Gas:       uint64(50_000),
		GasFeeCap: GasFeeCap,
		GasTipCap: big.NewInt(100),
	})

	return ethClient.SendRawTransaction(nodeID, accID, tx)
}
