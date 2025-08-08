package utils

import (
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/contracts"
)

// RunSetup performs the complete setup: fund geth accounts, deploy contracts, and mint tokens
func RunSetup() error {
	// URLs for both networks
	evmdURL := "http://localhost:8545"
	gethURL := "http://localhost:8547"

	log.Println("Step 1: Funding geth dev accounts...")
	err := fundGethAccounts(gethURL)
	if err != nil {
		return fmt.Errorf("failed to fund geth accounts: %w", err)
	}
	log.Println("✓ Geth accounts funded successfully")

	log.Println("Step 2: Deploying ERC20 contracts to both networks...")
	result, err := deployContracts(evmdURL, gethURL)
	if err != nil {
		return fmt.Errorf("failed to deploy contracts: %w", err)
	}
	log.Println("✓ Contracts deployed successfully")

	log.Println("Step 3: Minting ERC20 tokens to synchronize state...")
	err = MintTokensOnBothNetworks(evmdURL, gethURL,
		result.EvmdDeployment.Address, result.GethDeployment.Address)
	if err != nil {
		return fmt.Errorf("failed to mint tokens: %w", err)
	}
	log.Println("✓ Token minting completed successfully")

	log.Println("Step 4: Verifying state synchronization...")
	err = VerifyTokenBalances(evmdURL, gethURL,
		result.EvmdDeployment.Address, result.GethDeployment.Address)
	if err != nil {
		return fmt.Errorf("state verification failed: %w", err)
	}
	log.Println("✓ State synchronization verified")

	log.Println("Step 5: Saving contract addresses for future use...")
	err = SaveContractAddresses(
		result.EvmdDeployment.Address,
		result.GethDeployment.Address,
		"dev0")
	if err != nil {
		return fmt.Errorf("failed to save contract addresses: %w", err)
	}
	log.Println("✓ Contract addresses saved")

	return nil
}

// fundGethAccounts funds the standard dev accounts in geth using coinbase balance
func fundGethAccounts(gethURL string) error {
	// Connect to geth
	client, err := ethclient.Dial(gethURL)
	if err != nil {
		return fmt.Errorf("failed to connect to geth at %s: %w", gethURL, err)
	}

	// Fund the accounts
	results, err := FundStandardAccounts(client, gethURL)
	if err != nil {
		return fmt.Errorf("failed to fund accounts: %w", err)
	}

	// Print results
	fmt.Println("\nFunding Results:")
	for _, result := range results {
		if result.Success {
			fmt.Printf("✓ %s (%s): %s ETH - TX: %s\n",
				result.Account,
				result.Address.Hex(),
				"1000", // We know it's 1000 ETH
				result.TxHash.Hex())
		} else {
			fmt.Printf("✗ %s (%s): Failed - %s\n",
				result.Account,
				result.Address.Hex(),
				result.Error)
		}
	}

	// Wait for transactions to be mined
	fmt.Println("\nWaiting for transactions to be mined...")
	time.Sleep(15 * time.Second) // Dev mode mines every 12 seconds

	// Check final balances
	fmt.Println("\nChecking final balances:")
	balances, err := CheckAccountBalances(client)
	if err != nil {
		return fmt.Errorf("failed to check balances: %w", err)
	}

	for name, balance := range balances {
		address := StandardDevAccounts[name]
		ethBalance := new(big.Int).Div(balance, big.NewInt(1e18)) // Convert wei to ETH
		fmt.Printf("%s (%s): %s ETH\n", name, address.Hex(), ethBalance.String())
	}

	fmt.Println("\n✓ Geth dev accounts funded successfully")
	return nil
}

// deployContracts deploys the ERC20 contract to both evmd and geth
func deployContracts(evmdURL, gethURL string) (*DeploymentResult, error) {
	// The embedded .bin file contains hex-encoded text, need to decode it to bytes
	contractBytecode := common.FromHex(string(contracts.ContractByteCode))
	result, err := DeployERC20Contract(evmdURL, gethURL, contractBytecode)
	if err != nil {
		return nil, fmt.Errorf("deployment failed: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("deployment unsuccessful: %s", result.Error)
	}

	fmt.Printf("\n✓ ERC20 Contract Deployment Summary:\n")
	if result.EvmdDeployment != nil {
		fmt.Printf("  evmd: %s (tx: %s, block: %s)\n",
			result.EvmdDeployment.Address.Hex(),
			result.EvmdDeployment.TxHash.Hex(),
			result.EvmdDeployment.BlockNumber.String())
	}
	if result.GethDeployment != nil {
		fmt.Printf("  geth: %s (tx: %s, block: %s)\n",
			result.GethDeployment.Address.Hex(),
			result.GethDeployment.TxHash.Hex(),
			result.GethDeployment.BlockNumber.String())
	}

	return result, nil
}

// RunTransactionGeneration generates test transactions on both networks
func RunTransactionGeneration() error {
	// URLs for both networks
	evmdURL := "http://localhost:8545"
	gethURL := "http://localhost:8547"

	log.Println("Step 1: Loading contract addresses from registry...")

	evmdContract, gethContract, err := GetContractAddresses()
	if err != nil {
		return fmt.Errorf("failed to load contract addresses: %w", err)
	}

	log.Printf("Loaded contracts - evmd: %s, geth: %s\n", evmdContract.Hex(), gethContract.Hex())

	log.Println("Step 2: Executing transaction scenarios...")
	batch, err := ExecuteTransactionBatch(evmdURL, gethURL, evmdContract, gethContract)
	if err != nil {
		return fmt.Errorf("failed to execute transaction batch: %w", err)
	}

	log.Println("Step 3: Generating transaction summary...")
	summary := GenerateTransactionSummary(batch)
	fmt.Printf("%s\n", summary)

	// Get successful transaction hashes for potential use in API testing
	evmdHashes, gethHashes := batch.GetTransactionHashes()
	log.Printf("Generated %d evmd transaction hashes and %d geth transaction hashes\n",
		len(evmdHashes), len(gethHashes))

	return nil
}