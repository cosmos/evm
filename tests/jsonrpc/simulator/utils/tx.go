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
			receipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), txHash)
			if err != nil && !errors.Is(err, ethereum.NotFound) {
				return err
			}
			if err == nil {
				rCtx.Evmd.ProcessedTransactions = append(rCtx.Evmd.ProcessedTransactions, txHash)
				rCtx.Evmd.BlockNumsIncludingTx = append(rCtx.Evmd.BlockNumsIncludingTx, receipt.BlockNumber.Uint64())
				if receipt.ContractAddress != (common.Address{}) {
					rCtx.Evmd.ERC20Addr = receipt.ContractAddress
				}
				if receipt.Status == 0 {
					return fmt.Errorf("transaction %s failed", txHash.Hex())
				}

				// If dual API comparison is enabled, create equivalent transaction on geth
				if rCtx.EnableComparison && rCtx.Geth != nil {
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
	evmdTx, isPending, err := rCtx.Evmd.TransactionByHash(context.Background(), evmdTxHash)
	if err != nil || isPending {
		log.Printf("Warning: Could not get evmd transaction %s for geth replication: %v", evmdTxHash.Hex(), err)
		return
	}

	// Get the transaction receipt to understand what it did
	evmdReceipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), evmdTxHash)
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

	// Check and fund account if necessary
	fromAddr := crypto.PubkeyToAddress(privateKey.PublicKey)
	if err := ensureGethAccountFunding(rCtx, fromAddr); err != nil {
		return common.Hash{}, fmt.Errorf("failed to fund geth account: %w", err)
	}

	// Get chain ID for geth
	chainID, err := rCtx.Geth.ChainID(context.Background())
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get geth chain ID: %w", err)
	}

	// Get nonce for geth
	nonce, err := rCtx.Geth.PendingNonceAt(context.Background(), fromAddr)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get nonce: %w", err)
	}

	// Get gas price for geth
	gasPrice, err := rCtx.Geth.SuggestGasPrice(context.Background())
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
		if rCtx.Geth.ERC20Addr != (common.Address{}) && evmdTx.To() != nil && *evmdTx.To() == rCtx.Evmd.ERC20Addr {
			toAddr = rCtx.Geth.ERC20Addr
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
	err = rCtx.Geth.SendTransaction(context.Background(), signedTx)
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
			receipt, err := rCtx.Geth.TransactionReceipt(context.Background(), txHash)
			if err != nil && !errors.Is(err, ethereum.NotFound) {
				return err
			}
			if err == nil {
				// Update geth state in the context
				rCtx.Geth.ProcessedTransactions = append(rCtx.Geth.ProcessedTransactions, txHash)
				rCtx.Geth.BlockNumsIncludingTx = append(rCtx.Geth.BlockNumsIncludingTx, receipt.BlockNumber.Uint64())

				// Update geth contract address if this was a deployment
				if receipt.ContractAddress != (common.Address{}) {
					rCtx.Geth.ERC20Addr = receipt.ContractAddress
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

// ensureGethAccountFunding ensures a geth account has sufficient funds for transactions
func ensureGethAccountFunding(rCtx *types.RPCContext, targetAddr common.Address) error {
	// Check current balance
	balance, err := rCtx.Geth.BalanceAt(context.Background(), targetAddr, nil)
	if err != nil {
		return fmt.Errorf("failed to get balance: %w", err)
	}

	// If balance is sufficient (>= 10 ETH), no need to fund
	minBalance := new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)) // 10 ETH
	if balance.Cmp(minBalance) >= 0 {
		return nil
	}

	// Silently funding geth account

	// Get geth's pre-funded dev account
	var accounts []string
	err = rCtx.Geth.RPCClient().Call(&accounts, "eth_accounts")
	if err != nil || len(accounts) == 0 {
		return fmt.Errorf("no funded accounts available in geth: %w", err)
	}

	devAccount := common.HexToAddress(accounts[0])
	// Using geth dev account for funding

	// Transfer 100 ETH from dev account to target account
	transferAmount := new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)) // 100 ETH

	// Send transaction via RPC (since geth dev account is unlocked)
	var txHash common.Hash
	err = rCtx.Geth.RPCClient().Call(&txHash, "eth_sendTransaction", map[string]interface{}{
		"from":  devAccount.Hex(),
		"to":    targetAddr.Hex(),
		"value": fmt.Sprintf("0x%x", transferAmount),
		"gas":   "0x5208", // 21000 gas
	})

	if err != nil {
		return fmt.Errorf("failed to send funding transaction: %w", err)
	}

	// Wait for the funding transaction to be mined
	err = waitForGethTx(rCtx, txHash, 30*time.Second)
	if err != nil {
		return fmt.Errorf("funding transaction failed: %w", err)
	}

	// Successfully funded geth account

	return nil
}
