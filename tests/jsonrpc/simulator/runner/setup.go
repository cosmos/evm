package runner

import (
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/contracts"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/utils"
)

// Setup performs the complete setup: fund geth accounts, deploy contracts, and mint tokens
func Setup() (*types.RPCContext, error) {
	// Load configuration from conf.yaml
	conf := config.MustLoadConfig()

	// Create RPC context
	rCtx, err := types.NewRPCContext(conf)
	if err != nil {
		log.Fatalf("Failed to create context: %v", err)
	}

	log.Println("Step 1: Funding geth dev accounts...")
	err = fundGethAccounts(rCtx)
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
	err = mintTokensOnBothNetworks(rCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to mint tokens: %w", err)
	}
	log.Println("✓ Token minting completed successfully")

	log.Println("Step 4: Verifying state synchronization...")
	err = utils.VerifyTokenBalances(rCtx)
	if err != nil {
		return nil, fmt.Errorf("state verification failed: %w", err)
	}
	log.Println("✓ State synchronization verified")

	// create filter query for ERC20 transfers
	log.Println("Step 5: Creating filter for ERC20 transfers...")
	err = newFilter(rCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to create filter: %w", err)
	}
	log.Printf("Created filter for ERC20 transfers: evmd=%s, geth=%s\n", rCtx.Evmd.FilterID, rCtx.Geth.FilterID)

	return rCtx, nil
}

// fundGethAccounts funds the standard dev accounts in geth using coinbase balance
func fundGethAccounts(rCtx *types.RPCContext) error {
	// Fund the accounts
	results, err := utils.FundStandardAccounts(rCtx, true)
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
	time.Sleep(3 * time.Second)

	// Check final balances
	fmt.Println("\nChecking final balances:")
	balances, err := utils.Balances(rCtx.Geth.Client)
	if err != nil {
		return fmt.Errorf("failed to check balances: %w", err)
	}

	for name, balance := range balances {
		address := utils.StandardDevAccounts[name]
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
	addr, txHash, blockNum, err := utils.DeployContract(rCtx, contractBytecode, false)
	if err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}
	rCtx.Evmd.ERC20Addr = addr
	rCtx.Evmd.ERC20Abi = &parsedABI
	rCtx.Evmd.ERC20ByteCode = contractBytecode
	rCtx.Evmd.BlockNumsIncludingTx = append(rCtx.Evmd.BlockNumsIncludingTx, blockNum.Uint64())
	rCtx.Evmd.ProcessedTransactions = append(rCtx.Evmd.ProcessedTransactions, common.HexToHash(txHash))

	addr, txHash, blockNum, err = utils.DeployContract(rCtx, contractBytecode, true)
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

// mintTokensOnBothNetworks distributes ERC20 tokens to specified accounts on both evmd and geth
func mintTokensOnBothNetworks(rCtx *types.RPCContext) error {
	fmt.Printf("\n=== Distributing ERC20 Tokens for State Synchronization ===\n")

	// Define accounts and amounts to distribute (dev0 keeps remaining balance)
	distributionTargets := map[string]*big.Int{
		config.Dev1PrivateKey: new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18)), // 1000 tokens
		config.Dev2PrivateKey: new(big.Int).Mul(big.NewInt(500), big.NewInt(1e18)),  // 500 tokens
		config.Dev3PrivateKey: new(big.Int).Mul(big.NewInt(750), big.NewInt(1e18)),  // 750 tokens
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
		_, recipientAddr, err := utils.GetPrivateKeyAndAddress(privateKeyHex)
		if err != nil {
			continue
		}

		fmt.Printf("  Transferring %s tokens to %s...\n",
			new(big.Int).Div(amount, big.NewInt(1e18)).String(), // Convert to readable units
			recipientAddr.Hex()[:10]+"...")

		// Transfer tokens from owner to recipient
		receipt, err := utils.TransferTokensToAccount(rCtx, recipientAddr, amount, ownerPrivateKey, isGeth)
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

func newFilter(rCtx *types.RPCContext) error {
	filterQuery, filterID, err := utils.NewERC20FilterLogs(rCtx, false)
	if err != nil {
		return fmt.Errorf("failed to create evmd filter: %w", err)
	}
	rCtx.Evmd.FilterID = filterID
	rCtx.Evmd.FilterQuery = filterQuery

	// Create filter on geth
	filterQuery, filterID, err = utils.NewERC20FilterLogs(rCtx, true)
	if err != nil {
		return fmt.Errorf("failed to create evmd filter: %w", err)
	}
	rCtx.Geth.FilterID = filterID
	rCtx.Geth.FilterQuery = filterQuery

	return nil
}
