package namespaces

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	ethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/status-im/keycard-go/hexutils"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/contracts"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/utils"
)

const (
	NamespaceEth = "eth"

	// Eth namespace - client subcategory
	MethodNameEthChainID     types.RpcName = "eth_chainId"
	MethodNameEthSyncing     types.RpcName = "eth_syncing"
	MethodNameEthCoinbase    types.RpcName = "eth_coinbase"
	MethodNameEthAccounts    types.RpcName = "eth_accounts"
	MethodNameEthBlockNumber types.RpcName = "eth_blockNumber"
	MethodNameEthMining      types.RpcName = "eth_mining"
	MethodNameEthHashrate    types.RpcName = "eth_hashrate"

	// Eth namespace - fee_market subcategory
	MethodNameEthGasPrice             types.RpcName = "eth_gasPrice"
	MethodNameEthBlobBaseFee          types.RpcName = "eth_blobBaseFee"
	MethodNameEthMaxPriorityFeePerGas types.RpcName = "eth_maxPriorityFeePerGas"
	MethodNameEthFeeHistory           types.RpcName = "eth_feeHistory"

	// Eth namespace - state subcategory
	MethodNameEthGetBalance          types.RpcName = "eth_getBalance"
	MethodNameEthGetStorageAt        types.RpcName = "eth_getStorageAt"
	MethodNameEthGetTransactionCount types.RpcName = "eth_getTransactionCount"
	MethodNameEthGetCode             types.RpcName = "eth_getCode"
	MethodNameEthGetProof            types.RpcName = "eth_getProof"

	// Eth namespace - block subcategory
	MethodNameEthGetBlockByHash                   types.RpcName = "eth_getBlockByHash"
	MethodNameEthGetBlockByNumber                 types.RpcName = "eth_getBlockByNumber"
	MethodNameEthGetBlockTransactionCountByHash   types.RpcName = "eth_getBlockTransactionCountByHash"
	MethodNameEthGetBlockTransactionCountByNumber types.RpcName = "eth_getBlockTransactionCountByNumber"
	MethodNameEthGetUncleCountByBlockHash         types.RpcName = "eth_getUncleCountByBlockHash"
	MethodNameEthGetUncleCountByBlockNumber       types.RpcName = "eth_getUncleCountByBlockNumber"
	MethodNameEthGetUncleByBlockHashAndIndex      types.RpcName = "eth_getUncleByBlockHashAndIndex"
	MethodNameEthGetUncleByBlockNumberAndIndex    types.RpcName = "eth_getUncleByBlockNumberAndIndex"
	MethodNameEthGetBlockReceipts                 types.RpcName = "eth_getBlockReceipts"

	// Eth namespace - transaction subcategory
	MethodNameEthGetTransactionByHash                types.RpcName = "eth_getTransactionByHash"
	MethodNameEthGetTransactionByBlockHashAndIndex   types.RpcName = "eth_getTransactionByBlockHashAndIndex"
	MethodNameEthGetTransactionByBlockNumberAndIndex types.RpcName = "eth_getTransactionByBlockNumberAndIndex"
	MethodNameEthGetTransactionReceipt               types.RpcName = "eth_getTransactionReceipt"
	MethodNameEthGetTransactionCountByHash           types.RpcName = "eth_getTransactionCountByHash"
	MethodNameEthGetPendingTransactions              types.RpcName = "eth_getPendingTransactions"

	// Eth namespace - filter subcategory
	MethodNameEthNewFilter                   types.RpcName = "eth_newFilter"
	MethodNameEthNewBlockFilter              types.RpcName = "eth_newBlockFilter"
	MethodNameEthNewPendingTransactionFilter types.RpcName = "eth_newPendingTransactionFilter"
	MethodNameEthGetFilterChanges            types.RpcName = "eth_getFilterChanges"
	MethodNameEthGetFilterLogs               types.RpcName = "eth_getFilterLogs"
	MethodNameEthUninstallFilter             types.RpcName = "eth_uninstallFilter"
	MethodNameEthGetLogs                     types.RpcName = "eth_getLogs"

	// Eth namespace - execute subcategory
	MethodNameEthCall        types.RpcName = "eth_call"
	MethodNameEthEstimateGas types.RpcName = "eth_estimateGas"

	// Eth namespace - submit subcategory
	MethodNameEthSendTransaction    types.RpcName = "eth_sendTransaction"
	MethodNameEthSendRawTransaction types.RpcName = "eth_sendRawTransaction"

	// Eth namespace - sign subcategory (deprecated in many clients)
	MethodNameEthSign            types.RpcName = "eth_sign"
	MethodNameEthSignTransaction types.RpcName = "eth_signTransaction"

	// Eth namespace - other/deprecated methods
	MethodNameEthProtocolVersion  types.RpcName = "eth_protocolVersion"
	MethodNameEthGetCompilers     types.RpcName = "eth_getCompilers"
	MethodNameEthCompileSolidity  types.RpcName = "eth_compileSolidity"
	MethodNameEthGetWork          types.RpcName = "eth_getWork"
	MethodNameEthSubmitWork       types.RpcName = "eth_submitWork"
	MethodNameEthSubmitHashrate   types.RpcName = "eth_submitHashrate"
	MethodNameEthCreateAccessList types.RpcName = "eth_createAccessList"

	// Eth namespace - WebSocket-only subscription methods
	MethodNameEthSubscribe   types.RpcName = "eth_subscribe"
	MethodNameEthUnsubscribe types.RpcName = "eth_unsubscribe"
)

func EthCoinbase(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.EthCli.Client().Call(&result, "eth_coinbase")
	if err != nil {
		// Even if it fails, mark as deprecated
		return &types.RpcResult{
			Method:   MethodNameEthCoinbase,
			Status:   types.Legacy,
			Value:    fmt.Sprintf("API deprecated as of v1.14.0 - call failed: %s", err.Error()),
			ErrMsg:   "eth_coinbase deprecated as of Ethereum v1.14.0 - use eth_getBalance with miner address instead",
			Category: NamespaceEth,
		}, nil
	}

	// API works but is deprecated
	return &types.RpcResult{
		Method:   MethodNameEthCoinbase,
		Status:   types.Legacy,
		Value:    fmt.Sprintf("Deprecated API but functional: %s", result),
		ErrMsg:   "eth_coinbase deprecated as of Ethereum v1.14.0 - use eth_getBalance with miner address instead",
		Category: NamespaceEth,
	}, nil
}

func EthBlockNumber(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthBlockNumber); result != nil {
		return result, nil
	}

	blockNumber, err := rCtx.EthCli.BlockNumber(context.Background())
	if err != nil {
		return nil, err
	}

	// Block number 0 is valid for fresh chains
	status := types.Ok

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthBlockNumber)

	result := &types.RpcResult{
		Method:   MethodNameEthBlockNumber,
		Status:   status,
		Value:    blockNumber,
		Category: NamespaceEth,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGasPrice(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGasPrice); result != nil {
		return result, nil
	}

	gasPrice, err := rCtx.EthCli.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, err
	}

	// gasPrice should never be nil, but zero is valid in dev/test environments
	if gasPrice == nil {
		return nil, fmt.Errorf("gasPrice is nil")
	}

	status := types.Ok

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthGasPrice, "eth_gasPrice")

	result := &types.RpcResult{
		Method:   MethodNameEthGasPrice,
		Status:   status,
		Value:    gasPrice.String(),
		Category: NamespaceEth,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthMaxPriorityFeePerGas(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthMaxPriorityFeePerGas); result != nil {
		return result, nil
	}

	maxPriorityFeePerGas, err := rCtx.EthCli.SuggestGasTipCap(context.Background())
	if err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthMaxPriorityFeePerGas)

	result := &types.RpcResult{
		Method:   MethodNameEthMaxPriorityFeePerGas,
		Status:   types.Ok,
		Value:    maxPriorityFeePerGas.String(),
		Category: NamespaceEth,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthChainID(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthChainID); result != nil {
		return result, nil
	}

	chainID, err := rCtx.EthCli.ChainID(context.Background())
	if err != nil {
		return nil, err
	}

	// chainId should never be zero
	if chainID.Cmp(big.NewInt(0)) == 0 {
		return nil, fmt.Errorf("chainId is zero")
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthChainID)

	result := &types.RpcResult{
		Method:   MethodNameEthChainID,
		Status:   types.Ok,
		Value:    chainID.String(),
		Category: NamespaceEth,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetBalance(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetBalance); result != nil {
		return result, nil
	}

	balance, err := rCtx.EthCli.BalanceAt(context.Background(), rCtx.Acc.Address, nil)
	if err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthGetBalance, rCtx.Acc.Address.Hex(), "latest")

	result := &types.RpcResult{
		Method:   MethodNameEthGetBalance,
		Status:   types.Ok,
		Value:    balance.String(),
		Category: NamespaceEth,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetTransactionCount(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetTransactionCount); result != nil {
		return result, nil
	}

	nonce, err := rCtx.EthCli.PendingNonceAt(context.Background(), rCtx.Acc.Address)
	if err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthGetTransactionCount, rCtx.Acc.Address.Hex(), "latest")

	return &types.RpcResult{
		Method:   MethodNameEthGetTransactionCount,
		Status:   types.Ok,
		Value:    nonce,
		Category: NamespaceEth,
	}, nil
}

func EthGetBlockByHash(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetBlockByHash); result != nil {
		return result, nil
	}

	// Use real transaction receipt data to get a valid block hash
	if len(rCtx.ProcessedTransactions) == 0 {
		return nil, errors.New("no processed transactions available - run transaction generation first")
	}

	// Get a receipt from one of our processed transactions to get a real block hash
	receipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), rCtx.ProcessedTransactions[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt for transaction %s: %w", rCtx.ProcessedTransactions[0].Hex(), err)
	}

	// Use the block hash from the receipt to test getBlockByHash
	block, err := rCtx.EthCli.BlockByHash(context.Background(), receipt.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("block hash lookup failed for hash %s from receipt: %w", receipt.BlockHash.Hex(), err)
	}

	// Perform dual API comparison if enabled - use different block hashes for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetBlockByHash, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.GethProcessedTransactions) > 0 {
			// Get geth receipt to get geth block hash
			if gethReceipt, err := rCtx.GethCli.TransactionReceipt(context.Background(), rCtx.GethProcessedTransactions[0]); err == nil {
				return []interface{}{gethReceipt.BlockHash.Hex(), true}
			}
		}
		return []interface{}{receipt.BlockHash.Hex(), true}
	})

	result := &types.RpcResult{
		Method: MethodNameEthGetBlockByHash,
		Status: types.Ok,
		Value:  utils.MustBeautifyBlock(types.NewRPCBlock(block)),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetBlockByNumber(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetBlockByNumber); result != nil {
		return result, nil
	}

	blkNum, err := rCtx.EthCli.BlockNumber(context.Background())
	if err != nil {
		return nil, err
	}

	blk, err := rCtx.EthCli.BlockByNumber(context.Background(), new(big.Int).SetUint64(blkNum))
	if err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled - use "latest" for both clients to compare block structure
	rCtx.PerformComparison(MethodNameEthGetBlockByNumber, "latest", true)

	result := &types.RpcResult{
		Method: MethodNameEthGetBlockByNumber,
		Status: types.Ok,
		Value:  utils.MustBeautifyBlock(types.NewRPCBlock(blk)),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthSendRawTransactionTransferValue(rCtx *types.RPCContext) (*types.RpcResult, error) {
	// testedRPCs is a slice of RpcResult that will be appended to rCtx.AlreadyTestedRPCs
	// if the transaction is successfully sent
	var testedRPCs []*types.RpcResult
	var err error
	// Create a new transaction
	if rCtx.ChainID, err = rCtx.EthCli.ChainID(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthChainID,
		Status: types.Ok,
		Value:  rCtx.ChainID.String(),
	})

	nonce, err := rCtx.EthCli.PendingNonceAt(context.Background(), rCtx.Acc.Address)
	if err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthGetTransactionCount,
		Status: types.Ok,
		Value:  nonce,
	})

	if rCtx.MaxPriorityFeePerGas, err = rCtx.EthCli.SuggestGasTipCap(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthMaxPriorityFeePerGas,
		Status: types.Ok,
		Value:  rCtx.MaxPriorityFeePerGas.String(),
	})
	if rCtx.GasPrice, err = rCtx.EthCli.SuggestGasPrice(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthGasPrice,
		Status: types.Ok,
		Value:  rCtx.GasPrice.String(),
	})

	randomRecipient := utils.MustCreateRandomAccount().Address
	value := new(big.Int).SetUint64(1)
	balanceBeforeSend, err := rCtx.EthCli.BalanceAt(context.Background(), rCtx.Acc.Address, nil)
	if err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthGetBalance,
		Status: types.Ok,
		Value:  balanceBeforeSend.String(),
	})

	if balanceBeforeSend.Cmp(value) < 0 {
		return nil, errors.New("insufficient balanceBeforeSend")
	}

	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   rCtx.ChainID,
		Nonce:     nonce,
		GasTipCap: rCtx.MaxPriorityFeePerGas,
		GasFeeCap: new(big.Int).Add(rCtx.GasPrice, big.NewInt(1000000000)),
		Gas:       21000, // fixed gas limit for transfer
		To:        &randomRecipient,
		Value:     value,
	})

	// TODO: Make signer using types.MakeSigner with chain params
	signer := gethtypes.NewLondonSigner(rCtx.ChainID)
	signedTx, err := gethtypes.SignTx(tx, signer, rCtx.Acc.PrivKey)
	if err != nil {
		return nil, err
	}

	if err = rCtx.EthCli.SendTransaction(context.Background(), signedTx); err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthSendRawTransaction, signedTx)

	result := &types.RpcResult{
		Method: MethodNameEthSendRawTransaction,
		Status: types.Ok,
		Value:  signedTx.Hash().Hex(),
	}
	testedRPCs = append(testedRPCs, result)

	// wait for the transaction to be mined
	tout, _ := time.ParseDuration(rCtx.Conf.Timeout)
	if err = utils.WaitForTx(rCtx, signedTx.Hash(), tout); err != nil {
		return nil, err
	}

	balance, err := rCtx.EthCli.BalanceAt(context.Background(), rCtx.Acc.Address, nil)
	if err != nil {
		return nil, err
	}
	// check if the balance decreased by the value of the transaction (+ gas fee)
	if new(big.Int).Sub(balanceBeforeSend, balance).Cmp(value) < 0 {
		return nil, errors.New("balanceBeforeSend mismatch, maybe the transaction was not mined or implementation is incorrect")
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, testedRPCs...)

	return result, nil
}

func EthSendRawTransactionDeployContract(rCtx *types.RPCContext) (*types.RpcResult, error) {
	// testedRPCs is a slice of RpcResult that will be appended to rCtx.AlreadyTestedRPCs
	// if the transaction is successfully sent
	var testedRPCs []*types.RpcResult
	var err error
	// Create a new transaction
	if rCtx.ChainID, err = rCtx.EthCli.ChainID(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthChainID,
		Status: types.Ok,
		Value:  rCtx.ChainID.String(),
	})

	nonce, err := rCtx.EthCli.PendingNonceAt(context.Background(), rCtx.Acc.Address)
	if err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthGetTransactionCount,
		Status: types.Ok,
		Value:  nonce,
	})

	if rCtx.MaxPriorityFeePerGas, err = rCtx.EthCli.SuggestGasTipCap(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthMaxPriorityFeePerGas,
		Status: types.Ok,
		Value:  rCtx.MaxPriorityFeePerGas.String(),
	})
	if rCtx.GasPrice, err = rCtx.EthCli.SuggestGasPrice(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthGasPrice,
		Status: types.Ok,
		Value:  rCtx.GasPrice.String(),
	})

	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   rCtx.ChainID,
		Nonce:     nonce,
		GasTipCap: rCtx.MaxPriorityFeePerGas,
		GasFeeCap: new(big.Int).Add(rCtx.GasPrice, big.NewInt(1000000000)),
		Gas:       10000000,
		Data:      common.FromHex(string(contracts.ContractByteCode)),
	})

	// TODO: Make signer using types.MakeSigner with chain params
	signer := gethtypes.NewLondonSigner(rCtx.ChainID)
	signedTx, err := gethtypes.SignTx(tx, signer, rCtx.Acc.PrivKey)
	if err != nil {
		return nil, err
	}

	if err = rCtx.EthCli.SendTransaction(context.Background(), signedTx); err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthSendRawTransaction, signedTx)

	result := &types.RpcResult{
		Method: MethodNameEthSendRawTransaction,
		Status: types.Ok,
		Value:  signedTx.Hash().Hex(),
	}
	testedRPCs = append(testedRPCs, result)

	// wait for the transaction to be mined
	tout, _ := time.ParseDuration(rCtx.Conf.Timeout)
	if err = utils.WaitForTx(rCtx, signedTx.Hash(), tout); err != nil {
		return nil, err
	}

	if rCtx.ERC20Addr == (common.Address{}) {
		return nil, errors.New("contract address is empty, failed to deploy")
	}

	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, testedRPCs...)

	return result, nil
}

// EthSendRawTransaction unified test that combines all scenarios
func EthSendRawTransaction(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var allResults []*types.RpcResult
	var failedScenarios []string
	var passedScenarios []string

	// Test 1: Transfer value
	result1, err := EthSendRawTransactionTransferValue(rCtx)
	if err != nil || result1.Status != types.Ok {
		failedScenarios = append(failedScenarios, "Transfer value")
	} else {
		passedScenarios = append(passedScenarios, "Transfer value")
	}
	if result1 != nil {
		allResults = append(allResults, result1)
	}

	// Test 2: Deploy contract
	result2, err := EthSendRawTransactionDeployContract(rCtx)
	if err != nil || result2.Status != types.Ok {
		failedScenarios = append(failedScenarios, "Deploy contract")
	} else {
		passedScenarios = append(passedScenarios, "Deploy contract")
	}
	if result2 != nil {
		allResults = append(allResults, result2)
	}

	// Test 3: Transfer ERC20
	result3, err := EthSendRawTransactionTransferERC20(rCtx)
	if err != nil || result3.Status != types.Ok {
		failedScenarios = append(failedScenarios, "Transfer ERC20")
	} else {
		passedScenarios = append(passedScenarios, "Transfer ERC20")
	}
	if result3 != nil {
		allResults = append(allResults, result3)
	}

	// Determine overall result
	status := types.Ok
	var errMsg string
	if len(failedScenarios) > 0 {
		status = types.Error
		errMsg = fmt.Sprintf("Failed scenarios: %s. Passed scenarios: %s",
			strings.Join(failedScenarios, ", "),
			strings.Join(passedScenarios, ", "))
	}

	// Create summary result
	return &types.RpcResult{
		Method:      MethodNameEthSendRawTransaction,
		Status:      status,
		Value:       fmt.Sprintf("Completed %d scenarios: %s", len(allResults), strings.Join(passedScenarios, ", ")),
		ErrMsg:      errMsg,
		Description: fmt.Sprintf("Combined test: %d passed, %d failed", len(passedScenarios), len(failedScenarios)),
		Category:    NamespaceEth,
	}, nil
}

func EthSendRawTransactionTransferERC20(rCtx *types.RPCContext) (*types.RpcResult, error) {
	// testedRPCs is a slice of RpcResult that will be appended to rCtx.AlreadyTestedRPCs
	// if the transaction is successfully sent
	var testedRPCs []*types.RpcResult
	var err error
	// Create a new transaction
	if rCtx.ChainID, err = rCtx.EthCli.ChainID(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthChainID,
		Status: types.Ok,
		Value:  rCtx.ChainID.String(),
	})

	nonce, err := rCtx.EthCli.PendingNonceAt(context.Background(), rCtx.Acc.Address)
	if err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthGetTransactionCount,
		Status: types.Ok,
		Value:  nonce,
	})

	if rCtx.MaxPriorityFeePerGas, err = rCtx.EthCli.SuggestGasTipCap(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthMaxPriorityFeePerGas,
		Status: types.Ok,
		Value:  rCtx.MaxPriorityFeePerGas.String(),
	})
	if rCtx.GasPrice, err = rCtx.EthCli.SuggestGasPrice(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthGasPrice,
		Status: types.Ok,
		Value:  rCtx.GasPrice.String(),
	})

	randomRecipient := utils.MustCreateRandomAccount().Address
	data, err := rCtx.ERC20Abi.Pack("transfer", randomRecipient, new(big.Int).SetUint64(1))
	if err != nil {
		log.Fatalf("Failed to pack transaction data: %v", err)
	}

	// Erc20 transfer
	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   rCtx.ChainID,
		Nonce:     nonce,
		GasTipCap: rCtx.MaxPriorityFeePerGas,
		GasFeeCap: new(big.Int).Add(rCtx.GasPrice, big.NewInt(1000000000)),
		Gas:       10000000,
		To:        &rCtx.ERC20Addr,
		Data:      data,
	})

	// TODO: Make signer using types.MakeSigner with chain params
	signer := gethtypes.NewLondonSigner(rCtx.ChainID)
	signedTx, err := gethtypes.SignTx(tx, signer, rCtx.Acc.PrivKey)
	if err != nil {
		return nil, err
	}

	if err = rCtx.EthCli.SendTransaction(context.Background(), signedTx); err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthSendRawTransaction, signedTx)

	result := &types.RpcResult{
		Method: MethodNameEthSendRawTransaction,
		Status: types.Ok,
		Value:  signedTx.Hash().Hex(),
	}
	testedRPCs = append(testedRPCs, result)

	// wait for the transaction to be mined
	tout, _ := time.ParseDuration(rCtx.Conf.Timeout)
	if err = utils.WaitForTx(rCtx, signedTx.Hash(), tout); err != nil {
		return nil, err
	}

	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, testedRPCs...)

	return result, nil
}

func EthGetBlockReceipts(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetBlockReceipts); result != nil {
		return result, nil
	}

	if len(rCtx.BlockNumsIncludingTx) == 0 {
		return nil, errors.New("no blocks with transactions")
	}

	// TODO: Random pick
	// pick a block with transactions
	blkNum := rCtx.BlockNumsIncludingTx[0]
	if blkNum > uint64(math.MaxInt64) {
		return nil, fmt.Errorf("block number %d exceeds int64 max value", blkNum)
	}
	rpcBlockNum := ethrpc.BlockNumber(int64(blkNum))
	receipts, err := rCtx.EthCli.BlockReceipts(context.Background(), ethrpc.BlockNumberOrHash{BlockNumber: &rpcBlockNum})
	if err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled - use different block numbers for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetBlockReceipts, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.GethBlockNumsIncludingTx) > 0 {
			return []interface{}{fmt.Sprintf("0x%x", rCtx.GethBlockNumsIncludingTx[0])}
		} else if len(rCtx.BlockNumsIncludingTx) > 0 {
			return []interface{}{fmt.Sprintf("0x%x", rCtx.BlockNumsIncludingTx[0])}
		}
		return []interface{}{"latest"}
	})

	result := &types.RpcResult{
		Method: MethodNameEthGetBlockReceipts,
		Status: types.Ok,
		Value:  utils.MustBeautifyReceipts(receipts),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetTransactionByHash(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetTransactionByHash); result != nil {
		return result, nil
	}

	if len(rCtx.ProcessedTransactions) == 0 {
		return nil, errors.New("no transactions")
	}

	// TODO: Random pick
	txHash := rCtx.ProcessedTransactions[0]

	tx, _, err := rCtx.EthCli.TransactionByHash(context.Background(), txHash)
	if err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled - use different transaction hashes for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetTransactionByHash, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.GethProcessedTransactions) > 0 {
			return []interface{}{rCtx.GethProcessedTransactions[0].Hex()}
		}
		return []interface{}{txHash.Hex()}
	})

	result := &types.RpcResult{
		Method: MethodNameEthGetTransactionByHash,
		Status: types.Ok,
		Value:  utils.MustBeautifyTransaction(tx),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetTransactionByBlockHashAndIndex(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetTransactionByBlockHashAndIndex); result != nil {
		return result, nil
	}

	if len(rCtx.ProcessedTransactions) == 0 {
		return nil, errors.New("no processed transactions available - run transaction generation first")
	}

	// Get a receipt from one of our processed transactions to get a real block hash
	receipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), rCtx.ProcessedTransactions[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt for transaction %s: %w", rCtx.ProcessedTransactions[0].Hex(), err)
	}

	// Use the transaction index from the receipt and block hash from the receipt
	tx, err := rCtx.EthCli.TransactionInBlock(context.Background(), receipt.BlockHash, receipt.TransactionIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction at index %d in block %s: %w", receipt.TransactionIndex, receipt.BlockHash.Hex(), err)
	}

	// Perform dual API comparison if enabled - use different transaction data for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetTransactionByBlockHashAndIndex, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.GethProcessedTransactions) > 0 {
			// Get geth transaction receipt to get block hash and index
			if receipt, err := rCtx.GethCli.TransactionReceipt(context.Background(), rCtx.GethProcessedTransactions[0]); err == nil {
				return []interface{}{receipt.BlockHash.Hex(), fmt.Sprintf("0x%x", receipt.TransactionIndex)}
			}
		} else if len(rCtx.ProcessedTransactions) > 0 {
			// Get evmd transaction receipt to get block hash and index
			if receipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), rCtx.ProcessedTransactions[0]); err == nil {
				return []interface{}{receipt.BlockHash.Hex(), fmt.Sprintf("0x%x", receipt.TransactionIndex)}
			}
		}
		return []interface{}{"0x0", "0x0"} // Fallback that will likely return null
	})

	result := &types.RpcResult{
		Method: MethodNameEthGetTransactionByBlockHashAndIndex,
		Status: types.Ok,
		Value:  utils.MustBeautifyTransaction(tx),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetTransactionByBlockNumberAndIndex(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetTransactionByBlockNumberAndIndex); result != nil {
		return result, nil
	}

	if len(rCtx.BlockNumsIncludingTx) == 0 {
		return nil, errors.New("no blocks with transactions")
	}

	// TODO: Random pick
	blkNum := rCtx.BlockNumsIncludingTx[0]
	var tx gethtypes.Transaction
	if err := rCtx.EthCli.Client().CallContext(context.Background(), &tx, string(MethodNameEthGetTransactionByBlockNumberAndIndex), blkNum, "0x0"); err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparisonWithProvider(MethodNameEthGetTransactionByBlockNumberAndIndex, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.GethBlockNumsIncludingTx) > 0 {
			return []interface{}{fmt.Sprintf("0x%x", rCtx.GethBlockNumsIncludingTx[0]), "0x0"}
		}
		if len(rCtx.BlockNumsIncludingTx) > 0 {
			return []interface{}{fmt.Sprintf("0x%x", rCtx.BlockNumsIncludingTx[0]), "0x0"}
		}
		return []interface{}{"latest", "0x0"}
	})

	result := &types.RpcResult{
		Method: MethodNameEthGetTransactionByBlockNumberAndIndex,
		Status: types.Ok,
		Value:  utils.MustBeautifyTransaction(&tx),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetBlockTransactionCountByNumber(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetBlockTransactionCountByNumber); result != nil {
		return result, nil
	}

	// Get current block number
	blockNumber, err := rCtx.EthCli.BlockNumber(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get block number: %w", err)
	}

	// Use a recent block that has transactions
	targetBlockNum := blockNumber
	if blockNumber > 1 {
		targetBlockNum = blockNumber - 1
	}

	// Get the block first to get its hash, then get transaction count
	if targetBlockNum > uint64(math.MaxInt64) {
		return nil, fmt.Errorf("targetBlockNum %d exceeds int64 max value", targetBlockNum)
	}
	block, err := rCtx.EthCli.BlockByNumber(context.Background(), big.NewInt(int64(targetBlockNum)))
	if err != nil {
		return nil, fmt.Errorf("failed to get block %d: %w", targetBlockNum, err)
	}

	// Get transaction count for the block
	count := uint64(len(block.Transactions()))

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthGetBlockTransactionCountByNumber, fmt.Sprintf("0x%x", targetBlockNum))

	result := &types.RpcResult{
		Method:   MethodNameEthGetBlockTransactionCountByNumber,
		Status:   types.Ok,
		Value:    count,
		Category: NamespaceEth,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

// Uncle methods - these should always return 0 or nil in Cosmos EVM (no uncles in PoS)
func EthGetUncleCountByBlockHash(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetUncleCountByBlockHash); result != nil {
		return result, nil
	}

	// Get a block hash - try from processed transactions first, fallback to latest block
	var blockHash common.Hash
	if len(rCtx.ProcessedTransactions) > 0 {
		receipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), rCtx.ProcessedTransactions[0])
		if err != nil {
			return nil, fmt.Errorf("failed to get receipt: %w", err)
		}
		blockHash = receipt.BlockHash
	} else {
		// Fallback to latest block
		block, err := rCtx.EthCli.BlockByNumber(context.Background(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest block: %w", err)
		}
		blockHash = block.Hash()
	}

	var uncleCount string
	err := rCtx.EthCli.Client().CallContext(context.Background(), &uncleCount, string(MethodNameEthGetUncleCountByBlockHash), blockHash)
	if err != nil {
		return nil, fmt.Errorf("eth_getUncleCountByBlockHash call failed: %w", err)
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparisonWithProvider(MethodNameEthGetUncleCountByBlockHash, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.GethProcessedTransactions) > 0 {
			if gethReceipt, err := rCtx.GethCli.TransactionReceipt(context.Background(), rCtx.GethProcessedTransactions[0]); err == nil {
				return []interface{}{gethReceipt.BlockHash.Hex()}
			}
		}
		return []interface{}{blockHash.Hex()}
	})

	// Should always be 0 in Cosmos EVM
	result := &types.RpcResult{
		Method:   MethodNameEthGetUncleCountByBlockHash,
		Status:   types.Ok,
		Value:    uncleCount,
		Category: NamespaceEth,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetUncleCountByBlockNumber(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetUncleCountByBlockNumber); result != nil {
		return result, nil
	}

	var uncleCount string
	err := rCtx.EthCli.Client().CallContext(context.Background(), &uncleCount, string(MethodNameEthGetUncleCountByBlockNumber), "latest")
	if err != nil {
		return nil, fmt.Errorf("eth_getUncleCountByBlockNumber call failed: %w", err)
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthGetUncleCountByBlockNumber, "latest")

	// Should always be 0 in Cosmos EVM
	result := &types.RpcResult{
		Method:   MethodNameEthGetUncleCountByBlockNumber,
		Status:   types.Ok,
		Value:    uncleCount,
		Category: NamespaceEth,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetUncleByBlockHashAndIndex(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetUncleByBlockHashAndIndex); result != nil {
		return result, nil
	}

	// Get a block hash - try from processed transactions first, fallback to latest block
	var blockHash common.Hash
	if len(rCtx.ProcessedTransactions) > 0 {
		receipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), rCtx.ProcessedTransactions[0])
		if err != nil {
			return nil, fmt.Errorf("failed to get receipt: %w", err)
		}
		blockHash = receipt.BlockHash
	} else {
		// Fallback to latest block
		block, err := rCtx.EthCli.BlockByNumber(context.Background(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest block: %w", err)
		}
		blockHash = block.Hash()
	}

	var uncle interface{}
	err := rCtx.EthCli.Client().CallContext(context.Background(), &uncle, string(MethodNameEthGetUncleByBlockHashAndIndex), blockHash, "0x0")
	if err != nil {
		return nil, fmt.Errorf("eth_getUncleByBlockHashAndIndex call failed: %w", err)
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthGetUncleByBlockHashAndIndex, blockHash, "0x0")

	// Should always be nil in Cosmos EVM
	result := &types.RpcResult{
		Method:   MethodNameEthGetUncleByBlockHashAndIndex,
		Status:   types.Ok,
		Value:    uncle,
		Category: NamespaceEth,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetUncleByBlockNumberAndIndex(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetUncleByBlockNumberAndIndex); result != nil {
		return result, nil
	}

	var uncle interface{}
	// Get current block number and format as hex
	blockNumber, err := rCtx.EthCli.BlockNumber(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get block number: %w", err)
	}
	blockNumberHex := fmt.Sprintf("0x%x", blockNumber)

	err = rCtx.EthCli.Client().CallContext(context.Background(), &uncle, string(MethodNameEthGetUncleByBlockNumberAndIndex), blockNumberHex, "0x0")
	if err != nil {
		return nil, fmt.Errorf("eth_getUncleByBlockNumberAndIndex call failed: %w", err)
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthGetUncleByBlockNumberAndIndex, blockNumberHex, "0x0")

	// Should always be nil in Cosmos EVM
	result := &types.RpcResult{
		Method:   MethodNameEthGetUncleByBlockNumberAndIndex,
		Status:   types.Ok,
		Value:    uncle,
		Category: NamespaceEth,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetTransactionCountByHash(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetTransactionCountByHash); result != nil {
		return result, nil
	}

	if len(rCtx.BlockNumsIncludingTx) == 0 {
		return nil, errors.New("no transactions")
	}

	// get block
	blkNum := rCtx.BlockNumsIncludingTx[0]
	blk, err := rCtx.EthCli.BlockByNumber(context.Background(), new(big.Int).SetUint64(blkNum))
	if err != nil {
		return nil, err
	}

	var count uint64
	if err = rCtx.EthCli.Client().CallContext(context.Background(), &count, string(MethodNameEthGetTransactionCountByHash), blk.Hash()); err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthGetTransactionCountByHash, blk.Hash())

	result := &types.RpcResult{
		Method: MethodNameEthGetTransactionCountByHash,
		Status: types.Ok,
		Value:  count,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetTransactionReceipt(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetTransactionReceipt); result != nil {
		return result, nil
	}

	if len(rCtx.ProcessedTransactions) == 0 {
		return nil, errors.New("no transactions")
	}

	txHash := rCtx.ProcessedTransactions[0]

	receipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), txHash)
	if err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled - use different transaction hashes for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetTransactionReceipt, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.GethProcessedTransactions) > 0 {
			return []interface{}{rCtx.GethProcessedTransactions[0].Hex()}
		}
		return []interface{}{txHash.Hex()}
	})

	result := &types.RpcResult{
		Method: MethodNameEthGetTransactionReceipt,
		Status: types.Ok,
		Value:  utils.MustBeautifyReceipt(receipt),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetBlockTransactionCountByHash(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetBlockTransactionCountByHash); result != nil {
		return result, nil
	}

	if len(rCtx.ProcessedTransactions) == 0 {
		return nil, errors.New("no processed transactions available - run transaction generation first")
	}

	// Get a receipt from one of our processed transactions to get a real block hash
	receipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), rCtx.ProcessedTransactions[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt for transaction %s: %w", rCtx.ProcessedTransactions[0].Hex(), err)
	}

	count, err := rCtx.EthCli.TransactionCount(context.Background(), receipt.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction count for block hash %s: %w", receipt.BlockHash.Hex(), err)
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparisonWithProvider(MethodNameEthGetBlockTransactionCountByHash, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.GethProcessedTransactions) > 0 {
			if gethReceipt, err := rCtx.GethCli.TransactionReceipt(context.Background(), rCtx.GethProcessedTransactions[0]); err == nil {
				return []interface{}{gethReceipt.BlockHash.Hex()}
			}
		}
		return []interface{}{receipt.BlockHash.Hex()}
	})

	result := &types.RpcResult{
		Method: MethodNameEthGetBlockTransactionCountByHash,
		Status: types.Ok,
		Value:  count,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetCode(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetCode); result != nil {
		return result, nil
	}

	if rCtx.ERC20Addr == (common.Address{}) {
		return nil, errors.New("no contract address, must be deployed first")
	}

	code, err := rCtx.EthCli.CodeAt(context.Background(), rCtx.ERC20Addr, nil)
	if err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled - use different contract addresses for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetCode, func(isGeth bool) []interface{} {
		if isGeth && rCtx.GethERC20Addr != (common.Address{}) {
			return []interface{}{rCtx.GethERC20Addr.Hex(), "latest"}
		}
		return []interface{}{rCtx.ERC20Addr.Hex(), "latest"}
	})

	result := &types.RpcResult{
		Method: MethodNameEthGetCode,
		Status: types.Ok,
		Value:  hexutils.BytesToHex(code),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetStorageAt(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetStorageAt); result != nil {
		return result, nil
	}

	if rCtx.ERC20Addr == (common.Address{}) {
		return nil, errors.New("no contract address, must be deployed first")
	}

	key := utils.MustCalculateSlotKey(rCtx.Acc.Address, 4)

	// Perform dual API comparison if enabled - use different contract addresses for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetStorageAt, func(isGeth bool) []interface{} {
		if isGeth && rCtx.GethERC20Addr != (common.Address{}) {
			return []interface{}{rCtx.GethERC20Addr.Hex(), fmt.Sprintf("0x%x", key), "latest"}
		}
		return []interface{}{rCtx.ERC20Addr.Hex(), fmt.Sprintf("0x%x", key), "latest"}
	})

	storage, err := rCtx.EthCli.StorageAt(context.Background(), rCtx.ERC20Addr, key, nil)
	if err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method:   MethodNameEthGetStorageAt,
		Status:   types.Ok,
		Value:    hexutils.BytesToHex(storage),
		Category: NamespaceEth,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthNewFilter(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthNewFilter); result != nil {
		return result, nil
	}

	if len(rCtx.BlockNumsIncludingTx) == 0 {
		return nil, errors.New("no blocks with transactions")
	}

	fErc20Transfer := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(rCtx.BlockNumsIncludingTx[0] - 1),
		Addresses: []common.Address{rCtx.ERC20Addr},
		Topics: [][]common.Hash{
			{rCtx.ERC20Abi.Events["Transfer"].ID}, // Filter for Transfer event
		},
	}
	args, err := utils.ToFilterArg(fErc20Transfer)
	if err != nil {
		return nil, err
	}
	var filterID string
	if err = rCtx.EthCli.Client().CallContext(context.Background(), &filterID, string(MethodNameEthNewFilter), args); err != nil {
		return nil, err
	}

	if len(rCtx.GethBlockNumsIncludingTx) == 0 {
		return nil, errors.New("no blocks with transactions")
	}

	fErc20TransferGeth := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(rCtx.GethBlockNumsIncludingTx[0] - 1),
		Addresses: []common.Address{rCtx.GethERC20Addr},
		Topics: [][]common.Hash{
			{rCtx.ERC20Abi.Events["Transfer"].ID}, // Filter for Transfer event
		},
	}
	argsGeth, err := utils.ToFilterArg(fErc20TransferGeth)
	if err != nil {
		return nil, err
	}
	var filterIDGeth string
	if err = rCtx.GethCli.Client().CallContext(context.Background(), &filterIDGeth, string(MethodNameEthNewFilter), argsGeth); err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled - use different block hashes for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetBlockByHash, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.GethProcessedTransactions) > 0 {
			return []interface{}{argsGeth}
		}
		return []interface{}{args}
	})

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthNewFilter, args)

	result := &types.RpcResult{
		Method: MethodNameEthNewFilter,
		Status: types.Ok,
		Value:  filterID,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	rCtx.FilterId = filterID
	rCtx.FilterQuery = fErc20Transfer
	rCtx.GethFilterId = filterIDGeth
	rCtx.GethFilterQuery = fErc20TransferGeth

	return result, nil
}

func EthGetFilterLogs(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetFilterLogs); result != nil {
		return result, nil
	}

	if rCtx.FilterId == "" {
		return nil, errors.New("no filter id, must create a filter first")
	}

	if _, err := EthSendRawTransactionTransferERC20(rCtx); err != nil {
		return nil, errors.New("transfer ERC20 must be succeeded before checking filter logs")
	}

	var logs []gethtypes.Log
	if err := rCtx.EthCli.Client().CallContext(context.Background(), &logs, string(MethodNameEthGetFilterLogs), rCtx.FilterId); err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparisonWithProvider(MethodNameEthGetFilterLogs, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.GethProcessedTransactions) > 0 {
			return []interface{}{rCtx.FilterId}
		}
		return []interface{}{rCtx.GethFilterId}
	})

	result := &types.RpcResult{
		Method: MethodNameEthGetFilterLogs,
		Status: types.Ok,
		Value:  utils.MustBeautifyLogs(logs),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthNewBlockFilter(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthNewBlockFilter); result != nil {
		return result, nil
	}

	var filterID string
	if err := rCtx.EthCli.Client().CallContext(context.Background(), &filterID, string(MethodNameEthNewBlockFilter)); err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthNewBlockFilter)

	result := &types.RpcResult{
		Method: MethodNameEthNewBlockFilter,
		Status: types.Ok,
		Value:  filterID,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	rCtx.BlockFilterId = filterID

	return result, nil
}

func EthGetFilterChanges(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetFilterChanges); result != nil {
		return result, nil
	}

	if rCtx.BlockFilterId == "" {
		return nil, errors.New("no block filter id, must create a block filter first")
	}

	// TODO: Make it configurable
	time.Sleep(3 * time.Second) // wait for a new block to be mined

	var changes []interface{}
	if err := rCtx.EthCli.Client().CallContext(context.Background(), &changes, string(MethodNameEthGetFilterChanges), rCtx.BlockFilterId); err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled - use different block hashes for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetFilterChanges, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.GethProcessedTransactions) > 0 {
			return []interface{}{rCtx.BlockFilterId}
		}
		return []interface{}{rCtx.GethBlockFilterId}
	})

	status := types.Ok
	// Empty results are valid - no warnings needed

	result := &types.RpcResult{
		Method:   MethodNameEthGetFilterChanges,
		Status:   status,
		Value:    changes,
		Category: NamespaceEth,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthUninstallFilter(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthUninstallFilter); result != nil {
		return result, nil
	}

	if rCtx.FilterId == "" {
		return nil, errors.New("no filter id, must create a filter first")
	}

	var res bool
	if err := rCtx.EthCli.Client().CallContext(context.Background(), &res, string(MethodNameEthUninstallFilter), rCtx.FilterId); err != nil {
		return nil, err
	}
	if !res {
		return nil, errors.New("uninstall filter failed")
	}

	if err := rCtx.EthCli.Client().CallContext(context.Background(), &res, string(MethodNameEthUninstallFilter), rCtx.FilterId); err != nil {
		return nil, err
	}
	if res {
		return nil, errors.New("uninstall filter should be failed because it was already uninstalled")
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthUninstallFilter, rCtx.FilterId)

	result := &types.RpcResult{
		Method: MethodNameEthUninstallFilter,
		Status: types.Ok,
		Value:  rCtx.FilterId,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetLogs(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetLogs); result != nil {
		return result, nil
	}

	if _, err := EthNewFilter(rCtx); err != nil {
		return nil, errors.New("failed to create a filter")
	}

	if _, err := EthSendRawTransactionTransferERC20(rCtx); err != nil {
		return nil, errors.New("transfer ERC20 must be succeeded before checking filter logs")
	}

	// set from block because of limit
	logs, err := rCtx.EthCli.FilterLogs(context.Background(), rCtx.FilterQuery)
	if err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled
	args, _ := utils.ToFilterArg(rCtx.FilterQuery)
	rCtx.PerformComparison(MethodNameEthGetLogs, args)

	status := types.Ok
	// Empty results are valid - no warnings needed

	result := &types.RpcResult{
		Method:   MethodNameEthGetLogs,
		Status:   status,
		Value:    utils.MustBeautifyLogs(logs),
		Category: NamespaceEth,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

// Additional Eth method handlers
func EthProtocolVersion(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.EthCli.Client().Call(&result, "eth_protocolVersion")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthProtocolVersion,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceEth,
		}, nil
	}
	rpcResult := &types.RpcResult{
		Method:   MethodNameEthProtocolVersion,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceEth,
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthProtocolVersion)

	return rpcResult, nil
}

func EthSyncing(rCtx *types.RPCContext) (*types.RpcResult, error) {
	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthSyncing)

	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, "eth_syncing")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthSyncing,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceEth,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameEthSyncing,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceEth,
	}, nil
}

func EthAccounts(rCtx *types.RPCContext) (*types.RpcResult, error) {
	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthAccounts)

	var result []string
	err := rCtx.EthCli.Client().Call(&result, "eth_accounts")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthAccounts,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceEth,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameEthAccounts,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceEth,
	}, nil
}

// Mining method handlers
func EthMining(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result bool
	err := rCtx.EthCli.Client().Call(&result, "eth_mining")
	if err != nil {
		// Even if it fails, mark as deprecated
		return &types.RpcResult{
			Method:   MethodNameEthMining,
			Status:   types.Legacy,
			Value:    fmt.Sprintf("API deprecated as of v1.14.0 - call failed: %s", err.Error()),
			ErrMsg:   "eth_mining deprecated as of Ethereum v1.14.0 - PoW mining no longer supported in PoS",
			Category: NamespaceEth,
		}, nil
	}

	// API works but is deprecated
	return &types.RpcResult{
		Method:   MethodNameEthMining,
		Status:   types.Legacy,
		Value:    fmt.Sprintf("Deprecated API but functional: %t", result),
		ErrMsg:   "eth_mining deprecated as of Ethereum v1.14.0 - PoW mining no longer supported in PoS",
		Category: NamespaceEth,
	}, nil
}

func EthHashrate(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.EthCli.Client().Call(&result, "eth_hashrate")
	if err != nil {
		// Even if it fails, mark as deprecated
		return &types.RpcResult{
			Method:   MethodNameEthHashrate,
			Status:   types.Legacy,
			Value:    fmt.Sprintf("API deprecated as of v1.14.0 - call failed: %s", err.Error()),
			ErrMsg:   "eth_hashrate deprecated as of Ethereum v1.14.0 - PoW mining no longer supported in PoS",
			Category: NamespaceEth,
		}, nil
	}

	// API works but is deprecated
	return &types.RpcResult{
		Method:   MethodNameEthHashrate,
		Status:   types.Legacy,
		Value:    fmt.Sprintf("Deprecated API but functional: %s", result),
		ErrMsg:   "eth_hashrate deprecated as of Ethereum v1.14.0 - PoW mining no longer supported in PoS",
		Category: NamespaceEth,
	}, nil
}

func EthCall(rCtx *types.RPCContext) (*types.RpcResult, error) {
	// Simple eth_call test
	callMsg := ethereum.CallMsg{
		To:   &rCtx.Acc.Address,
		Data: []byte{},
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthCall, map[string]interface{}{
		"to":   rCtx.Acc.Address.Hex(),
		"data": "0x",
	}, "latest")

	result, err := rCtx.EthCli.CallContract(context.Background(), callMsg, nil)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthCall,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceEth,
		}, nil
	}

	return &types.RpcResult{
		Method:   MethodNameEthCall,
		Status:   types.Ok,
		Value:    "0x" + hex.EncodeToString(result),
		Category: NamespaceEth,
	}, nil
}

func EthEstimateGas(rCtx *types.RPCContext) (*types.RpcResult, error) {
	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthEstimateGas, map[string]interface{}{
		"from":  rCtx.Acc.Address.Hex(),
		"to":    rCtx.Acc.Address.Hex(),
		"value": "0x0",
	})

	// Simple gas estimation test
	callMsg := ethereum.CallMsg{
		From:  rCtx.Acc.Address,
		To:    &rCtx.Acc.Address,
		Value: big.NewInt(0),
	}

	gasLimit, err := rCtx.EthCli.EstimateGas(context.Background(), callMsg)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthEstimateGas,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceEth,
		}, nil
	}

	return &types.RpcResult{
		Method:   MethodNameEthEstimateGas,
		Status:   types.Ok,
		Value:    fmt.Sprintf("0x%x", gasLimit),
		Category: NamespaceEth,
	}, nil
}

func EthFeeHistory(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, string(MethodNameEthFeeHistory), "0x2", "latest", []float64{25.0, 50.0, 75.0})

	if err != nil {
		if err.Error() == "the method "+string(MethodNameEthFeeHistory)+" does not exist/is not available" ||
			err.Error() == types.ErrorMethodNotFound {
			return &types.RpcResult{
				Method:   MethodNameEthFeeHistory,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: NamespaceEth,
			}, nil
		}
		return &types.RpcResult{
			Method:   MethodNameEthFeeHistory,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceEth,
		}, nil
	}

	rpcResult := &types.RpcResult{
		Method:   MethodNameEthFeeHistory,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceEth,
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthFeeHistory, "0x2", "latest", []float64{25.0, 50.0, 75.0})

	return rpcResult, nil
}

func EthBlobBaseFee(rCtx *types.RPCContext) (*types.RpcResult, error) {
	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthBlobBaseFee)

	return utils.CallEthClient(rCtx, MethodNameEthBlobBaseFee, NamespaceEth)
}

func EthGetProof(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, string(MethodNameEthGetProof), rCtx.Acc.Address.Hex(), []string{}, "latest")

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthGetProof, rCtx.Acc.Address.Hex(), []string{}, "latest")

	if err != nil {
		if err.Error() == "the method "+string(MethodNameEthGetProof)+" does not exist/is not available" ||
			err.Error() == "Method not found" {
			return &types.RpcResult{
				Method:   MethodNameEthGetProof,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: NamespaceEth,
			}, nil
		}
		return &types.RpcResult{
			Method:   MethodNameEthGetProof,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceEth,
		}, nil
	}

	return &types.RpcResult{
		Method:   MethodNameEthGetProof,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceEth,
	}, nil
}

// EthSendTransaction sends a transaction using eth_sendTransaction
// This requires the account to be unlocked or managed by the node
func EthSendTransaction(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthSendTransaction); result != nil {
		return result, nil
	}

	// Create a simple transaction object for testing
	tx := map[string]interface{}{
		"from":  rCtx.Acc.Address.Hex(),
		"to":    "0x0100000000000000000000000000000000000000", // Bank precompile
		"value": "0x1",                                        // 1 wei
		"gas":   "0x5208",                                     // 21000 gas
	}

	var txHash string
	err := rCtx.EthCli.Client().Call(&txHash, string(MethodNameEthSendTransaction), tx)
	if err != nil {
		// Key not found errors should now be treated as failures since we have keys in keyring
		return &types.RpcResult{
			Method:   MethodNameEthSendTransaction,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Transaction signing failed - keys should be available in keyring: %s", err.Error()),
			Category: NamespaceEth,
		}, nil
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthSendTransaction, tx)

	result := &types.RpcResult{
		Method:   MethodNameEthSendTransaction,
		Status:   types.Ok,
		Value:    txHash,
		Category: NamespaceEth,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

// EthSign signs data using eth_sign
// This requires the account to be unlocked or managed by the node
func EthSign(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthSign); result != nil {
		return result, nil
	}

	// Test data to sign (32-byte hash)
	testData := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	var signature string
	err := rCtx.EthCli.Client().Call(&signature, string(MethodNameEthSign), rCtx.Acc.Address.Hex(), testData)
	if err != nil {
		// Key not found errors should now be treated as failures since we have keys in keyring
		// eth_sign disabled is still acceptable as some nodes disable it for security
		if strings.Contains(err.Error(), "eth_sign is disabled") {
			return &types.RpcResult{
				Method:   MethodNameEthSign,
				Status:   types.Ok, // API is disabled for security reasons - this is acceptable
				Value:    fmt.Sprintf("API disabled for security: %s", err.Error()),
				Category: NamespaceEth,
			}, nil
		}
		// All other errors (including key not found) should be treated as failures
		return &types.RpcResult{
			Method:   MethodNameEthSign,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Signing failed - keys should be available in keyring: %s", err.Error()),
			Category: NamespaceEth,
		}, nil
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthSign, rCtx.Acc.Address.Hex(), testData)

	result := &types.RpcResult{
		Method:   MethodNameEthSign,
		Status:   types.Ok,
		Value:    signature,
		Category: NamespaceEth,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

func EthCreateAccessList(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	callData := map[string]interface{}{
		"to":   rCtx.Acc.Address.Hex(),
		"data": "0x",
	}
	err := rCtx.EthCli.Client().Call(&result, string(MethodNameEthCreateAccessList), callData, "latest")

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthCreateAccessList, callData, "latest")

	if err != nil {
		if err.Error() == "the method "+string(MethodNameEthCreateAccessList)+" does not exist/is not available" ||
			err.Error() == "Method not found" {
			return &types.RpcResult{
				Method:   MethodNameEthCreateAccessList,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: NamespaceEth,
			}, nil
		}
		return &types.RpcResult{
			Method:   MethodNameEthCreateAccessList,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceEth,
		}, nil
	}

	return &types.RpcResult{
		Method:   MethodNameEthCreateAccessList,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceEth,
	}, nil
}
