package suite

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

func (s *SystemTestSuite) TransferLegacyTx(
	nodeID string,
	accID string,
	nonce uint64,
	gasPrice *big.Int,
	_ *big.Int,
) (common.Hash, error) {
	to := s.EthClient.Accs["acc3"].Address
	value := big.NewInt(1000)
	gasLimit := uint64(50_000)

	tx := ethtypes.NewTransaction(nonce, to, value, gasLimit, gasPrice, nil)

	return s.EthClient.SendRawTransaction(nodeID, accID, tx)
}

func (s *SystemTestSuite) TransferDynamicFeeTx(
	nodeID string,
	accID string,
	nonce uint64,
	gasFeeCap *big.Int,
	gasTipCap *big.Int,
) (common.Hash, error) {
	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   s.EthClient.ChainID,
		Nonce:     nonce,
		To:        &(s.EthClient.Accs["acc3"].Address),
		Value:     big.NewInt(1000),
		Gas:       uint64(50_000),
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
	})

	return s.EthClient.SendRawTransaction(nodeID, accID, tx)
}
