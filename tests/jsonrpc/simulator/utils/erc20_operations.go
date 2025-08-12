package utils

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

// ERC20MintResult represents the result of a token minting operation
type ERC20MintResult struct {
	Contract    common.Address `json:"contract"`
	Recipient   common.Address `json:"recipient"`
	Amount      *big.Int       `json:"amount"`
	TxHash      common.Hash    `json:"txHash"`
	GasUsed     uint64         `json:"gasUsed"`
	BlockNumber *big.Int       `json:"blockNumber"`
	Success     bool           `json:"success"`
	Error       string         `json:"error,omitempty"`
}

// MintTokensOnBothNetworks distributes ERC20 tokens to specified accounts on both evmd and geth
func MintTokensOnBothNetworks(rCtx *types.RPCContext, evmdURL, gethURL string) error {
	fmt.Printf("\n=== Distributing ERC20 Tokens for State Synchronization ===\n")

	// Define accounts and amounts to distribute (dev0 keeps remaining balance)
	distributionTargets := map[string]*big.Int{
		config.Dev1PrivateKey: new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18)), // 1000 tokens
		config.Dev2PrivateKey: new(big.Int).Mul(big.NewInt(500), big.NewInt(1e18)),  // 500 tokens
		config.Dev3PrivateKey: new(big.Int).Mul(big.NewInt(750), big.NewInt(1e18)),  // 750 tokens
		// dev0 (contract deployer) keeps remaining tokens - no need to transfer to self
	}

	// Distribute on evmd (from dev0 who deployed the contract)
	fmt.Printf("Distributing tokens on evmd (contract: %s)...\n", rCtx.Evmd.ERC20Addr.Hex())
	evmdReceipts, err := distributeTokensOnNetwork(rCtx, distributionTargets, config.Dev0PrivateKey, false)
	if err != nil {
		return fmt.Errorf("failed to distribute tokens on evmd: %w", err)
	}

	// Distribute on geth (need to first transfer from coinbase to dev1, then distribute)
	fmt.Printf("Distributing tokens on geth (contract: %s)...\n", rCtx.Geth.ERC20Addr.Hex())
	gethReceipts, err := distributeTokensOnNetwork(rCtx, distributionTargets, config.Dev0PrivateKey, true)
	if err != nil {
		return fmt.Errorf("failed to distribute tokens on geth: %w", err)
	}

	// Count successful distributions
	evmdSuccess := 0
	gethSuccess := 0
	for _, receipt := range evmdReceipts {
		if receipt.Status == 1 {
			evmdSuccess++
		}
	}
	for _, receipt := range gethReceipts {
		if receipt.Status == 1 {
			gethSuccess++
		}
	}

	fmt.Printf("\nDistribution summary: evmd (%d/%d), geth (%d/%d)\n",
		evmdSuccess, len(evmdReceipts), gethSuccess, len(gethReceipts))

	if evmdSuccess != len(distributionTargets) || gethSuccess != len(distributionTargets) {
		return fmt.Errorf("distribution failed - not all accounts received tokens")
	}

	fmt.Printf("✓ Token distribution completed successfully on both networks\n")
	return nil
}

// distributeTokensOnNetwork transfers tokens from owner to multiple accounts on a single network
func distributeTokensOnNetwork(rCtx *types.RPCContext, distributionTargets map[string]*big.Int, ownerPrivateKey string, isGeth bool) (ethtypes.Receipts, error) {
	var receipts ethtypes.Receipts

	for privateKeyHex, amount := range distributionTargets {
		// Get recipient address
		_, recipientAddr, err := GetPrivateKeyAndAddress(privateKeyHex)
		if err != nil {
			continue
		}

		fmt.Printf("  Transferring %s tokens to %s...\n",
			new(big.Int).Div(amount, big.NewInt(1e18)).String(), // Convert to readable units
			recipientAddr.Hex()[:10]+"...")

		// Transfer tokens from owner to recipient
		receipt, err := transferTokensToAccount(rCtx, recipientAddr, amount, ownerPrivateKey, isGeth)
		if err != nil {
			fmt.Printf("    ✗ Error: %v\n", err)
		} else {
			fmt.Printf("    ✓ Success (tx: %s)\n", receipt.TxHash.Hex()[:10]+"...")
		}

		receipts = append(receipts, receipt)

		// Small delay between transfers
		time.Sleep(200 * time.Millisecond)
	}

	return receipts, nil
}

// transferTokensToAccount transfers ERC20 tokens from owner to a specific account
func transferTokensToAccount(rCtx *types.RPCContext, recipient common.Address, amount *big.Int, ownerPrivateKey string, isGeth bool) (*ethtypes.Receipt, error) {
	ethCli := rCtx.Evmd
	if isGeth {
		ethCli = rCtx.Geth
	}

	// Create transaction
	data, err := ethCli.ERC20Abi.Pack("transfer", recipient, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to pack transfer data: %w", err)
	}
	return SendRawTransaction(rCtx, ownerPrivateKey, ethCli.ERC20Addr, big.NewInt(0), data, isGeth)
}

// VerifyTokenBalances verifies that token balances are identical on both networks
func VerifyTokenBalances(rCtx *types.RPCContext, evmdURL, gethURL string) error {
	fmt.Printf("\n=== Verifying Token Balance Synchronization ===\n")

	accounts := []string{config.Dev0PrivateKey, config.Dev1PrivateKey, config.Dev2PrivateKey, config.Dev3PrivateKey}

	for _, privateKeyHex := range accounts {
		_, addr, err := GetPrivateKeyAndAddress(privateKeyHex)
		if err != nil {
			return fmt.Errorf("failed to get address for verification: %w", err)
		}

		// Get balance on evmd
		evmdBalance, err := getTokenBalance(evmdURL, rCtx.Evmd.ERC20Addr, addr)
		if err != nil {
			return fmt.Errorf("failed to get evmd balance for %s: %w", addr.Hex(), err)
		}

		// Get balance on geth
		gethBalance, err := getTokenBalance(gethURL, rCtx.Geth.ERC20Addr, addr)
		if err != nil {
			return fmt.Errorf("failed to get geth balance for %s: %w", addr.Hex(), err)
		}

		// Compare balances
		if evmdBalance.Cmp(gethBalance) != 0 {
			return fmt.Errorf("balance mismatch for %s: evmd=%s, geth=%s",
				addr.Hex(), evmdBalance.String(), gethBalance.String())
		}

		readableBalance := new(big.Int).Div(evmdBalance, big.NewInt(1e18))
		fmt.Printf("  ✓ %s: %s tokens (identical on both networks)\n",
			addr.Hex()[:10]+"...", readableBalance.String())
	}

	fmt.Printf("✓ All token balances verified as identical\n")
	return nil
}

// getTokenBalance gets the ERC20 token balance for an address
func getTokenBalance(rpcURL string, contractAddr, account common.Address) (*big.Int, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}

	// balanceOf(address) function signature: 0x70a08231
	balanceOfSig := crypto.Keccak256([]byte("balanceOf(address)"))[:4]
	data := make([]byte, 36) // 4 bytes signature + 32 bytes address

	// Function signature
	copy(data[0:4], balanceOfSig)

	// Account address (left-padded to 32 bytes)
	copy(data[16:36], account.Bytes())

	// Call the contract
	msg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: data,
	}
	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		return nil, err
	}

	// Convert result to big.Int
	balance := new(big.Int).SetBytes(result)
	return balance, nil
}
