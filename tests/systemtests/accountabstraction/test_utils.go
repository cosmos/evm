//go:build system_test

package accountabstraction

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cosmos/evm/tests/systemtests/clients"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/tidwall/gjson"
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

func loadContractCreationBytecode(filePath string) ([]byte, error) {
	_, caller, _, ok := runtime.Caller(0)
	if !ok {
		return nil, errors.New("failed to resolve caller for smart wallet artifact")
	}

	artifactPath := filepath.Join(filepath.Dir(caller), filePath)
	contents, err := os.ReadFile(filepath.Clean(artifactPath))
	if err != nil {
		return nil, fmt.Errorf("failed to read smart wallet artifact: %w", err)
	}

	bytecodeHex := gjson.GetBytes(contents, "bytecode.object").String()
	if bytecodeHex == "" {
		bytecodeHex = gjson.GetBytes(contents, "bytecode").String()
	}
	if bytecodeHex == "" {
		return nil, errors.New("smart wallet artifact has empty creation bytecode")
	}

	bytecodeHex = strings.TrimPrefix(bytecodeHex, "0x")
	if bytecodeHex == "" {
		return nil, errors.New("smart wallet artifact has empty creation bytecode")
	}

	bytecode, err := hex.DecodeString(bytecodeHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode smart wallet bytecode: %w", err)
	}

	return bytecode, nil
}

func loadContractABI(filePath string) (abi.ABI, error) {
	_, caller, _, ok := runtime.Caller(0)
	if !ok {
		return abi.ABI{}, errors.New("failed to resolve caller for contract artifact")
	}

	artifactPath := filepath.Join(filepath.Dir(caller), filePath)
	contents, err := os.ReadFile(filepath.Clean(artifactPath))
	if err != nil {
		return abi.ABI{}, fmt.Errorf("failed to read contract artifact: %w", err)
	}

	abiField := gjson.GetBytes(contents, "abi")
	if !abiField.Exists() {
		return abi.ABI{}, errors.New("contract artifact missing abi field")
	}

	parsedABI, err := abi.JSON(strings.NewReader(abiField.Raw))
	if err != nil {
		return abi.ABI{}, fmt.Errorf("failed to parse contract ABI: %w", err)
	}

	return parsedABI, nil
}

func deployContract(ethClient *clients.EthClient, deployer *clients.EthAccount, creationBytecode []byte) (common.Address, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ethCli := ethClient.Clients["node0"]

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

	receipt, err := ethClient.WaitForCommit("node0", signedTx.Hash().Hex(), time.Second*10)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to fetch set code tx receipt: %w", err)
	}

	if receipt.Status != 1 {
		return common.Address{}, fmt.Errorf("set code tx reverted: %s", signedTx.Hash())
	}

	return receipt.ContractAddress, nil
}
