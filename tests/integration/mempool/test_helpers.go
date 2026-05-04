package mempool

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/cometbft/cometbft/crypto/tmhash"

	"github.com/cosmos/evm/testutil/integration/base/factory"
	"github.com/cosmos/evm/testutil/keyring"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// Constants
const (
	TxGas    = 100_000
	feeDenom = "aatom"
)

// createCosmosSendTransactionWithKey creates a simple bank send transaction
// with the specified key, sending 1000aatom
func (s *IntegrationTestSuite) createCosmosSendTx(key keyring.Key, gasPrice *big.Int) sdk.Tx {
	return s.createCosmosSendTxWithAmount(key, big.NewInt(1000), gasPrice)
}

// createCosmosSendTransactionWithKey creates a simple bank send transaction
// with the specified key and amount
func (s *IntegrationTestSuite) createCosmosSendTxWithAmount(key keyring.Key, amt *big.Int, gasPrice *big.Int) sdk.Tx {
	fromAddr := key.AccAddr
	toAddr := s.keyring.GetKey(1).AccAddr
	amount := sdk.NewCoins(sdk.NewCoin(feeDenom, sdkmath.NewIntFromBigInt(amt)))

	bankMsg := banktypes.NewMsgSend(fromAddr, toAddr, amount)

	gasPriceConverted := sdkmath.NewIntFromBigInt(gasPrice)

	txArgs := factory.CosmosTxArgs{
		Msgs:     []sdk.Msg{bankMsg},
		GasPrice: &gasPriceConverted,
	}
	tx, err := s.factory.BuildCosmosTx(key.Priv, txArgs)
	s.Require().NoError(err)

	return tx
}

// createCosmosSendTransactionWithKey creates a simple bank send transaction
// with the specified key and amount. Uses a custom nonce for the sender that
// may not match what is currently the next nonce on chain for this sender.
// Requires a custom gasLimit to be set to prevent simulation of the tx in
// order to fetch the gas limit (since this will fail due to a nonce mismatch).
func (s *IntegrationTestSuite) createCosmosSendTxWithNonceAndGas(key keyring.Key, nonce uint64, amt *big.Int, gasLimit uint64, gasPrice *big.Int) sdk.Tx {
	fromAddr := key.AccAddr
	toAddr := s.keyring.GetKey(1).AccAddr
	amount := sdk.NewCoins(sdk.NewCoin(feeDenom, sdkmath.NewIntFromBigInt(amt)))

	bankMsg := banktypes.NewMsgSend(fromAddr, toAddr, amount)

	gasPriceConverted := sdkmath.NewIntFromBigInt(gasPrice)

	txArgs := factory.CosmosTxArgs{
		Msgs:     []sdk.Msg{bankMsg},
		GasPrice: &gasPriceConverted,
		Nonce:    &nonce,
		Gas:      &gasLimit,
	}
	tx, err := s.factory.BuildCosmosTx(key.Priv, txArgs)
	s.Require().NoError(err)

	return tx
}

func (s *IntegrationTestSuite) createMultiSignerCosmosSendTx(gasPrice *big.Int, keys ...keyring.Key) sdk.Tx {
	return s.createMultiSignerCosmosSendTxWithAmount(big.NewInt(1000), gasPrice, keys...)
}

func (s *IntegrationTestSuite) createMultiSignerCosmosSendTxWithAmount(amt *big.Int, gasPrice *big.Int, keys ...keyring.Key) sdk.Tx {
	if len(keys) == 0 {
		panic("no keys provided")
	}

	toAddr := s.keyring.GetKey(9).AccAddr
	amount := sdk.NewCoins(sdk.NewCoin(feeDenom, sdkmath.NewIntFromBigInt(amt)))

	// one MsgSend per signer so the tx's aggregated GetSigners() returns one
	// entry per provided key
	msgs := make([]sdk.Msg, len(keys))
	privs := make([]types.PrivKey, len(keys))
	for i, key := range keys {
		msgs[i] = banktypes.NewMsgSend(key.AccAddr, toAddr, amount)
		privs[i] = key.Priv
	}

	gasPriceConverted := sdkmath.NewIntFromBigInt(gasPrice)

	txArgs := factory.CosmosTxArgs{
		Msgs:     msgs,
		GasPrice: &gasPriceConverted,
	}
	tx, err := s.factory.BuildMultiSignerCosmosTx(txArgs, privs...)
	s.Require().NoError(err)

	return tx
}

// createEVMTransaction creates an EVM transaction using the provided key,
// nonce, and gas price. Defaults to sending 1000 of the native gas token
func (s *IntegrationTestSuite) createEVMValueTransferTx(key keyring.Key, nonce int, gasPrice *big.Int) sdk.Tx {
	return s.createEVMValueTransferTxWithValue(key, nonce, big.NewInt(1000), gasPrice)
}

// createEVMTransaction creates an EVM transaction using the provided key,
// nonce, value, and gas price
func (s *IntegrationTestSuite) createEVMValueTransferTxWithValue(key keyring.Key, nonce int, value *big.Int, gasPrice *big.Int) sdk.Tx {
	to := s.keyring.GetKey(1).Addr

	if nonce < 0 {
		s.Require().NoError(fmt.Errorf("nonce must be non-negative"))
	}

	ethTxArgs := evmtypes.EvmTxArgs{
		// #nosec G115 -- nonce checked >= 0 above
		Nonce:    uint64(nonce),
		To:       &to,
		Amount:   value,
		GasLimit: TxGas,
		GasPrice: gasPrice,
		Input:    nil,
	}
	tx, err := s.factory.GenerateSignedEthTx(key.Priv, ethTxArgs)
	s.Require().NoError(err)

	return tx
}

// createEVMTransaction creates an EVM transaction using the provided key
func (s *IntegrationTestSuite) createEVMValueTransferDynamicFeeTx(key keyring.Key, nonce int, gasFeeCap, gasTipCap *big.Int) sdk.Tx {
	to := s.keyring.GetKey(1).Addr

	if nonce < 0 {
		s.Require().NoError(fmt.Errorf("nonce must be non-negative"))
	}

	ethTxArgs := evmtypes.EvmTxArgs{
		// #nosec G115 -- nonce checked >= 0 above
		Nonce:     uint64(nonce),
		To:        &to,
		Amount:    big.NewInt(1000),
		GasLimit:  TxGas,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Input:     nil,
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

// insertTxs call mempool Insert for multiple transactions
func (s *IntegrationTestSuite) insertTxs(txs []sdk.Tx) error {
	for idx, tx := range txs {
		if err := s.insertTx(tx); err != nil {
			return fmt.Errorf("failed to insert for tx at idx %d %s: %w", idx, s.getTxHash(tx), err)
		}
	}
	return nil
}

// insertTx call mempool Insert for a transaction
func (s *IntegrationTestSuite) insertTx(tx sdk.Tx) error {
	return s.network.App.GetMempool().Insert(s.network.GetContext(), tx)
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
