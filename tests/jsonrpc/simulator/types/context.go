package types

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
)

type Account struct {
	Address common.Address
	PrivKey *ecdsa.PrivateKey
}

type RPCContext struct {
	Conf                  *config.Config
	EthCli                *ethclient.Client
	Acc                   *Account
	ChainID               *big.Int
	MaxPriorityFeePerGas  *big.Int
	GasPrice              *big.Int
	ProcessedTransactions []common.Hash
	BlockNumsIncludingTx  []uint64
	AlreadyTestedRPCs     []*RpcResult
	ERC20Abi              *abi.ABI
	ERC20ByteCode         []byte
	ERC20Addr             common.Address
	FilterQuery           ethereum.FilterQuery
	FilterId              string
	BlockFilterId         string
}

func NewRPCContext(conf *config.Config) (*RPCContext, error) {
	// Connect to the Ethereum client
	ethCli, err := ethclient.Dial(conf.RpcEndpoint)
	if err != nil {
		return nil, err
	}

	ecdsaPrivKey, err := crypto.HexToECDSA(conf.RichPrivKey)
	if err != nil {
		return nil, err
	}

	ctx := &RPCContext{
		Conf:   conf,
		EthCli: ethCli,
		Acc: &Account{
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

func (rCtx *RPCContext) AlreadyTested(rpc RpcName) *RpcResult {
	for _, testedRPC := range rCtx.AlreadyTestedRPCs {
		if rpc == testedRPC.Method {
			return testedRPC
		}
	}
	return nil

}

// loadExistingState scans the blockchain and creates test transactions if needed
func (rCtx *RPCContext) loadExistingState() error {
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
