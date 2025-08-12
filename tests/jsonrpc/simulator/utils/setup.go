package utils

import (
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/contracts"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

// RunSetup performs the complete setup: fund geth accounts, deploy contracts, and mint tokens
func RunSetup() (*types.RPCContext, error) {
	// Load configuration from conf.yaml
	conf := config.MustLoadConfig("config.yaml")

	// Create RPC context
	rCtx, err := types.NewRPCContext(conf)
	if err != nil {
		log.Fatalf("Failed to create context: %v", err)
	}

	// URLs for both networks
	evmdURL := "http://localhost:8545"
	gethURL := "http://localhost:8547"

	log.Println("Step 1: Funding geth dev accounts...")
	err = fundGethAccounts(rCtx, gethURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fund geth accounts: %w", err)
	}
	log.Println("✓ Geth accounts funded successfully")

	log.Println("Step 2: Deploying ERC20 contracts to both networks...")
	err = deployContracts(rCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy contracts: %w", err)
	}

	log.Println("✓ Contracts deployed successfully")

	log.Println("Step 3: Minting ERC20 tokens to synchronize state...")
	err = MintTokensOnBothNetworks(rCtx, evmdURL, gethURL)
	if err != nil {
		return nil, fmt.Errorf("failed to mint tokens: %w", err)
	}
	log.Println("✓ Token minting completed successfully")

	// create filter query for ERC20 transfers
	log.Println("Step 4: Creating filter for ERC20 transfers...")
	filterQuery, filterID, err := NewERC20FilterLogs(rCtx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create evmd filter: %w", err)
	}
	rCtx.Evmd.FilterID = filterID
	rCtx.Evmd.FilterQuery = filterQuery

	// Create filter on geth
	filterQuery, filterID, err = NewERC20FilterLogs(rCtx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create evmd filter: %w", err)
	}
	rCtx.Geth.FilterID = filterID
	rCtx.Geth.FilterQuery = filterQuery
	log.Printf("Created filter for ERC20 transfers: evmd=%s, geth=%s\n", rCtx.Evmd.FilterID, rCtx.Geth.FilterID)

	log.Println("Step 4: Verifying state synchronization...")
	err = VerifyTokenBalances(rCtx, evmdURL, gethURL)
	if err != nil {
		return nil, fmt.Errorf("state verification failed: %w", err)
	}
	log.Println("✓ State synchronization verified")

	// run transaction generation
	log.Println("Step 5: Generating test transactions...")
	err = RunTransactionGeneration(rCtx)
	if err != nil {
		log.Fatalf("Transaction generation failed: %v", err)
	}
	log.Println("✓ Test transactions generated successfully")

	return rCtx, nil
}

// fundGethAccounts funds the standard dev accounts in geth using coinbase balance
func fundGethAccounts(rCtx *types.RPCContext, gethURL string) error {
	// Connect to geth
	client, err := ethclient.Dial(gethURL)
	if err != nil {
		return fmt.Errorf("failed to connect to geth at %s: %w", gethURL, err)
	}

	// Fund the accounts
	results, err := fundStandardAccounts(rCtx, client, gethURL)
	if err != nil {
		return fmt.Errorf("failed to fund accounts: %w", err)
	}

	// Print results
	fmt.Println("\nFunding Results:")
	for _, result := range results {
		if result.Success {
			fmt.Printf("✓ %s (%s): %s ETH - TX: %s\n", result.Account, result.Address.Hex(), "1000", result.TxHash.Hex())
		} else {
			fmt.Printf("✗ %s (%s): Failed - %s\n", result.Account, result.Address.Hex(), result.Error)
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
func deployContracts(rCtx *types.RPCContext) error {
	// Read the ABI file
	abiFile, err := os.ReadFile("contracts/ERC20Token.abi")
	if err != nil {
		log.Fatalf("Failed to read ABI file: %v", err)
	}
	// Parse the ABI
	parsedABI, err := abi.JSON(strings.NewReader(string(abiFile)))
	if err != nil {
		log.Fatalf("Failed to parse ERC20 ABI: %v", err)
	}

	contractBytecode := common.FromHex(string(contracts.ContractByteCode))
	addr, txHash, blockNum, err := DeployERC20Contract(rCtx, contractBytecode, false)
	if err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}
	rCtx.Evmd.ERC20Addr = addr
	rCtx.Evmd.ERC20Abi = &parsedABI
	rCtx.Evmd.ERC20ByteCode = contractBytecode
	rCtx.Evmd.BlockNumsIncludingTx = append(rCtx.Geth.BlockNumsIncludingTx, blockNum.Uint64())
	rCtx.Evmd.ProcessedTransactions = append(rCtx.Geth.ProcessedTransactions, common.HexToHash(txHash))

	addr, txHash, blockNum, err = DeployERC20Contract(rCtx, contractBytecode, true)
	if err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}
	rCtx.Geth.ERC20Addr = addr
	rCtx.Geth.ERC20Abi = &parsedABI
	rCtx.Geth.ERC20ByteCode = contractBytecode
	rCtx.Geth.BlockNumsIncludingTx = append(rCtx.Geth.BlockNumsIncludingTx, blockNum.Uint64())
	rCtx.Geth.ProcessedTransactions = append(rCtx.Geth.ProcessedTransactions, common.HexToHash(txHash))

	fmt.Printf("\n✓ ERC20 Contract Deployment Summary:\n")
	return nil
}

// RunTransactionGeneration generates test transactions on both networks
func RunTransactionGeneration(rCtx *types.RPCContext) error {
	// URLs for both networks
	evmdURL := "http://localhost:8545"
	gethURL := "http://localhost:8547"

	log.Println("Step 1: Loading contract addresses from registry...")

	evmdContract := rCtx.Evmd.ERC20Addr
	gethContract := rCtx.Geth.ERC20Addr

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
