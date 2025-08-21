package mempool

import (
	"fmt"
	"math/big"

	abci "github.com/cometbft/cometbft/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/testutil/keyring"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// createCosmosSendTransactionWithKey creates a simple bank send transaction with the specified key
func (s *IntegrationTestSuite) createCosmosSendTransactionWithKey(key keyring.Key, gasPrice *big.Int) sdk.Tx {
	feeDenom := "aatom"
	gasLimit := uint64(TxGas)

	// Calculate fee amount from gas price: fee = gas_price * gas_limit
	feeAmount := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))

	fmt.Printf("DEBUG: Creating cosmos transaction with gas price: %s aatom/gas, fee: %s %s\n", gasPrice.String(), feeAmount.String(), feeDenom)

	fromAddr := key.AccAddr
	toAddr := s.keyring.GetKey(1).AccAddr
	amount := sdk.NewCoins(sdk.NewInt64Coin(feeDenom, 1000))

	bankMsg := banktypes.NewMsgSend(fromAddr, toAddr, amount)

	txBuilder := s.network.App.GetTxConfig().NewTxBuilder()
	err := txBuilder.SetMsgs(bankMsg)
	s.Require().NoError(err)

	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin(feeDenom, feeAmount.Int64())))
	txBuilder.SetGasLimit(gasLimit)

	// Sign the transaction with proper signing instead of dummy signature
	err = s.factory.SignCosmosTx(key.Priv, txBuilder)
	s.Require().NoError(err)

	fmt.Printf("DEBUG: Created cosmos transaction successfully\n")
	return txBuilder.GetTx()
}

// createCosmosSendTransaction creates a simple bank send transaction using the first key
func (s *IntegrationTestSuite) createCosmosSendTransaction(gasPrice *big.Int) sdk.Tx {
	key := s.keyring.GetKey(0)
	return s.createCosmosSendTransactionWithKey(key, gasPrice)
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

// createEVMTransaction creates an EVM transaction using the provided key
func (s *IntegrationTestSuite) createEVMTransactionWithKey(key keyring.Key, gasPrice *big.Int) (sdk.Tx, error) {
	fmt.Printf("DEBUG: Creating EVM transaction with gas price: %s\n", gasPrice.String())

	privKey := key.Priv

	// Convert Cosmos address to EVM address
	fromAddr := common.BytesToAddress(key.AccAddr.Bytes())
	fmt.Printf("DEBUG: Using prefunded account: %s\n", fromAddr.Hex())

	to := common.HexToAddress("0x1234567890123456789012345678901234567890")
	ethTx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    0,
		To:       &to,
		Value:    big.NewInt(1000),
		Gas:      TxGas,
		GasPrice: gasPrice,
		Data:     nil,
	})

	// Convert to ECDSA private key for signing
	ethPrivKey, ok := privKey.(*ethsecp256k1.PrivKey)
	if !ok {
		return nil, fmt.Errorf("expected ethsecp256k1.PrivKey, got %T", privKey)
	}

	ecdsaPrivKey, err := ethPrivKey.ToECDSA()
	if err != nil {
		return nil, err
	}

	signer := ethtypes.HomesteadSigner{}
	signedTx, err := ethtypes.SignTx(ethTx, signer, ecdsaPrivKey)
	if err != nil {
		return nil, err
	}

	msgEthTx := &evmtypes.MsgEthereumTx{}
	msgEthTx.FromEthereumTx(signedTx)

	txBuilder := s.network.App.GetTxConfig().NewTxBuilder()
	err = txBuilder.SetMsgs(msgEthTx)
	if err != nil {
		return nil, err
	}

	fmt.Printf("DEBUG: Created EVM transaction successfully\n")
	return txBuilder.GetTx(), nil
}

// createEVMTransaction creates an EVM transaction using the first key
func (s *IntegrationTestSuite) createEVMTransaction(gasPrice *big.Int) (sdk.Tx, error) {
	key := s.keyring.GetKey(0)
	return s.createEVMTransactionWithKey(key, gasPrice)
}

// createEVMContractDeployment creates an EVM transaction for contract deployment
func (s *IntegrationTestSuite) createEVMContractDeployment(key keyring.Key, gasPrice *big.Int, data []byte) (sdk.Tx, error) {
	fmt.Printf("DEBUG: Creating EVM contract deployment transaction with gas price: %s\n", gasPrice.String())

	privKey := key.Priv

	// Convert Cosmos address to EVM address
	fromAddr := common.BytesToAddress(key.AccAddr.Bytes())
	fmt.Printf("DEBUG: Using prefunded account: %s\n", fromAddr.Hex())

	ethTx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    0,
		To:       nil, // nil for contract deployment
		Value:    big.NewInt(0),
		Gas:      100000,
		GasPrice: gasPrice,
		Data:     data,
	})

	// Convert to ECDSA private key for signing
	ethPrivKey, ok := privKey.(*ethsecp256k1.PrivKey)
	if !ok {
		return nil, fmt.Errorf("expected ethsecp256k1.PrivKey, got %T", privKey)
	}

	ecdsaPrivKey, err := ethPrivKey.ToECDSA()
	if err != nil {
		return nil, err
	}

	signer := ethtypes.HomesteadSigner{}
	signedTx, err := ethtypes.SignTx(ethTx, signer, ecdsaPrivKey)
	if err != nil {
		return nil, err
	}

	msgEthTx := &evmtypes.MsgEthereumTx{}
	msgEthTx.FromEthereumTx(signedTx)

	txBuilder := s.network.App.GetTxConfig().NewTxBuilder()
	err = txBuilder.SetMsgs(msgEthTx)
	if err != nil {
		return nil, err
	}

	fmt.Printf("DEBUG: Created EVM contract deployment transaction successfully\n")
	return txBuilder.GetTx(), nil
}

// createEVMValueTransfer creates an EVM transaction for value transfer
func (s *IntegrationTestSuite) createEVMValueTransfer(key keyring.Key, gasPrice *big.Int, value *big.Int, to common.Address) (sdk.Tx, error) {
	fmt.Printf("DEBUG: Creating EVM value transfer transaction with gas price: %s\n", gasPrice.String())

	privKey := key.Priv

	// Convert Cosmos address to EVM address
	fromAddr := common.BytesToAddress(key.AccAddr.Bytes())
	fmt.Printf("DEBUG: Using prefunded account: %s\n", fromAddr.Hex())

	ethTx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    0,
		To:       &to,
		Value:    value,
		Gas:      TxGas,
		GasPrice: gasPrice,
		Data:     nil,
	})

	// Convert to ECDSA private key for signing
	ethPrivKey, ok := privKey.(*ethsecp256k1.PrivKey)
	if !ok {
		return nil, fmt.Errorf("expected ethsecp256k1.PrivKey, got %T", privKey)
	}

	ecdsaPrivKey, err := ethPrivKey.ToECDSA()
	if err != nil {
		return nil, err
	}

	signer := ethtypes.HomesteadSigner{}
	signedTx, err := ethtypes.SignTx(ethTx, signer, ecdsaPrivKey)
	if err != nil {
		return nil, err
	}

	msgEthTx := &evmtypes.MsgEthereumTx{}
	msgEthTx.FromEthereumTx(signedTx)
	if err != nil {
		return nil, err
	}

	txBuilder := s.network.App.GetTxConfig().NewTxBuilder()
	err = txBuilder.SetMsgs(msgEthTx)
	if err != nil {
		return nil, err
	}

	fmt.Printf("DEBUG: Created EVM value transfer transaction successfully\n")
	return txBuilder.GetTx(), nil
}

// createEVMTransactionWithNonce creates an EVM transaction with a specific nonce
func (s *IntegrationTestSuite) createEVMTransactionWithNonce(key keyring.Key, gasPrice *big.Int, nonce int) (sdk.Tx, error) {
	fmt.Printf("DEBUG: Creating EVM transaction with gas price: %s and nonce: %d\n", gasPrice.String(), nonce)

	privKey := key.Priv

	// Convert Cosmos address to EVM address
	fromAddr := common.BytesToAddress(key.AccAddr.Bytes())
	fmt.Printf("DEBUG: Using prefunded account: %s\n", fromAddr.Hex())

	to := common.HexToAddress("0x1234567890123456789012345678901234567890")
	ethTx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    uint64(nonce), //#nosec G115 -- int overflow is not a concern here
		To:       &to,
		Value:    big.NewInt(1000),
		Gas:      TxGas,
		GasPrice: gasPrice,
		Data:     nil,
	})

	// Convert to ECDSA private key for signing
	ethPrivKey, ok := privKey.(*ethsecp256k1.PrivKey)
	if !ok {
		return nil, fmt.Errorf("expected ethsecp256k1.PrivKey, got %T", privKey)
	}

	ecdsaPrivKey, err := ethPrivKey.ToECDSA()
	if err != nil {
		return nil, err
	}

	signer := ethtypes.HomesteadSigner{}
	signedTx, err := ethtypes.SignTx(ethTx, signer, ecdsaPrivKey)
	if err != nil {
		return nil, err
	}

	msgEthTx := &evmtypes.MsgEthereumTx{}
	msgEthTx.FromEthereumTx(signedTx)
	if err != nil {
		return nil, err
	}

	txBuilder := s.network.App.GetTxConfig().NewTxBuilder()
	err = txBuilder.SetMsgs(msgEthTx)
	if err != nil {
		return nil, err
	}

	fmt.Printf("DEBUG: Created EVM transaction successfully\n")
	return txBuilder.GetTx(), nil
}

func (s *IntegrationTestSuite) checkTxs(txs []sdk.Tx) ([]*abci.ResponseCheckTx, error) {
	result := make([]*abci.ResponseCheckTx, 0)

	for _, tx := range txs {
		txBytes, err := s.network.App.GetTxConfig().TxEncoder()(tx)
		if err != nil {
			return []*abci.ResponseCheckTx{}, fmt.Errorf("failed to encode cosmos tx: %w", err)
		}

		res, err := s.network.App.CheckTx(&abci.RequestCheckTx{
			Tx:   txBytes,
			Type: abci.CheckTxType_New,
		})
		if err != nil {
			return []*abci.ResponseCheckTx{}, fmt.Errorf("failed to encode cosmos tx: %w", err)
		}

		result = append(result, res)
	}

	return result, nil
}
