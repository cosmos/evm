package utils

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
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
	err = fundGethAccounts(gethURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fund geth accounts: %w", err)
	}
	log.Println("✓ Geth accounts funded successfully")

	log.Println("Step 2: Deploying ERC20 contracts to both networks...")
	result, err := deployContracts(evmdURL, gethURL)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy contracts: %w", err)
	}
	// set rpc context
	rCtx.Evmd.ERC20Addr = result.EvmdDeployment.Address
	rCtx.Evmd.ERC20Abi = result.EvmdDeployment.ABI
	rCtx.Evmd.ERC20ByteCode = result.EvmdDeployment.ByteCode
	rCtx.Evmd.BlockNumsIncludingTx = append(rCtx.Evmd.BlockNumsIncludingTx, result.EvmdDeployment.BlockNumber.Uint64())
	rCtx.Evmd.ProcessedTransactions = append(rCtx.Evmd.ProcessedTransactions, result.EvmdDeployment.TxHash)

	rCtx.Geth.ERC20Addr = result.GethDeployment.Address
	rCtx.Geth.ERC20Abi = result.GethDeployment.ABI
	rCtx.Geth.ERC20ByteCode = result.GethDeployment.ByteCode
	rCtx.Geth.BlockNumsIncludingTx = append(rCtx.Geth.BlockNumsIncludingTx, result.GethDeployment.BlockNumber.Uint64())
	rCtx.Geth.ProcessedTransactions = append(rCtx.Geth.ProcessedTransactions, result.GethDeployment.TxHash)
	log.Println("✓ Contracts deployed successfully")

	log.Println("Step 3: Minting ERC20 tokens to synchronize state...")
	err = MintTokensOnBothNetworks(evmdURL, gethURL,
		result.EvmdDeployment.Address, result.GethDeployment.Address)
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
	err = VerifyTokenBalances(evmdURL, gethURL,
		result.EvmdDeployment.Address, result.GethDeployment.Address)
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
		result.EvmdDeployment.ABI = &parsedABI
		result.EvmdDeployment.ByteCode = contractBytecode
	}
	if result.GethDeployment != nil {
		fmt.Printf("  geth: %s (tx: %s, block: %s)\n",
			result.GethDeployment.Address.Hex(),
			result.GethDeployment.TxHash.Hex(),
			result.GethDeployment.BlockNumber.String())
		result.GethDeployment.ABI = &parsedABI
		result.GethDeployment.ByteCode = contractBytecode
	}

	return result, nil
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

func NewERC20FilterLogs(rCtx *types.RPCContext, isGeth bool) (ethereum.FilterQuery, string, error) {
	ethCli := rCtx.Evmd
	if isGeth {
		ethCli = rCtx.Geth
	}

	fErc20Transfer := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(0), // Start from genesis
		Addresses: []common.Address{ethCli.ERC20Addr},
		Topics: [][]common.Hash{
			{ethCli.ERC20Abi.Events["Transfer"].ID}, // Filter for Transfer event
		},
	}

	// Create filter on evmd
	args, err := ToFilterArg(fErc20Transfer)
	if err != nil {
		return fErc20Transfer, "", fmt.Errorf("failed to create filter args: %w", err)
	}
	var evmdFilterID string
	if err = ethCli.RPCClient().CallContext(context.Background(), &evmdFilterID, "eth_newFilter", args); err != nil {
		return fErc20Transfer, "", fmt.Errorf("failed to create filter on evmd: %w", err)
	}

	return fErc20Transfer, evmdFilterID, nil
}
