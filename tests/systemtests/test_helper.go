package systemtests

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/evmos/tests/systemtests/clients"
)

func ValueTransferLegacyTx(
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

func ValueTransferDynamicFeeTx(
	ethClient *clients.EthClient,
	nodeID string,
	accID string,
	nonce uint64,
	gasTipCap *big.Int,
	GasFeeCap *big.Int,
) (common.Hash, error) {
	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   ethClient.ChainID,
		Nonce:     nonce,
		To:        &(ethClient.Accs["acc3"].Address),
		Value:     big.NewInt(1000),
		Gas:       uint64(50_000),
		GasTipCap: gasTipCap,
		GasFeeCap: GasFeeCap,
	})

	return ethClient.SendRawTransaction(nodeID, accID, tx)
}
