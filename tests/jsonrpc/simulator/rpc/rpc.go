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

	return &RpcContext{
		Conf:   conf,
		EthCli: ethCli,
		Acc: &types.Account{
			Address: crypto.PubkeyToAddress(ecdsaPrivKey.PublicKey),
			PrivKey: ecdsaPrivKey,
		},
	}, nil
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
