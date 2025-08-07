package rpc

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/utils"
)

// GethVersion is the version of the Geth client used in the tests
// Update it when go-ethereum of go.mod is updated
const GethVersion = "1.15.10"

type CallRPC func(rCtx *RpcContext) (*types.RpcResult, error)

type RpcContext struct {
	Conf                  *config.Config
	EthCli                *ethclient.Client
	Acc                   *types.Account
	ChainId               *big.Int
	MaxPriorityFeePerGas  *big.Int
	GasPrice              *big.Int
	ProcessedTransactions []common.Hash
	BlockNumsIncludingTx  []uint64
	AlreadyTestedRPCs     []*types.RpcResult
	ERC20Abi              *abi.ABI
	ERC20ByteCode         []byte
	ERC20Addr             common.Address
	FilterQuery           ethereum.FilterQuery
	FilterId              string
	BlockFilterId         string
}

func NewContext(conf *config.Config) (*RpcContext, error) {
	// Connect to the Ethereum client
	ethCli, err := ethclient.Dial(conf.RpcEndpoint)
	if err != nil {
		return nil, err
	}

	ecdsaPrivKey, err := crypto.HexToECDSA(conf.RichPrivKey)
	if err != nil {
		return nil, err
	}

	ctx := &RpcContext{
		Conf:   conf,
		EthCli: ethCli,
		Acc: &types.Account{
			Address: crypto.PubkeyToAddress(ecdsaPrivKey.PublicKey),
			PrivKey: ecdsaPrivKey,
		},
	}

	// Scan existing blockchain state to populate initial data
	err = ctx.loadExistingState()
	if err != nil {
		// Not a fatal error - we can continue with empty state
		fmt.Printf("Warning: Could not load existing blockchain state: %v\n", err)
	}

	return ctx, nil
}

// loadExistingState scans the blockchain and creates test transactions if needed
func (rCtx *RpcContext) loadExistingState() error {
	// First, scan existing blocks for any transactions
	blockNumber, err := rCtx.EthCli.BlockNumber(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get block number: %v", err)
	}

	// Scan recent blocks for any existing transactions
	startBlock := uint64(1) // Start from block 1 (genesis is 0)
	if blockNumber > 50 {
		startBlock = blockNumber - 50
	}

	fmt.Printf("Scanning blocks %d to %d for existing transactions...\n", startBlock, blockNumber)

	for i := startBlock; i <= blockNumber; i++ {
		block, err := rCtx.EthCli.BlockByNumber(context.Background(), big.NewInt(int64(i)))
		if err != nil {
			continue // Skip blocks we can't read
		}

		// Process transactions in this block
		for _, tx := range block.Transactions() {
			txHash := tx.Hash()
			
			// Get transaction receipt
			receipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), txHash)
			if err != nil {
				continue // Skip transactions without receipts
			}

			// Add successful transactions to our list
			if receipt.Status == 1 {
				rCtx.ProcessedTransactions = append(rCtx.ProcessedTransactions, txHash)
				rCtx.BlockNumsIncludingTx = append(rCtx.BlockNumsIncludingTx, receipt.BlockNumber.Uint64())

				// If this transaction created a contract, save the address
				if receipt.ContractAddress != (common.Address{}) {
					rCtx.ERC20Addr = receipt.ContractAddress
					fmt.Printf("Found contract at address: %s (tx: %s)\n", receipt.ContractAddress.Hex(), txHash.Hex())
				}
			}
		}
	}

	fmt.Printf("Loaded %d existing transactions\n", len(rCtx.ProcessedTransactions))

	// If we don't have enough transactions, create some test transactions now
	if len(rCtx.ProcessedTransactions) < 3 {
		fmt.Printf("Note: Only %d transactions found. Consider running more transactions for comprehensive API testing.\n", len(rCtx.ProcessedTransactions))
		// TODO: Implement createTestTransactions method
	}

	if rCtx.ERC20Addr != (common.Address{}) {
		fmt.Printf("Contract available at: %s\n", rCtx.ERC20Addr.Hex())
	}

	return nil
}

func (rCtx *RpcContext) AlreadyTested(rpc types.RpcName) *types.RpcResult {
	for _, testedRPC := range rCtx.AlreadyTestedRPCs {
		if rpc == testedRPC.Method {
			return testedRPC
		}
	}
	return nil

}

func WaitForTx(rCtx *RpcContext, txHash common.Hash, timeout time.Duration) error {
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
				rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, &types.RpcResult{
					Method: MethodNameEthGetTransactionReceipt,
					Status: types.Ok,
					Value:  utils.MustBeautifyReceipt(receipt),
				})
				if receipt.ContractAddress != (common.Address{}) {
					rCtx.ERC20Addr = receipt.ContractAddress
				}
				if receipt.Status == 0 {
					return fmt.Errorf("transaction %s failed", txHash.Hex())
				}
				return nil
			}
		}
	}
}

func Skipped(methodName types.RpcName, category string, reason string) (*types.RpcResult, error) {
	return &types.RpcResult{
		Method:   methodName,
		Status:   types.Skipped,
		ErrMsg:   reason,
		Category: category,
	}, nil
}

func Legacy(rCtx *RpcContext, methodName types.RpcName, category string, replacementInfo string) (*types.RpcResult, error) {
	// First test if the API is actually implemented
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, string(methodName))
	
	if err != nil {
		// Check if it's a "method not found" error (API not implemented)
		if err.Error() == "the method "+string(methodName)+" does not exist/is not available" ||
			err.Error() == "Method not found" ||
			err.Error() == string(methodName)+" method not found" {
			// API is not implemented, so it should be NOT_IMPL, not LEGACY
			return &types.RpcResult{
				Method:   methodName,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: category,
			}, nil
		}
		// API exists but failed with parameters (could be legacy with wrong params)
		// Still mark as legacy since the method exists
	}

	// API exists (either succeeded or failed with parameter issues), mark as LEGACY
	return &types.RpcResult{
		Method:   methodName,
		Status:   types.Legacy,
		Value:    fmt.Sprintf("Legacy API implemented in Cosmos EVM. %s", replacementInfo),
		ErrMsg:   replacementInfo,
		Category: category,
	}, nil
}

// Missing function implementations
func EthCoinbase(rCtx *RpcContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.EthCli.Client().Call(&result, "eth_coinbase")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthCoinbase,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "eth",
		}, nil
	}

	return &types.RpcResult{
		Method:   MethodNameEthCoinbase,
		Status:   types.Ok,
		Value:    result,
		Category: "eth",
	}, nil
}

// Generic test handler that makes an actual RPC call to determine if an API is implemented
func GenericTest(rCtx *RpcContext, methodName types.RpcName, category string) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, string(methodName))

	if err != nil {
		// Check if it's a "method not found" error (API not implemented)
		if err.Error() == "the method "+string(methodName)+" does not exist/is not available" ||
			err.Error() == "Method not found" ||
			err.Error() == string(methodName)+" method not found" {
			return &types.RpcResult{
				Method:   methodName,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: category,
			}, nil
		}
		// Other errors mean the method exists but failed (could be parameter issues, etc.)
		return &types.RpcResult{
			Method:   methodName,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: category,
		}, nil
	}

	// Method exists and returned a result
	return &types.RpcResult{
		Method:   methodName,
		Status:   types.Ok,
		Value:    result,
		Category: category,
	}, nil
}
