package suite

import (
	"fmt"
	"math/big"
	"testing"

	sdkmath "cosmossdk.io/math"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// GetOptions retrieves the current test options.
func (s *SystemTestSuite) SendTx(
	t *testing.T,
	nodeID string,
	accID string,
	nonceIdx uint64,
	gasPrice *big.Int,
	gasTipCap *big.Int,
) (*TxInfo, error) {
	options := s.GetOptions()
	if options != nil && options.TxType == TxTypeCosmos {
		return s.SendCosmosTx(t, nodeID, accID, nonceIdx, gasPrice, nil)
	}
	return s.SendEthTx(t, nodeID, accID, nonceIdx, gasPrice, gasTipCap)
}

// SendEthTx sends an Ethereum transaction (either Legacy or Dynamic Fee based on options).
func (s *SystemTestSuite) SendEthTx(
	t *testing.T,
	nodeID string,
	accID string,
	nonceIdx uint64,
	gasPrice *big.Int,
	gasTipCap *big.Int,
) (*TxInfo, error) {
	options := s.GetOptions()
	if options != nil && options.IsDynamicFeeTx {
		return s.SendEthDynamicFeeTx(t, nodeID, accID, nonceIdx, gasPrice, gasTipCap)
	}
	return s.SendEthLegacyTx(t, nodeID, accID, nonceIdx, gasPrice)
}

// SendEthLegacyTx sends an Ethereum legacy transaction.
func (s *SystemTestSuite) SendEthLegacyTx(
	t *testing.T,
	nodeID string,
	accID string,
	nonceIdx uint64,
	gasPrice *big.Int,
) (*TxInfo, error) {
	nonce, err := s.NonceAt(nodeID, accID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current nonce: %v", err)
	}
	gappedNonce := nonce + nonceIdx
	to := s.EthClient.Accs["acc3"].Address
	value := big.NewInt(1000)
	gasLimit := uint64(50_000)

	tx := ethtypes.NewTransaction(gappedNonce, to, value, gasLimit, gasPrice, nil)
	txHash, err := s.EthClient.SendRawTransaction(nodeID, accID, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to send eth legacy tx: %v", err)
	}

	return NewTxInfo(nodeID, txHash.Hex(), TxTypeEVM), nil
}

// SendEthDynamicFeeTx sends an Ethereum dynamic fee transaction.
func (s *SystemTestSuite) SendEthDynamicFeeTx(
	t *testing.T,
	nodeID string,
	accID string,
	nonceIdx uint64,
	gasFeeCap *big.Int,
	gasTipCap *big.Int,
) (*TxInfo, error) {
	nonce, err := s.NonceAt(nodeID, accID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current nonce: %v", err)
	}
	gappedNonce := nonce + nonceIdx

	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   s.EthClient.ChainID,
		Nonce:     gappedNonce,
		To:        &(s.EthClient.Accs["acc3"].Address),
		Value:     big.NewInt(1000),
		Gas:       uint64(50_000),
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
	})

	txHash, err := s.EthClient.SendRawTransaction(nodeID, accID, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to send eth dynamic fee tx: %v", err)
	}

	return NewTxInfo(nodeID, txHash.Hex(), TxTypeEVM), nil
}

// SendCosmosTx sends a Cosmos transaction.
func (s *SystemTestSuite) SendCosmosTx(
	t *testing.T,
	nodeID string,
	accID string,
	nonceIdx uint64,
	gasPrice *big.Int,
	gasTipCap *big.Int,
) (*TxInfo, error) {
	from := s.CosmosClient.Accs[accID].AccAddress
	to := s.CosmosClient.Accs["acc3"].AccAddress
	amount := sdkmath.NewInt(1000)

	nonce, err := s.NonceAt(nodeID, accID)
	if err != nil {
		return nil, fmt.Errorf("failed to get current nonce: %v", err)
	}
	gappedNonce := nonce + nonceIdx

	resp, err := s.CosmosClient.BankSend(nodeID, accID, from, to, amount, gappedNonce, gasPrice)
	if err != nil {
		return nil, fmt.Errorf("failed to send cosmos bank send tx: %v", err)
	}

	return NewTxInfo(nodeID, resp.TxHash, TxTypeCosmos), nil
}
