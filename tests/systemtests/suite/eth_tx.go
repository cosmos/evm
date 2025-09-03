package suite

import (
	"fmt"
	"math/big"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

func (s *SystemTestSuite) SendEthTx(
	nodeID string,
	accID string,
	nonce uint64,
	gasPrice *big.Int,
	_ *big.Int,
) (string, error) {
	to := s.EthClient.Accs["acc3"].Address
	value := big.NewInt(1000)
	gasLimit := uint64(50_000)

	tx := ethtypes.NewTransaction(nonce, to, value, gasLimit, gasPrice, nil)

	txHash, err := s.EthClient.SendRawTransaction(nodeID, accID, tx)
	if err != nil {
		return "", fmt.Errorf("failed to send ")
	}

	return txHash.Hex(), nil
}

func (s *SystemTestSuite) SendEthDynamicFeeTx(
	nodeID string,
	accID string,
	nonce uint64,
	gasFeeCap *big.Int,
	gasTipCap *big.Int,
) (string, error) {
	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   s.EthClient.ChainID,
		Nonce:     nonce,
		To:        &(s.EthClient.Accs["acc3"].Address),
		Value:     big.NewInt(1000),
		Gas:       uint64(50_000),
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
	})

	txHash, err := s.EthClient.SendRawTransaction(nodeID, accID, tx)
	if err != nil {
		return "", fmt.Errorf("failed to send dynamic tx")
	}

	return txHash.Hex(), nil
}
