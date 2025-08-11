package utils

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func WaitForTx(rCtx *types.RPCContext, txHash common.Hash, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond) // Check every 500ms
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout exceeded while waiting for transaction %s", txHash.Hex())
		case <-ticker.C:
			receipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), txHash)
			if err != nil && !errors.Is(err, ethereum.NotFound) {
				return err
			}
			if err == nil {
				rCtx.ProcessedTransactions = append(rCtx.ProcessedTransactions, txHash)
				rCtx.BlockNumsIncludingTx = append(rCtx.BlockNumsIncludingTx, receipt.BlockNumber.Uint64())
				if receipt.ContractAddress != (common.Address{}) {
					rCtx.ERC20Addr = receipt.ContractAddress
				}
				if receipt.Status == 0 {
					return fmt.Errorf("transaction %s failed", txHash.Hex())
				}

				// If dual API comparison is enabled, create equivalent transaction on geth
				if rCtx.EnableComparison && rCtx.GethCli != nil {
					go createEquivalentGethTransaction(rCtx, txHash)
				}

				return nil
			}
		}
	}
}

// createEquivalentGethTransaction creates a similar transaction on geth for comparison
func createEquivalentGethTransaction(rCtx *types.RPCContext, evmdTxHash common.Hash) {
	// Get the original transaction from evmd
	evmdTx, isPending, err := rCtx.EthCli.TransactionByHash(context.Background(), evmdTxHash)
	if err != nil || isPending {
		log.Printf("Warning: Could not get evmd transaction %s for geth replication: %v", evmdTxHash.Hex(), err)
		return
	}

	// Get the transaction receipt to understand what it did
	evmdReceipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), evmdTxHash)
	if err != nil {
		log.Printf("Warning: Could not get evmd receipt %s for geth replication: %v", evmdTxHash.Hex(), err)
		return
	}

	// Create equivalent transaction on geth
	gethTxHash, err := createSimilarGethTransaction(rCtx, evmdTx, evmdReceipt)
	if err != nil {
		log.Printf("Warning: Could not create equivalent geth transaction: %v", err)
		return
	}

	// Wait for the geth transaction to be mined and update context
	err = waitForGethTx(rCtx, gethTxHash, 30*time.Second)
	if err != nil {
		log.Printf("Warning: Geth transaction %s failed or timed out: %v", gethTxHash.Hex(), err)
		return
	}

	log.Printf("Successfully created equivalent geth transaction %s for evmd tx %s", 
		gethTxHash.Hex(), evmdTxHash.Hex())
}

// createSimilarGethTransaction creates a transaction on geth similar to the evmd transaction
func createSimilarGethTransaction(rCtx *types.RPCContext, evmdTx *ethtypes.Transaction, evmdReceipt *ethtypes.Receipt) (common.Hash, error) {
	// Get private key for signing
	privateKey, err := crypto.HexToECDSA(config.Dev1PrivateKey)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get private key: %w", err)
	}

	// Get chain ID for geth
	chainID, err := rCtx.GethCli.ChainID(context.Background())
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get geth chain ID: %w", err)
	}

	// Get nonce for geth
	fromAddr := crypto.PubkeyToAddress(privateKey.PublicKey)
	nonce, err := rCtx.GethCli.PendingNonceAt(context.Background(), fromAddr)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get nonce: %w", err)
	}

	// Get gas price for geth
	gasPrice, err := rCtx.GethCli.SuggestGasPrice(context.Background())
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get gas price: %w", err)
	}

	var gethTx *ethtypes.Transaction

	// Create different types of transactions based on the original
	if evmdReceipt.ContractAddress != (common.Address{}) {
		// This was a contract deployment
		if len(evmdTx.Data()) > 0 {
			gethTx = ethtypes.NewContractCreation(nonce, evmdTx.Value(), evmdTx.Gas(), gasPrice, evmdTx.Data())
		} else {
			// Simple contract deployment with basic bytecode
			gethTx = ethtypes.NewContractCreation(nonce, big.NewInt(0), 200000, gasPrice, []byte("0x60806040"))
		}
	} else if evmdTx.To() != nil {
		// This was a regular transaction
		var toAddr common.Address
		var data []byte
		
		// If it was a contract call, use geth contract if available
		if rCtx.GethERC20Addr != (common.Address{}) && evmdTx.To() != nil && *evmdTx.To() == rCtx.ERC20Addr {
			toAddr = rCtx.GethERC20Addr
			data = evmdTx.Data() // Keep the same contract call data
		} else {
			// Simple value transfer to the same address used by evmd tests
			toAddr = fromAddr // Self-transfer for simplicity
			data = nil
		}
		
		gethTx = ethtypes.NewTransaction(nonce, toAddr, evmdTx.Value(), evmdTx.Gas(), gasPrice, data)
	} else {
		// Fallback: simple value transfer
		gethTx = ethtypes.NewTransaction(nonce, fromAddr, big.NewInt(1e16), 21000, gasPrice, nil)
	}

	// Sign the transaction
	signer := ethtypes.NewEIP155Signer(chainID)
	signedTx, err := ethtypes.SignTx(gethTx, signer, privateKey)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send the transaction
	err = rCtx.GethCli.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	return signedTx.Hash(), nil
}

// waitForGethTx waits for a geth transaction and updates the RPCContext
func waitForGethTx(rCtx *types.RPCContext, txHash common.Hash, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout exceeded while waiting for geth transaction %s", txHash.Hex())
		case <-ticker.C:
			receipt, err := rCtx.GethCli.TransactionReceipt(context.Background(), txHash)
			if err != nil && !errors.Is(err, ethereum.NotFound) {
				return err
			}
			if err == nil {
				// Update geth state in the context
				rCtx.GethProcessedTransactions = append(rCtx.GethProcessedTransactions, txHash)
				rCtx.GethBlockNumsIncludingTx = append(rCtx.GethBlockNumsIncludingTx, receipt.BlockNumber.Uint64())
				
				// Update geth contract address if this was a deployment
				if receipt.ContractAddress != (common.Address{}) {
					rCtx.GethERC20Addr = receipt.ContractAddress
					log.Printf("Geth contract deployed at: %s", receipt.ContractAddress.Hex())
				}
				
				if receipt.Status == 0 {
					return fmt.Errorf("geth transaction %s failed", txHash.Hex())
				}
				return nil
			}
		}
	}
}
