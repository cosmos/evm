package mempool

import (
	"encoding/hex"
	"fmt"
	"math/big"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/crypto/tmhash"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/evm/testutil/integration/base/factory"
	"github.com/cosmos/evm/testutil/keyring"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// Constants
const (
	TxGas = 100_000
)

// createCosmosSendTransactionWithKey creates a simple bank send transaction with the specified key
func (s *IntegrationTestSuite) createCosmosSendTx(key keyring.Key, gasPrice *big.Int) sdk.Tx {
	feeDenom := "aatom"

	fromAddr := key.AccAddr
	toAddr := s.keyring.GetKey(1).AccAddr
	amount := sdk.NewCoins(sdk.NewInt64Coin(feeDenom, 1000))

	bankMsg := banktypes.NewMsgSend(fromAddr, toAddr, amount)

	gasPriceConverted := sdkmath.NewIntFromBigInt(gasPrice)

	txArgs := factory.CosmosTxArgs{
		Msgs:     []sdk.Msg{bankMsg},
		GasPrice: &gasPriceConverted,
	}
	tx, err := s.factory.BuildCosmosTx(key.Priv, txArgs)
	s.Require().NoError(err)

	fmt.Printf("DEBUG: Created cosmos transaction successfully\n")
	return tx
}

// createEVMTransaction creates an EVM transaction using the provided key
func (s *IntegrationTestSuite) createEVMValueTransferTx(key keyring.Key, nonce int, gasPrice *big.Int) sdk.Tx {
	to := s.keyring.GetKey(1).Addr

	if nonce < 0 {
		s.Require().NoError(fmt.Errorf("nonce must be non-negative"))
	}

	ethTxArgs := evmtypes.EvmTxArgs{
		Nonce:    uint64(nonce),
		To:       &to,
		Amount:   big.NewInt(1000),
		GasLimit: TxGas,
		GasPrice: gasPrice,
		Input:    nil,
	}
	tx, err := s.factory.GenerateSignedEthTx(key.Priv, ethTxArgs)
	s.Require().NoError(err)

	return tx
}

// createEVMContractDeployTx creates an EVM transaction for contract deployment
func (s *IntegrationTestSuite) createEVMContractDeployTx(key keyring.Key, gasPrice *big.Int, data []byte) sdk.Tx {
	ethTxArgs := evmtypes.EvmTxArgs{
		Nonce:    0,
		To:       nil,
		Amount:   nil,
		GasLimit: TxGas,
		GasPrice: gasPrice,
		Input:    data,
	}
	tx, err := s.factory.GenerateSignedEthTx(key.Priv, ethTxArgs)
	s.Require().NoError(err)

	return tx
}

// checkTxs call abci CheckTx for multipile transactions
func (s *IntegrationTestSuite) checkTxs(txs []sdk.Tx) error {
	for _, tx := range txs {
		if err := s.checkTx(tx); err != nil {
			return err
		}
	}
	return nil
}

// checkTxs call abci CheckTx for a transaction
func (s *IntegrationTestSuite) checkTx(tx sdk.Tx) error {
	txBytes, err := s.network.App.GetTxConfig().TxEncoder()(tx)
	if err != nil {
		return fmt.Errorf("failed to encode cosmos tx: %w", err)
	}

	_, err = s.network.App.CheckTx(&abci.RequestCheckTx{
		Tx:   txBytes,
		Type: abci.CheckTxType_New,
	})
	if err != nil {
		return fmt.Errorf("failed to encode cosmos tx: %w", err)
	}

	return nil
}

// getTxHashes returns transaction hashes for multiple transactions
func (s *IntegrationTestSuite) getTxHashes(txs []sdk.Tx) []string {
	txHashes := []string{}
	for _, tx := range txs {
		txHash := s.getTxHash(tx)
		txHashes = append(txHashes, txHash)
	}

	return txHashes
}

// getTxHash returns transaction hash for a transaction
func (s *IntegrationTestSuite) getTxHash(tx sdk.Tx) string {
	txEncoder := s.network.App.GetTxConfig().TxEncoder()
	txBytes, err := txEncoder(tx)
	s.Require().NoError(err)

	return hex.EncodeToString(tmhash.Sum(txBytes))
}

// calculateCosmosGasPrice calculates the gas price for a Cosmos transaction
func (s *IntegrationTestSuite) calculateCosmosGasPrice(feeAmount int64, gasLimit uint64) *big.Int {
	return new(big.Int).Div(big.NewInt(feeAmount), big.NewInt(int64(gasLimit))) //#nosec G115 -- not concern, test
}

// calculateCosmosEffectiveTip calculates the effective tip for a Cosmos transaction
// This aligns with EVM transaction prioritization: effective_tip = gas_price - base_fee
func (s *IntegrationTestSuite) calculateCosmosEffectiveTip(feeAmount int64, gasLimit uint64, baseFee *big.Int) *big.Int {
	gasPrice := s.calculateCosmosGasPrice(feeAmount, gasLimit)
	if baseFee == nil || baseFee.Sign() == 0 {
		return gasPrice // No base fee, effective tip equals gas price
	}

	if gasPrice.Cmp(baseFee) < 0 {
		return big.NewInt(0) // Gas price lower than base fee, effective tip is zero
	}

	return new(big.Int).Sub(gasPrice, baseFee)
}
