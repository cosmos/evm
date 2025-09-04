package suite

import (
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

func (s *SystemTestSuite) SendTx(
	nodeID string,
	accID string,
	nonceIdx uint64,
	gasPrice *big.Int,
	gasTipCap *big.Int,
) (string, error) {
	nonce, err := s.NonceAt(nodeID, accID)
	if err != nil {
		return "", fmt.Errorf("failed to get future current nonce")
	}
	gappedNonce := nonce + nonceIdx

	if s.TestOption.TxType == TxTypeEVM {
		return s.SendEthTx(nodeID, accID, gappedNonce, gasPrice, gasTipCap)
	}
	return s.SendCosmosTx(nodeID, accID, gappedNonce, gasPrice, nil)
}

func (s *SystemTestSuite) SendEthTx(
	nodeID string,
	accID string,
	nonce uint64,
	gasPrice *big.Int,
	gasTipCap *big.Int,
) (string, error) {
	if s.TestOption.ApplyDynamicFeeTx {
		return s.SendEthDynamicFeeTx(nodeID, accID, nonce, gasPrice, gasTipCap)
	}
	return s.SendEthLegacyTx(nodeID, accID, nonce, gasPrice)
}

func (s *SystemTestSuite) SendEthLegacyTx(
	nodeID string,
	accID string,
	nonce uint64,
	gasPrice *big.Int,
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

func (s *SystemTestSuite) SendCosmosTx(
	nodeID string,
	accID string,
	nonce uint64,
	gasPrice *big.Int,
	_ *big.Int,
) (string, error) {
	from := s.CosmosClient.Accs[accID].AccAddress
	to := s.CosmosClient.Accs["acc3"].AccAddress
	amount := sdkmath.NewInt(1000)

	resp, err := s.CosmosClient.BankSend(nodeID, accID, from, to, amount, nonce, gasPrice)
	if err != nil {
		return "", fmt.Errorf("failed to cosmos tx bank send: %v", err)
	}
	return resp.TxHash, nil
}
