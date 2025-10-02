package accountabstraction

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cosmos/evm/tests/systemtests/clients"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
)

func createSetCodeAuthorization(chainID, nonce uint64, contractAddr common.Address) ethtypes.SetCodeAuthorization {
	return ethtypes.SetCodeAuthorization{
		ChainID: *uint256.NewInt(chainID),
		Address: contractAddr,
		Nonce:   nonce,
	}
}

func signSetCodeAuthorization(key *ecdsa.PrivateKey, authorization ethtypes.SetCodeAuthorization) (ethtypes.SetCodeAuthorization, error) {
	authorization, err := ethtypes.SignSetCode(key, authorization)
	if err != nil {
		return ethtypes.SetCodeAuthorization{}, fmt.Errorf("failed to sign set code authorization: %w", err)
	}

	return authorization, nil
}

func loadSmartWalletCreationBytecode(filePath string) ([]byte, error) {
	_, caller, _, ok := runtime.Caller(0)
	if !ok {
		return nil, errors.New("failed to resolve caller for smart wallet artifact")
	}

	artifactPath := filepath.Join(filepath.Dir(caller), filePath)
	contents, err := os.ReadFile(filepath.Clean(artifactPath))
	if err != nil {
		return nil, fmt.Errorf("failed to read smart wallet artifact: %w", err)
	}

	var payload struct {
		Bytecode string `json:"bytecode"`
	}
	if err := json.Unmarshal(contents, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal smart wallet artifact: %w", err)
	}

	bytecodeHex := strings.TrimPrefix(payload.Bytecode, "0x")
	if bytecodeHex == "" {
		return nil, errors.New("smart wallet artifact has empty creation bytecode")
	}

	bytecode, err := hex.DecodeString(bytecodeHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode smart wallet bytecode: %w", err)
	}

	return bytecode, nil
}

func deployContract(ethClient *clients.EthClient, creationBytecode []byte) (common.Address, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ethCli := ethClient.Clients["node0"]
	deployer := ethClient.Accs["acc0"]

	chainID, err := ethCli.ChainID(ctx)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to fetch chain id: %w", err)
	}

	nonce, err := ethCli.PendingNonceAt(ctx, deployer.Address)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to fetch pending nonce: %w", err)
	}

	gasFeeCap := big.NewInt(20_000_000_000)
	gasTipCap := big.NewInt(1_000_000_000)
	gasLimit := uint64(3_000_000)

	txData := &ethtypes.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       gasLimit,
		Value:     big.NewInt(0),
		Data:      creationBytecode,
	}

	signer := ethtypes.LatestSignerForChainID(chainID)
	signedTx, err := ethtypes.SignNewTx(deployer.PrivKey, signer, txData)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to sign contract deployment tx: %w", err)
	}

	if err := ethCli.SendTransaction(ctx, signedTx); err != nil {
		return common.Address{}, fmt.Errorf("failed to send contract deployment tx: %w", err)
	}

	receipt, err := waitForReceipt(ctx, ethCli, signedTx.Hash())
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to fetch set code tx receipt: %w", err)
	}

	if receipt.Status != 1 {
		return common.Address{}, fmt.Errorf("set code tx reverted: %s", signedTx.Hash())
	}

	return receipt.ContractAddress, nil
}

func waitForReceipt(ctx context.Context, provider receiptProvider, hash common.Hash) (*ethtypes.Receipt, error) {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		receipt, err := provider.TransactionReceipt(ctx, hash)
		if err == nil {
			return receipt, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, fmt.Errorf("timed out waiting for receipt %s", hash.Hex())
}

type receiptProvider interface {
	TransactionReceipt(ctx context.Context, hash common.Hash) (*ethtypes.Receipt, error)
}
