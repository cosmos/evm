package namespaces

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	ethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/websocket"
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
	MethodNameEthGetHeaderByHash                  types.RpcName = "eth_getHeaderByHash"
	MethodNameEthGetHeaderByNumber                types.RpcName = "eth_getHeaderByNumber"

	// Eth namespace - transaction subcategory
	MethodNameEthGetTransactionByHash                types.RpcName = "eth_getTransactionByHash"
	MethodNameEthGetTransactionByBlockHashAndIndex   types.RpcName = "eth_getTransactionByBlockHashAndIndex"
	MethodNameEthGetTransactionByBlockNumberAndIndex types.RpcName = "eth_getTransactionByBlockNumberAndIndex"
	MethodNameEthGetTransactionReceipt               types.RpcName = "eth_getTransactionReceipt"
	MethodNameEthGetTransactionCountByHash           types.RpcName = "eth_getTransactionCountByHash"
	MethodNameEthGetPendingTransactions              types.RpcName = "eth_getPendingTransactions"
	MethodNameEthPendingTransactions                 types.RpcName = "eth_pendingTransactions"

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
	MethodNameEthSimulateV1  types.RpcName = "eth_simulateV1"

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
	err := rCtx.Evmd.RPCClient().Call(&result, "eth_coinbase")
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

	blockNumber, err := rCtx.Evmd.BlockNumber(context.Background())
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

	return result, nil
}

func EthGasPrice(rCtx *types.RPCContext) (*types.RpcResult, error) {

	gasPrice, err := rCtx.Evmd.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, err
	}

	// gasPrice should never be nil, but zero is valid in dev/test environments
	if gasPrice == nil {
		return nil, fmt.Errorf("gasPrice is nil")
	}

	status := types.Ok

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthGasPrice)

	result := &types.RpcResult{
		Method:   MethodNameEthGasPrice,
		Status:   status,
		Value:    gasPrice.String(),
		Category: NamespaceEth,
	}

	return result, nil
}

func EthMaxPriorityFeePerGas(rCtx *types.RPCContext) (*types.RpcResult, error) {

	maxPriorityFeePerGas, err := rCtx.Evmd.SuggestGasTipCap(context.Background())
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

	return result, nil
}

func EthChainID(rCtx *types.RPCContext) (*types.RpcResult, error) {

	chainID, err := rCtx.Evmd.ChainID(context.Background())
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

	return result, nil
}

func EthGetBalance(rCtx *types.RPCContext) (*types.RpcResult, error) {

	balance, err := rCtx.Evmd.BalanceAt(context.Background(), rCtx.Evmd.Acc.Address, nil)
	if err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthGetBalance, rCtx.Evmd.Acc.Address.Hex(), "latest")

	result := &types.RpcResult{
		Method:   MethodNameEthGetBalance,
		Status:   types.Ok,
		Value:    balance.String(),
		Category: NamespaceEth,
	}

	return result, nil
}

func EthGetTransactionCount(rCtx *types.RPCContext) (*types.RpcResult, error) {

	nonce, err := rCtx.Evmd.PendingNonceAt(context.Background(), rCtx.Evmd.Acc.Address)
	if err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthGetTransactionCount, rCtx.Evmd.Acc.Address.Hex(), "latest")

	return &types.RpcResult{
		Method:   MethodNameEthGetTransactionCount,
		Status:   types.Ok,
		Value:    nonce,
		Category: NamespaceEth,
	}, nil
}

func EthGetBlockByHash(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Get a receipt from one of our processed transactions to get a real block hash
	receipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), rCtx.Evmd.ProcessedTransactions[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt for transaction %s: %w", rCtx.Evmd.ProcessedTransactions[0].Hex(), err)
	}

	// Use the block hash from the receipt to test getBlockByHash
	block, err := rCtx.Evmd.BlockByHash(context.Background(), receipt.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("block hash lookup failed for hash %s from receipt: %w", receipt.BlockHash.Hex(), err)
	}

	// Perform dual API comparison if enabled - use different block hashes for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetBlockByHash, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.Geth.ProcessedTransactions) > 0 {
			// Get geth receipt to get geth block hash
			if gethReceipt, err := rCtx.Geth.TransactionReceipt(context.Background(), rCtx.Geth.ProcessedTransactions[0]); err == nil {
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

	return result, nil
}

func EthGetBlockByNumber(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Get a receipt from one of our processed transactions to get a real block hash
	receipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), rCtx.Evmd.ProcessedTransactions[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt for transaction %s: %w", rCtx.Evmd.ProcessedTransactions[0].Hex(), err)
	}

	// Use the block hash from the receipt to test getBlockByHash
	block, err := rCtx.Evmd.BlockByNumber(context.Background(), receipt.BlockNumber)
	if err != nil {
		return nil, fmt.Errorf("block hash lookup failed for hash %s from receipt: %w", receipt.BlockHash.Hex(), err)
	}

	// Perform dual API comparison if enabled - use different block hashes for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetBlockByNumber, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.Geth.ProcessedTransactions) > 0 {
			// Get geth receipt to get geth block hash
			if gethReceipt, err := rCtx.Geth.TransactionReceipt(context.Background(), rCtx.Geth.ProcessedTransactions[0]); err == nil {
				return []interface{}{hexutil.EncodeBig(gethReceipt.BlockNumber), true}
			}
		}
		return []interface{}{hexutil.EncodeBig(receipt.BlockNumber), true}
	})

	result := &types.RpcResult{
		Method: MethodNameEthGetBlockByNumber,
		Status: types.Ok,
		Value:  utils.MustBeautifyBlock(types.NewRPCBlock(block)),
	}

	return result, nil
}

func EthSendRawTransactionTransferValue(rCtx *types.RPCContext) (*types.RpcResult, error) {
	// if the transaction is successfully sent
	var testedRPCs []*types.RpcResult
	var err error
	// Create a new transaction
	if rCtx.ChainID, err = rCtx.Evmd.ChainID(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthChainID,
		Status: types.Ok,
		Value:  rCtx.ChainID.String(),
	})

	nonce, err := rCtx.Evmd.PendingNonceAt(context.Background(), rCtx.Evmd.Acc.Address)
	if err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthGetTransactionCount,
		Status: types.Ok,
		Value:  nonce,
	})

	if rCtx.MaxPriorityFeePerGas, err = rCtx.Evmd.SuggestGasTipCap(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthMaxPriorityFeePerGas,
		Status: types.Ok,
		Value:  rCtx.MaxPriorityFeePerGas.String(),
	})
	if rCtx.GasPrice, err = rCtx.Evmd.SuggestGasPrice(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthGasPrice,
		Status: types.Ok,
		Value:  rCtx.GasPrice.String(),
	})

	randomRecipient := utils.MustCreateRandomAccount().Address
	value := new(big.Int).SetUint64(1)
	balanceBeforeSend, err := rCtx.Evmd.BalanceAt(context.Background(), rCtx.Evmd.Acc.Address, nil)
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
	signedTx, err := gethtypes.SignTx(tx, signer, rCtx.Evmd.Acc.PrivKey)
	if err != nil {
		return nil, err
	}

	if err = rCtx.Evmd.SendTransaction(context.Background(), signedTx); err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: MethodNameEthSendRawTransaction,
		Status: types.Ok,
		Value:  signedTx.Hash().Hex(),
	}
	testedRPCs = append(testedRPCs, result)

	// wait for the transaction to be mined
	tout, _ := time.ParseDuration(rCtx.Conf.Timeout)
	if _, err = utils.WaitForTx(rCtx, signedTx.Hash(), tout, false); err != nil {
		return nil, err
	}

	balance, err := rCtx.Evmd.BalanceAt(context.Background(), rCtx.Evmd.Acc.Address, nil)
	if err != nil {
		return nil, err
	}
	// check if the balance decreased by the value of the transaction (+ gas fee)
	if new(big.Int).Sub(balanceBeforeSend, balance).Cmp(value) < 0 {
		return nil, errors.New("balanceBeforeSend mismatch, maybe the transaction was not mined or implementation is incorrect")
	}

	return result, nil
}

func EthSendRawTransactionDeployContract(rCtx *types.RPCContext) (*types.RpcResult, error) {
	// if the transaction is successfully sent
	var testedRPCs []*types.RpcResult
	var err error
	// Create a new transaction
	if rCtx.ChainID, err = rCtx.Evmd.ChainID(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthChainID,
		Status: types.Ok,
		Value:  rCtx.ChainID.String(),
	})

	nonce, err := rCtx.Evmd.PendingNonceAt(context.Background(), rCtx.Evmd.Acc.Address)
	if err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthGetTransactionCount,
		Status: types.Ok,
		Value:  nonce,
	})

	if rCtx.MaxPriorityFeePerGas, err = rCtx.Evmd.SuggestGasTipCap(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthMaxPriorityFeePerGas,
		Status: types.Ok,
		Value:  rCtx.MaxPriorityFeePerGas.String(),
	})
	if rCtx.GasPrice, err = rCtx.Evmd.SuggestGasPrice(context.Background()); err != nil {
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
	signedTx, err := gethtypes.SignTx(tx, signer, rCtx.Evmd.Acc.PrivKey)
	if err != nil {
		return nil, err
	}

	if err = rCtx.Evmd.SendTransaction(context.Background(), signedTx); err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: MethodNameEthSendRawTransaction,
		Status: types.Ok,
		Value:  signedTx.Hash().Hex(),
	}
	testedRPCs = append(testedRPCs, result)

	// wait for the transaction to be mined
	tout, _ := time.ParseDuration(rCtx.Conf.Timeout)
	if _, err = utils.WaitForTx(rCtx, signedTx.Hash(), tout, false); err != nil {
		return nil, err
	}

	if rCtx.Evmd.ERC20Addr == (common.Address{}) {
		return nil, errors.New("contract address is empty, failed to deploy")
	}

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
	// if the transaction is successfully sent
	var testedRPCs []*types.RpcResult
	var err error
	// Create a new transaction
	if rCtx.ChainID, err = rCtx.Evmd.ChainID(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthChainID,
		Status: types.Ok,
		Value:  rCtx.ChainID.String(),
	})

	nonce, err := rCtx.Evmd.PendingNonceAt(context.Background(), rCtx.Evmd.Acc.Address)
	if err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthGetTransactionCount,
		Status: types.Ok,
		Value:  nonce,
	})

	if rCtx.MaxPriorityFeePerGas, err = rCtx.Evmd.SuggestGasTipCap(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthMaxPriorityFeePerGas,
		Status: types.Ok,
		Value:  rCtx.MaxPriorityFeePerGas.String(),
	})
	if rCtx.GasPrice, err = rCtx.Evmd.SuggestGasPrice(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthGasPrice,
		Status: types.Ok,
		Value:  rCtx.GasPrice.String(),
	})

	randomRecipient := utils.MustCreateRandomAccount().Address
	data, err := rCtx.Evmd.ERC20Abi.Pack("transfer", randomRecipient, new(big.Int).SetUint64(1))
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
		To:        &rCtx.Evmd.ERC20Addr,
		Data:      data,
	})

	// TODO: Make signer using types.MakeSigner with chain params
	signer := gethtypes.NewLondonSigner(rCtx.ChainID)
	signedTx, err := gethtypes.SignTx(tx, signer, rCtx.Evmd.Acc.PrivKey)
	if err != nil {
		return nil, err
	}

	if err = rCtx.Evmd.SendTransaction(context.Background(), signedTx); err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: MethodNameEthSendRawTransaction,
		Status: types.Ok,
		Value:  signedTx.Hash().Hex(),
	}
	testedRPCs = append(testedRPCs, result)

	// wait for the transaction to be mined
	tout, _ := time.ParseDuration(rCtx.Conf.Timeout)
	if _, err = utils.WaitForTx(rCtx, signedTx.Hash(), tout, false); err != nil {
		return nil, err
	}

	return result, nil
}

func EthGetBlockReceipts(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// TODO: Random pick
	// pick a block with transactions
	blkNum := rCtx.Evmd.BlockNumsIncludingTx[0]
	if blkNum > uint64(math.MaxInt64) {
		return nil, fmt.Errorf("block number %d exceeds int64 max value", blkNum)
	}
	rpcBlockNum := ethrpc.BlockNumber(int64(blkNum))
	receipts, err := rCtx.Evmd.BlockReceipts(context.Background(), ethrpc.BlockNumberOrHash{BlockNumber: &rpcBlockNum})
	if err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled - use different block numbers for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetBlockReceipts, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.Geth.BlockNumsIncludingTx) > 0 {
			return []interface{}{fmt.Sprintf("0x%x", rCtx.Geth.BlockNumsIncludingTx[0])}
		} else if len(rCtx.Evmd.BlockNumsIncludingTx) > 0 {
			return []interface{}{fmt.Sprintf("0x%x", rCtx.Evmd.BlockNumsIncludingTx[0])}
		}
		return []interface{}{"latest"}
	})

	result := &types.RpcResult{
		Method: MethodNameEthGetBlockReceipts,
		Status: types.Ok,
		Value:  utils.MustBeautifyReceipts(receipts),
	}

	return result, nil
}

func EthGetTransactionByHash(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// TODO: Random pick
	txHash := rCtx.Evmd.ProcessedTransactions[0]

	tx, _, err := rCtx.Evmd.TransactionByHash(context.Background(), txHash)
	if err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled - use different transaction hashes for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetTransactionByHash, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.Geth.ProcessedTransactions) > 0 {
			return []interface{}{rCtx.Geth.ProcessedTransactions[0].Hex()}
		}
		return []interface{}{txHash.Hex()}
	})

	result := &types.RpcResult{
		Method: MethodNameEthGetTransactionByHash,
		Status: types.Ok,
		Value:  utils.MustBeautifyTransaction(tx),
	}

	return result, nil
}

func EthGetTransactionByBlockHashAndIndex(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Get a receipt from one of our processed transactions to get a real block hash
	receipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), rCtx.Evmd.ProcessedTransactions[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt for transaction %s: %w", rCtx.Evmd.ProcessedTransactions[0].Hex(), err)
	}

	// Use the transaction index from the receipt and block hash from the receipt
	tx, err := rCtx.Evmd.TransactionInBlock(context.Background(), receipt.BlockHash, receipt.TransactionIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction at index %d in block %s: %w", receipt.TransactionIndex, receipt.BlockHash.Hex(), err)
	}

	// Perform dual API comparison if enabled - use different transaction data for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetTransactionByBlockHashAndIndex, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.Geth.ProcessedTransactions) > 0 {
			// Get geth transaction receipt to get block hash and index
			if receipt, err := rCtx.Geth.TransactionReceipt(context.Background(), rCtx.Geth.ProcessedTransactions[0]); err == nil {
				return []interface{}{receipt.BlockHash.Hex(), fmt.Sprintf("0x%x", receipt.TransactionIndex)}
			}
		} else if len(rCtx.Evmd.ProcessedTransactions) > 0 {
			// Get evmd transaction receipt to get block hash and index
			if receipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), rCtx.Evmd.ProcessedTransactions[0]); err == nil {
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

	return result, nil
}

func EthGetTransactionByBlockNumberAndIndex(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// TODO: Random pick
	blkNum := rCtx.Evmd.BlockNumsIncludingTx[0]
	var tx gethtypes.Transaction
	if err := rCtx.Evmd.RPCClient().CallContext(context.Background(), &tx, string(MethodNameEthGetTransactionByBlockNumberAndIndex), blkNum, "0x0"); err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparisonWithProvider(MethodNameEthGetTransactionByBlockNumberAndIndex, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.Geth.BlockNumsIncludingTx) > 0 {
			return []interface{}{fmt.Sprintf("0x%x", rCtx.Geth.BlockNumsIncludingTx[0]), "0x0"}
		}
		if len(rCtx.Evmd.BlockNumsIncludingTx) > 0 {
			return []interface{}{fmt.Sprintf("0x%x", rCtx.Evmd.BlockNumsIncludingTx[0]), "0x0"}
		}
		return []interface{}{"latest", "0x0"}
	})

	result := &types.RpcResult{
		Method: MethodNameEthGetTransactionByBlockNumberAndIndex,
		Status: types.Ok,
		Value:  utils.MustBeautifyTransaction(&tx),
	}

	return result, nil
}

func EthGetBlockTransactionCountByNumber(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Get a receipt from one of our processed transactions to get a real block hash
	receipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), rCtx.Evmd.ProcessedTransactions[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt for transaction %s: %w", rCtx.Evmd.ProcessedTransactions[0].Hex(), err)
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparisonWithProvider(MethodNameEthGetBlockTransactionCountByNumber, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.Geth.ProcessedTransactions) > 0 {
			if gethReceipt, err := rCtx.Geth.TransactionReceipt(context.Background(), rCtx.Geth.ProcessedTransactions[0]); err == nil {
				return []interface{}{hexutil.EncodeBig(gethReceipt.BlockNumber)}
			}
		}
		return []interface{}{hexutil.EncodeBig(receipt.BlockNumber)}
	})

	var count interface{}
	err = rCtx.Evmd.RPCClient().CallContext(context.Background(), &count, string(MethodNameEthGetBlockTransactionCountByNumber), hexutil.EncodeBig(receipt.BlockNumber))
	if err != nil {
		return nil, fmt.Errorf("eth_getBlockTransactionCountByNumber call failed: %w", err)
	}

	result := &types.RpcResult{
		Method:   MethodNameEthGetBlockTransactionCountByNumber,
		Status:   types.Ok,
		Value:    count,
		Category: NamespaceEth,
	}

	return result, nil
}

// Uncle methods - these should always return 0 or nil in Cosmos EVM (no uncles in PoS)
func EthGetUncleCountByBlockHash(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Get a block hash - try from processed transactions first, fallback to latest block
	var blockHash common.Hash
	if len(rCtx.Evmd.ProcessedTransactions) > 0 {
		receipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), rCtx.Evmd.ProcessedTransactions[0])
		if err != nil {
			return nil, fmt.Errorf("failed to get receipt: %w", err)
		}
		blockHash = receipt.BlockHash
	} else {
		// Fallback to latest block
		block, err := rCtx.Evmd.BlockByNumber(context.Background(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest block: %w", err)
		}
		blockHash = block.Hash()
	}

	var uncleCount string
	err := rCtx.Evmd.RPCClient().CallContext(context.Background(), &uncleCount, string(MethodNameEthGetUncleCountByBlockHash), blockHash)
	if err != nil {
		return nil, fmt.Errorf("eth_getUncleCountByBlockHash call failed: %w", err)
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparisonWithProvider(MethodNameEthGetUncleCountByBlockHash, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.Geth.ProcessedTransactions) > 0 {
			if gethReceipt, err := rCtx.Geth.TransactionReceipt(context.Background(), rCtx.Geth.ProcessedTransactions[0]); err == nil {
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

	return result, nil
}

func EthGetUncleCountByBlockNumber(rCtx *types.RPCContext) (*types.RpcResult, error) {

	var uncleCount string
	err := rCtx.Evmd.RPCClient().CallContext(context.Background(), &uncleCount, string(MethodNameEthGetUncleCountByBlockNumber), "latest")
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

	return result, nil
}

func EthGetUncleByBlockHashAndIndex(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Get a block hash - try from processed transactions first, fallback to latest block
	var blockHash common.Hash
	if len(rCtx.Evmd.ProcessedTransactions) > 0 {
		receipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), rCtx.Evmd.ProcessedTransactions[0])
		if err != nil {
			return nil, fmt.Errorf("failed to get receipt: %w", err)
		}
		blockHash = receipt.BlockHash
	} else {
		// Fallback to latest block
		block, err := rCtx.Evmd.BlockByNumber(context.Background(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest block: %w", err)
		}
		blockHash = block.Hash()
	}

	var uncle interface{}
	err := rCtx.Evmd.RPCClient().CallContext(context.Background(), &uncle, string(MethodNameEthGetUncleByBlockHashAndIndex), blockHash, "0x0")
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

	return result, nil
}

func EthGetUncleByBlockNumberAndIndex(rCtx *types.RPCContext) (*types.RpcResult, error) {

	var uncle interface{}
	// Get current block number and format as hex
	blockNumber, err := rCtx.Evmd.BlockNumber(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get block number: %w", err)
	}
	blockNumberHex := fmt.Sprintf("0x%x", blockNumber)

	err = rCtx.Evmd.RPCClient().CallContext(context.Background(), &uncle, string(MethodNameEthGetUncleByBlockNumberAndIndex), blockNumberHex, "0x0")
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

	return result, nil
}

func EthGetTransactionCountByHash(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// get block
	blkNum := rCtx.Evmd.BlockNumsIncludingTx[0]
	blk, err := rCtx.Evmd.BlockByNumber(context.Background(), new(big.Int).SetUint64(blkNum))
	if err != nil {
		return nil, err
	}

	var count uint64
	if err = rCtx.Evmd.RPCClient().CallContext(context.Background(), &count, string(MethodNameEthGetTransactionCountByHash), blk.Hash()); err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthGetTransactionCountByHash, blk.Hash())

	result := &types.RpcResult{
		Method: MethodNameEthGetTransactionCountByHash,
		Status: types.Ok,
		Value:  count,
	}

	return result, nil
}

func EthGetTransactionReceipt(rCtx *types.RPCContext) (*types.RpcResult, error) {

	if len(rCtx.Evmd.ProcessedTransactions) == 0 {
		return nil, errors.New("no transactions")
	}

	txHash := rCtx.Evmd.ProcessedTransactions[0]

	receipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), txHash)
	if err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled - use different transaction hashes for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetTransactionReceipt, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.Geth.ProcessedTransactions) > 0 {
			return []interface{}{rCtx.Geth.ProcessedTransactions[0].Hex()}
		}
		return []interface{}{txHash.Hex()}
	})

	result := &types.RpcResult{
		Method: MethodNameEthGetTransactionReceipt,
		Status: types.Ok,
		Value:  utils.MustBeautifyReceipt(receipt),
	}

	return result, nil
}

func EthGetBlockTransactionCountByHash(rCtx *types.RPCContext) (*types.RpcResult, error) {

	if len(rCtx.Evmd.ProcessedTransactions) == 0 {
		return nil, errors.New("no processed transactions available - run transaction generation first")
	}

	// Get a receipt from one of our processed transactions to get a real block hash
	receipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), rCtx.Evmd.ProcessedTransactions[0])
	if err != nil {
		return nil, fmt.Errorf("failed to get receipt for transaction %s: %w", rCtx.Evmd.ProcessedTransactions[0].Hex(), err)
	}

	count, err := rCtx.Evmd.TransactionCount(context.Background(), receipt.BlockHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction count for block hash %s: %w", receipt.BlockHash.Hex(), err)
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparisonWithProvider(MethodNameEthGetBlockTransactionCountByHash, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.Geth.ProcessedTransactions) > 0 {
			if gethReceipt, err := rCtx.Geth.TransactionReceipt(context.Background(), rCtx.Geth.ProcessedTransactions[0]); err == nil {
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

	return result, nil
}

func EthGetCode(rCtx *types.RPCContext) (*types.RpcResult, error) {

	if rCtx.Evmd.ERC20Addr == (common.Address{}) {
		return nil, errors.New("no contract address, must be deployed first")
	}

	code, err := rCtx.Evmd.CodeAt(context.Background(), rCtx.Evmd.ERC20Addr, nil)
	if err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled - use different contract addresses for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetCode, func(isGeth bool) []interface{} {
		if isGeth && rCtx.Geth.ERC20Addr != (common.Address{}) {
			return []interface{}{rCtx.Geth.ERC20Addr.Hex(), "latest"}
		}
		return []interface{}{rCtx.Evmd.ERC20Addr.Hex(), "latest"}
	})

	result := &types.RpcResult{
		Method: MethodNameEthGetCode,
		Status: types.Ok,
		Value:  hexutils.BytesToHex(code),
	}

	return result, nil
}

func EthGetStorageAt(rCtx *types.RPCContext) (*types.RpcResult, error) {

	if rCtx.Evmd.ERC20Addr == (common.Address{}) {
		return nil, errors.New("no contract address, must be deployed first")
	}

	key := utils.MustCalculateSlotKey(rCtx.Evmd.Acc.Address, 4)

	// Perform dual API comparison if enabled - use different contract addresses for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetStorageAt, func(isGeth bool) []interface{} {
		if isGeth && rCtx.Geth.ERC20Addr != (common.Address{}) {
			return []interface{}{rCtx.Geth.ERC20Addr.Hex(), fmt.Sprintf("0x%x", key), "latest"}
		}
		return []interface{}{rCtx.Evmd.ERC20Addr.Hex(), fmt.Sprintf("0x%x", key), "latest"}
	})

	storage, err := rCtx.Evmd.StorageAt(context.Background(), rCtx.Evmd.ERC20Addr, key, nil)
	if err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method:   MethodNameEthGetStorageAt,
		Status:   types.Ok,
		Value:    hexutils.BytesToHex(storage),
		Category: NamespaceEth,
	}

	return result, nil
}

func EthNewFilter(rCtx *types.RPCContext) (*types.RpcResult, error) {

	fErc20Transfer := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(rCtx.Evmd.BlockNumsIncludingTx[0] - 1),
		Addresses: []common.Address{rCtx.Evmd.ERC20Addr},
		Topics: [][]common.Hash{
			{rCtx.Evmd.ERC20Abi.Events["Transfer"].ID}, // Filter for Transfer event
		},
	}
	args, err := utils.ToFilterArg(fErc20Transfer)
	if err != nil {
		return nil, err
	}
	var filterID string
	if err = rCtx.Evmd.RPCClient().CallContext(context.Background(), &filterID, string(MethodNameEthNewFilter), args); err != nil {
		return nil, err
	}

	fErc20TransferGeth := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(rCtx.Geth.BlockNumsIncludingTx[0] - 1),
		Addresses: []common.Address{rCtx.Geth.ERC20Addr},
		Topics: [][]common.Hash{
			{rCtx.Evmd.ERC20Abi.Events["Transfer"].ID}, // Filter for Transfer event
		},
	}
	argsGeth, err := utils.ToFilterArg(fErc20TransferGeth)
	if err != nil {
		return nil, err
	}
	var filterIDGeth string
	if err = rCtx.Geth.RPCClient().CallContext(context.Background(), &filterIDGeth, string(MethodNameEthNewFilter), argsGeth); err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled - use different block hashes for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthNewFilter, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.Geth.ProcessedTransactions) > 0 {
			return []interface{}{argsGeth}
		}
		return []interface{}{args}
	})

	result := &types.RpcResult{
		Method: MethodNameEthNewFilter,
		Status: types.Ok,
		Value:  filterID,
	}
	rCtx.Evmd.FilterID = filterID
	rCtx.Evmd.FilterQuery = fErc20Transfer
	rCtx.Geth.FilterID = filterIDGeth
	rCtx.Geth.FilterQuery = fErc20TransferGeth

	return result, nil
}

func EthGetFilterLogs(rCtx *types.RPCContext) (*types.RpcResult, error) {

	if rCtx.Evmd.FilterID == "" {
		return nil, errors.New("no filter id, must create a filter first")
	}

	if _, err := EthSendRawTransactionTransferERC20(rCtx); err != nil {
		return nil, errors.New("transfer ERC20 must be succeeded before checking filter logs")
	}

	var logs []gethtypes.Log
	if err := rCtx.Evmd.RPCClient().CallContext(context.Background(), &logs, string(MethodNameEthGetFilterLogs), rCtx.Evmd.FilterID); err != nil {
		return nil, err
	}

	fErc20TransferGeth := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(rCtx.Geth.BlockNumsIncludingTx[0] - 1),
		Addresses: []common.Address{rCtx.Geth.ERC20Addr},
		Topics: [][]common.Hash{
			{rCtx.Evmd.ERC20Abi.Events["Transfer"].ID}, // Filter for Transfer event
		},
	}
	argsGeth, err := utils.ToFilterArg(fErc20TransferGeth)
	if err != nil {
		return nil, err
	}
	var filterIDGeth string
	if err = rCtx.Geth.RPCClient().CallContext(context.Background(), &filterIDGeth, string(MethodNameEthNewFilter), argsGeth); err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled - use different block hashes for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetFilterLogs, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.Evmd.ProcessedTransactions) > 0 {
			return []interface{}{filterIDGeth}
		}
		return []interface{}{rCtx.Evmd.FilterID}
	})

	result := &types.RpcResult{
		Method: MethodNameEthGetFilterLogs,
		Status: types.Ok,
		Value:  utils.MustBeautifyLogs(logs),
	}

	return result, nil
}

func EthNewBlockFilter(rCtx *types.RPCContext) (*types.RpcResult, error) {

	var filterID string
	if err := rCtx.Evmd.Client.Client().CallContext(context.Background(), &filterID, string(MethodNameEthNewBlockFilter)); err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthNewBlockFilter)

	result := &types.RpcResult{
		Method: MethodNameEthNewBlockFilter,
		Status: types.Ok,
		Value:  filterID,
	}
	rCtx.Evmd.BlockFilterID = filterID

	return result, nil
}

func EthGetFilterChanges(rCtx *types.RPCContext) (*types.RpcResult, error) {

	var changes []interface{}
	if err := rCtx.Evmd.RPCClient().CallContext(context.Background(), &changes, string(MethodNameEthGetFilterChanges), rCtx.Evmd.BlockFilterID); err != nil {
		return nil, err
	}

	fErc20TransferGeth := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(rCtx.Evmd.BlockNumsIncludingTx[0] - 1),
		Addresses: []common.Address{rCtx.Evmd.ERC20Addr},
		Topics: [][]common.Hash{
			{rCtx.Evmd.ERC20Abi.Events["Transfer"].ID}, // Filter for Transfer event
		},
	}
	argsGeth, err := utils.ToFilterArg(fErc20TransferGeth)
	if err != nil {
		return nil, err
	}
	var filterIDGeth string
	if err = rCtx.Geth.RPCClient().CallContext(context.Background(), &filterIDGeth, string(MethodNameEthNewFilter), argsGeth); err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled - use different block hashes for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetFilterChanges, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.Evmd.ProcessedTransactions) > 0 {
			return []interface{}{filterIDGeth}
		}
		return []interface{}{rCtx.Evmd.BlockFilterID}
	})

	status := types.Ok
	// Empty results are valid - no warnings needed

	result := &types.RpcResult{
		Method:   MethodNameEthGetFilterChanges,
		Status:   status,
		Value:    changes,
		Category: NamespaceEth,
	}

	return result, nil
}

func EthUninstallFilter(rCtx *types.RPCContext) (*types.RpcResult, error) {

	_, filterID, err := utils.NewERC20FilterLogs(rCtx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create filter logs: %w", err)
	}

	var res bool
	if err := rCtx.Evmd.RPCClient().CallContext(context.Background(), &res, string(MethodNameEthUninstallFilter), filterID); err != nil {
		return nil, err
	}
	if !res {
		return nil, errors.New("uninstall filter failed")
	}

	if err := rCtx.Evmd.RPCClient().CallContext(context.Background(), &res, string(MethodNameEthUninstallFilter), filterID); err != nil {
		return nil, err
	}
	if res {
		return nil, errors.New("uninstall filter should be failed because it was already uninstalled")
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthUninstallFilter, filterID)

	result := &types.RpcResult{
		Method: MethodNameEthUninstallFilter,
		Status: types.Ok,
		Value:  filterID,
	}

	return result, nil
}

func EthGetLogs(rCtx *types.RPCContext) (*types.RpcResult, error) {

	if _, err := EthNewFilter(rCtx); err != nil {
		return nil, errors.New("failed to create a filter")
	}

	if _, err := EthSendRawTransactionTransferERC20(rCtx); err != nil {
		return nil, errors.New("transfer ERC20 must be succeeded before checking filter logs")
	}

	// set from block because of limit
	logs, err := rCtx.Evmd.FilterLogs(context.Background(), rCtx.Evmd.FilterQuery)
	if err != nil {
		return nil, err
	}

	args, err := utils.ToFilterArg(rCtx.Evmd.FilterQuery)
	if err != nil {
		return nil, err
	}

	fErc20TransferGeth := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(rCtx.Geth.BlockNumsIncludingTx[0] - 1),
		Addresses: []common.Address{rCtx.Geth.ERC20Addr},
		Topics: [][]common.Hash{
			{rCtx.Evmd.ERC20Abi.Events["Transfer"].ID}, // Filter for Transfer event
		},
	}
	argsGeth, err := utils.ToFilterArg(fErc20TransferGeth)
	if err != nil {
		return nil, err
	}

	// Perform dual API comparison if enabled - use different block hashes for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthGetLogs, func(isGeth bool) []interface{} {
		if isGeth && len(rCtx.Evmd.ProcessedTransactions) > 0 {
			return []interface{}{argsGeth}
		}
		return []interface{}{args}
	})

	status := types.Ok
	// Empty results are valid - no warnings needed

	result := &types.RpcResult{
		Method:   MethodNameEthGetLogs,
		Status:   status,
		Value:    utils.MustBeautifyLogs(logs),
		Category: NamespaceEth,
	}

	return result, nil
}

// Additional Eth method handlers
func EthProtocolVersion(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.Evmd.RPCClient().Call(&result, "eth_protocolVersion")
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
	err := rCtx.Evmd.RPCClient().Call(&result, "eth_syncing")
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
	err := rCtx.Evmd.RPCClient().Call(&result, "eth_accounts")
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
	err := rCtx.Evmd.RPCClient().Call(&result, "eth_mining")
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
	err := rCtx.Evmd.RPCClient().Call(&result, "eth_hashrate")
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
		To:   &rCtx.Evmd.Acc.Address,
		Data: []byte{},
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameEthCall, map[string]interface{}{
		"to":   rCtx.Evmd.Acc.Address.Hex(),
		"data": "0x",
	}, "latest")

	result, err := rCtx.Evmd.CallContract(context.Background(), callMsg, nil)
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

// estimateNativeTransferGas exercises the simplest path – sending native ETH
// from the pre-funded dev0 account to a second dev account – so we can compare
// Cosmos EVM and go-ethereum behaviour for plain value transfers.
func estimateNativeTransferGas(rCtx *types.RPCContext) (string, error) {
	recipient := utils.StandardDevAccounts["dev2"]
	callMsg := ethereum.CallMsg{
		From:  rCtx.Evmd.Acc.Address,
		To:    &recipient,
		Value: big.NewInt(1e15),
	}

	gasLimit, err := rCtx.Evmd.EstimateGas(context.Background(), callMsg)
	if err != nil {
		return "", err
	}

	if rCtx.EnableComparison && rCtx.Geth != nil && rCtx.Geth.Client != nil {
		gethGasLimit, err := rCtx.Geth.EstimateGas(context.Background(), callMsg)
		if err != nil {
			return "", fmt.Errorf("geth estimateGas failed: %w", err)
		}
		if gasLimit != gethGasLimit {
			return "", fmt.Errorf("estimateGas mismatch (native): evmd=%d geth=%d", gasLimit, gethGasLimit)
		}
	}

	return fmt.Sprintf("0x%x", gasLimit), nil
}

// estimateERC20TransferGas measures gas usage for a token transfer against the
// ERC20 instance that Setup() deploys and mints to the standard dev accounts.
// This mirrors the real execution path (contract code lives on-chain already).
func estimateERC20TransferGas(rCtx *types.RPCContext) (string, bool, error) {
	if rCtx.Evmd.ERC20Abi == nil || rCtx.Evmd.ERC20Addr == (common.Address{}) {
		return "", true, nil
	}

	sender := utils.StandardDevAccounts["dev1"]
	recipient := utils.StandardDevAccounts["dev2"]
	amount := new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18))

	data, err := rCtx.Evmd.ERC20Abi.Pack("transfer", recipient, amount)
	if err != nil {
		return "", false, err
	}

	erc20Call := ethereum.CallMsg{From: sender, To: &rCtx.Evmd.ERC20Addr, Data: data}

	if rCtx.EnableComparison {
		rCtx.PerformComparison(MethodNameEthEstimateGas, map[string]interface{}{
			"from": sender.Hex(),
			"to":   rCtx.Evmd.ERC20Addr.Hex(),
			"data": "0x" + hex.EncodeToString(data),
		})
	}

	gas, err := rCtx.Evmd.EstimateGas(context.Background(), erc20Call)
	if err != nil {
		return "", false, err
	}

	if rCtx.EnableComparison && rCtx.Geth != nil && rCtx.Geth.Client != nil {
		gethGas, err := rCtx.Geth.EstimateGas(context.Background(), erc20Call)
		if err != nil {
			return "", false, fmt.Errorf("geth estimateGas failed: %w", err)
		}
		if gas != gethGas {
			return "", false, fmt.Errorf("estimateGas mismatch (erc20 transfer): evmd=%d geth=%d", gas, gethGas)
		}
	}

	return fmt.Sprintf("0x%x", gas), false, nil
}

// estimateERC20OverrideGas simulates the "deploy via state override" RPC usage:
//   - inject runtime bytecode at a synthetic contract address
//   - seed balances[dev1] so the transfer can succeed
//   - provide enough native balance for dev1 to cover gas
//
// This ensures our override behaviour matches geth for contract calls without
// first deploying on-chain.
func estimateERC20OverrideGas(rCtx *types.RPCContext) (string, bool, error) {
	const overrideContractHex = "0x5555555555555555555555555555555555555555"

	if rCtx.Evmd.ERC20Abi == nil || len(rCtx.Evmd.ERC20ByteCode) == 0 {
		return "", true, nil
	}

	runtimeHex, err := utils.ExtractRuntimeBytecodeHex(rCtx.Evmd.ERC20ByteCode)
	if err != nil {
		return "", true, nil
	}

	overrideSender := utils.StandardDevAccounts["dev1"]
	overrideRecipient := utils.StandardDevAccounts["dev2"]
	overrideAmount := new(big.Int).Mul(big.NewInt(5), big.NewInt(1e18))

	data, err := rCtx.Evmd.ERC20Abi.Pack("transfer", overrideRecipient, overrideAmount)
	if err != nil {
		return "", false, err
	}

	// balanceOf is stored at slot 4 in the deployed ERC20 layout.
	// (name, symbol, decimals, totalSupply precede the mapping.)
	const balanceMappingSlot = 4
	slotKey := utils.MustCalculateSlotKey(overrideSender, balanceMappingSlot)
	tokenBalance := fmt.Sprintf("0x%064x", overrideAmount)
	fromBalance := fmt.Sprintf("0x%x", new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)))

	overrideParams := map[string]interface{}{
		"from": overrideSender.Hex(),
		"to":   overrideContractHex,
		"data": "0x" + hex.EncodeToString(data),
	}
	overrideState := map[string]interface{}{
		overrideContractHex: map[string]interface{}{
			"code": runtimeHex,
			"stateDiff": map[string]string{
				slotKey.Hex(): tokenBalance,
			},
		},
		overrideSender.Hex(): map[string]string{
			"balance": fromBalance,
		},
	}

	if rCtx.EnableComparison {
		rCtx.PerformComparison(MethodNameEthEstimateGas, overrideParams, "latest", overrideState)
	}

	var overrideGas hexutil.Uint64
	if err := rCtx.Evmd.RPCClient().Call(&overrideGas, string(MethodNameEthEstimateGas), overrideParams, "latest", overrideState); err != nil {
		return "", false, err
	}

	if rCtx.EnableComparison && rCtx.Geth != nil && rCtx.Geth.Client != nil {
		var gethGas hexutil.Uint64
		if err := rCtx.Geth.RPCClient().Call(&gethGas, string(MethodNameEthEstimateGas), overrideParams, "latest", overrideState); err != nil {
			return "", false, fmt.Errorf("geth estimateGas failed: %w", err)
		}
		if uint64(overrideGas) != uint64(gethGas) {
			return "", false, fmt.Errorf("estimateGas mismatch (erc20 override): evmd=%d geth=%d", uint64(overrideGas), uint64(gethGas))
		}
	}

	return fmt.Sprintf("0x%x", uint64(overrideGas)), false, nil
}

// EthEstimateGas aggregates native ETH, deployed ERC20, and override ERC20
// scenarios so we continuously exercise the major gas-estimation paths.
func EthEstimateGas(rCtx *types.RPCContext) (*types.RpcResult, error) {
	if rCtx.EnableComparison {
		rCtx.PerformComparison(MethodNameEthEstimateGas, map[string]interface{}{
			"from":  rCtx.Evmd.Acc.Address.Hex(),
			"to":    rCtx.Evmd.Acc.Address.Hex(),
			"value": "0x0",
		})
	}

	// Collect individual scenario results keyed by a descriptive name.
	results := map[string]string{}

	single, err := estimateNativeTransferGas(rCtx)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthEstimateGas,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceEth,
		}, nil
	}
	singleKey := "nativeTransfer"
	results[singleKey] = single

	if erc20Gas, skipped, err := estimateERC20TransferGas(rCtx); err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthEstimateGas,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceEth,
		}, nil
	} else if skipped {
		results["erc20Transfer"] = "skipped"
	} else {
		results["erc20Transfer"] = erc20Gas
	}

	if overrideGas, skipped, err := estimateERC20OverrideGas(rCtx); err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthEstimateGas,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceEth,
		}, nil
	} else if skipped {
		results["erc20Override"] = "skipped"
	} else {
		results["erc20Override"] = overrideGas
	}

	return &types.RpcResult{
		Method:   MethodNameEthEstimateGas,
		Status:   types.Ok,
		Value:    results,
		Category: NamespaceEth,
	}, nil
}

func EthFeeHistory(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.Evmd.RPCClient().Call(&result, string(MethodNameEthFeeHistory), "0x2", "latest", []float64{25.0, 50.0, 75.0})
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

	blockNumber := rCtx.Evmd.BlockNumsIncludingTx[0]
	blockNumberHex := fmt.Sprintf("0x%x", blockNumber)

	err := rCtx.Evmd.RPCClient().Call(&result, string(MethodNameEthGetProof), rCtx.Evmd.Acc.Address.Hex(), []string{}, blockNumberHex)

	rCtx.PerformComparisonWithProvider(MethodNameEthGetProof, func(isGeth bool) []interface{} {
		if isGeth {
			blockNumber := rCtx.Geth.BlockNumsIncludingTx[0]
			blockNumberHex := fmt.Sprintf("0x%x", blockNumber)
			return []interface{}{rCtx.Geth.Acc.Address.Hex(), []string{}, blockNumberHex}
		}
		return []interface{}{rCtx.Evmd.Acc.Address.Hex(), []string{}, blockNumberHex}
	})

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

	// Create a simple transaction object for testing
	tx := map[string]interface{}{
		"from":     rCtx.Evmd.Acc.Address.Hex(),
		"to":       "0x963EBDf2e1f8DB8707D05FC75bfeFFBa1B5BaC17", // Bank precompile
		"value":    "0x1",                                        // 1 wei
		"gas":      "0x5208",                                     // 21000 gas
		"gasPrice": "0x9184e72a000",                              // 10000000000000
	}

	var txHash string
	err := rCtx.Evmd.RPCClient().Call(&txHash, string(MethodNameEthSendTransaction), tx)
	if err != nil {
		// Key not found errors should now be treated as failures since we have keys in keyring
		return &types.RpcResult{
			Method:   MethodNameEthSendTransaction,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Transaction signing failed - keys should be available in keyring: %s", err.Error()),
			Category: NamespaceEth,
		}, nil
	}

	// Perform dual API comparison if enabled - use different block hashes for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthSendTransaction, func(isGeth bool) []interface{} {
		tx := map[string]interface{}{
			"to":       "0x963EBDf2e1f8DB8707D05FC75bfeFFBa1B5BaC17", // dev1 account address
			"value":    "0x1",                                        // 1 wei
			"gas":      "0x5208",                                     // 21000 gas
			"gasPrice": "0x9184e72a000",                              // 10000000000000
		}

		if isGeth {
			var result []string
			err := rCtx.Geth.RPCClient().Call(&result, string(MethodNameEthAccounts))
			if err != nil {
				return nil
			}
			tx["from"] = result[0] // Use the first account address from Geth
		} else {
			tx["from"] = rCtx.Evmd.Acc.Address.Hex() // Use the account address from RPC context
		}
		return []interface{}{tx}
	})

	result := &types.RpcResult{
		Method:   MethodNameEthSendTransaction,
		Status:   types.Ok,
		Value:    txHash,
		Category: NamespaceEth,
	}
	return result, nil
}

// EthSign signs data using eth_sign
// This requires the account to be unlocked or managed by the node
func EthSign(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Test data to sign (32-byte hash)
	testData := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	var signature string
	err := rCtx.Evmd.RPCClient().Call(&signature, string(MethodNameEthSign), rCtx.Evmd.Acc.Address.Hex(), testData)
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

	// Perform dual API comparison if enabled - use different block hashes for each client
	rCtx.PerformComparisonWithProvider(MethodNameEthSign, func(isGeth bool) []interface{} {
		testData := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
		acc := rCtx.Evmd.Acc.Address.Hex()

		if isGeth {
			var result []string
			err := rCtx.Geth.RPCClient().Call(&result, string(MethodNameEthAccounts))
			if err != nil {
				return nil
			}
			acc = result[0] // Use the first account address from Geth
		}
		return []interface{}{acc, testData}
	})

	result := &types.RpcResult{
		Method:   MethodNameEthSign,
		Status:   types.Ok,
		Value:    signature,
		Category: NamespaceEth,
	}
	return result, nil
}

func EthCreateAccessList(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	callData := map[string]interface{}{
		"from": rCtx.Evmd.Acc.Address.Hex(),
		"to":   rCtx.Evmd.Acc.Address.Hex(),
		"data": "0x",
	}
	err := rCtx.Evmd.RPCClient().Call(&result, string(MethodNameEthCreateAccessList), callData, "latest")

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

func EthGetHeaderByHash(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Get a block hash from processed transactions
	receipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), rCtx.Evmd.ProcessedTransactions[0])
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthGetHeaderByHash,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Failed to get transaction receipt: %v", err),
			Category: NamespaceEth,
		}, nil
	}

	var header any
	err = rCtx.Evmd.RPCClient().Call(&header, string(MethodNameEthGetHeaderByHash), receipt.BlockHash.Hex())
	if err != nil {
		if strings.Contains(err.Error(), "does not exist/is not available") ||
			strings.Contains(err.Error(), "Method not found") {
			result := &types.RpcResult{
				Method:   MethodNameEthGetHeaderByHash,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: NamespaceEth,
			}
			return result, nil
		}
		result := &types.RpcResult{
			Method:   MethodNameEthGetHeaderByHash,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceEth,
		}
		return result, nil
	}

	// Validate header structure
	validationErrors := []string{}
	if header == nil {
		validationErrors = append(validationErrors, "header is null")
	} else if headerMap, ok := header.(map[string]any); ok {
		// Check for required header fields
		requiredFields := []string{"number", "hash", "parentHash", "timestamp", "gasUsed", "gasLimit"}
		for _, field := range requiredFields {
			if _, exists := headerMap[field]; !exists {
				validationErrors = append(validationErrors, fmt.Sprintf("missing header field '%s'", field))
			}
		}
	}

	if len(validationErrors) > 0 {
		result := &types.RpcResult{
			Method:   MethodNameEthGetHeaderByHash,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Header validation failed: %s", strings.Join(validationErrors, ", ")),
			Category: NamespaceEth,
		}
		return result, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameEthGetHeaderByHash,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Header retrieved successfully for hash %s", receipt.BlockHash.Hex()[:10]+"..."),
		Category: NamespaceEth,
	}
	return result, nil
}

func EthGetHeaderByNumber(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Get current block number
	blockNumber, err := rCtx.Evmd.BlockNumber(context.Background())
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthGetHeaderByNumber,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Failed to get block number: %v", err),
			Category: NamespaceEth,
		}, nil
	}

	blockNumberHex := fmt.Sprintf("0x%x", blockNumber)

	var header any
	err = rCtx.Evmd.RPCClient().Call(&header, string(MethodNameEthGetHeaderByNumber), blockNumberHex)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist/is not available") ||
			strings.Contains(err.Error(), "Method not found") {
			result := &types.RpcResult{
				Method:   MethodNameEthGetHeaderByNumber,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: NamespaceEth,
			}
			return result, nil
		}
		result := &types.RpcResult{
			Method:   MethodNameEthGetHeaderByNumber,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceEth,
		}
		return result, nil
	}

	// Validate header structure
	validationErrors := []string{}
	if header == nil {
		validationErrors = append(validationErrors, "header is null")
	} else if headerMap, ok := header.(map[string]any); ok {
		// Check for required header fields
		requiredFields := []string{"number", "hash", "parentHash", "timestamp", "gasUsed", "gasLimit"}
		for _, field := range requiredFields {
			if _, exists := headerMap[field]; !exists {
				validationErrors = append(validationErrors, fmt.Sprintf("missing header field '%s'", field))
			}
		}
	}

	if len(validationErrors) > 0 {
		result := &types.RpcResult{
			Method:   MethodNameEthGetHeaderByNumber,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Header validation failed: %s", strings.Join(validationErrors, ", ")),
			Category: NamespaceEth,
		}
		return result, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameEthGetHeaderByNumber,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Header retrieved successfully for block %s", blockNumberHex),
		Category: NamespaceEth,
	}
	return result, nil
}

func EthSimulateV1(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Create a simulation request with test parameters
	simulationReq := map[string]any{
		"blockStateCalls": []map[string]any{
			{
				"blockOverrides": map[string]any{
					"gasLimit": "0x1c9c380", // 30M gas limit
				},
				"calls": []map[string]any{
					{
						"from":  rCtx.Evmd.Acc.Address.Hex(),
						"to":    rCtx.Evmd.Acc.Address.Hex(),
						"gas":   "0x5208", // 21000 gas
						"data":  "0x",
						"value": "0x0",
					},
				},
			},
		},
		"traceTransfers": true,
		"validation":     true,
	}

	var result any
	err := rCtx.Evmd.RPCClient().Call(&result, string(MethodNameEthSimulateV1), simulationReq)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist/is not available") ||
			strings.Contains(err.Error(), "Method not found") ||
			strings.Contains(err.Error(), "method not found") {
			result := &types.RpcResult{
				Method:   MethodNameEthSimulateV1,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM - eth_simulateV1 is a newer Ethereum API",
				Category: NamespaceEth,
			}
			return result, nil
		}
		rpcResult := &types.RpcResult{
			Method:   MethodNameEthSimulateV1,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceEth,
		}
		return rpcResult, nil
	}

	// Validate simulation result structure
	validationErrors := []string{}
	if result == nil {
		validationErrors = append(validationErrors, "simulation result is null")
	} else if resultMap, ok := result.(map[string]any); ok {
		// Check for expected simulation result fields
		if blockResults, exists := resultMap["blockResults"]; !exists {
			validationErrors = append(validationErrors, "missing 'blockResults' in simulation response")
		} else if blockResultsArray, ok := blockResults.([]any); ok {
			if len(blockResultsArray) == 0 {
				validationErrors = append(validationErrors, "blockResults array is empty")
			}
		}
	}

	if len(validationErrors) > 0 {
		rpcResult := &types.RpcResult{
			Method:   MethodNameEthSimulateV1,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Simulation validation failed: %s", strings.Join(validationErrors, ", ")),
			Category: NamespaceEth,
		}
		return rpcResult, nil
	}

	rpcResult := &types.RpcResult{
		Method:   MethodNameEthSimulateV1,
		Status:   types.Ok,
		Value:    "Simulation executed successfully with validation and trace transfers",
		Category: NamespaceEth,
	}
	return rpcResult, nil
}

func EthPendingTransactions(rCtx *types.RPCContext) (*types.RpcResult, error) {

	var pendingTxs any
	err := rCtx.Evmd.RPCClient().Call(&pendingTxs, string(MethodNameEthPendingTransactions))
	if err != nil {
		if strings.Contains(err.Error(), "does not exist/is not available") ||
			strings.Contains(err.Error(), "Method not found") ||
			strings.Contains(err.Error(), "method not found") {
			result := &types.RpcResult{
				Method:   MethodNameEthPendingTransactions,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM - use eth_getPendingTransactions instead",
				Category: NamespaceEth,
			}
			return result, nil
		}
		result := &types.RpcResult{
			Method:   MethodNameEthPendingTransactions,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceEth,
		}
		return result, nil
	}

	// Validate response structure (same validation as eth_getPendingTransactions)
	validationErrors := []string{}
	var txCount int

	if pendingTxs == nil {
		// null response is valid (no pending transactions)
		txCount = 0
	} else if txArray, ok := pendingTxs.([]any); ok {
		txCount = len(txArray)

		// Validate transaction structure if there are pending transactions
		if txCount > 0 {
			for i, tx := range txArray {
				if txMap, ok := tx.(map[string]any); ok {
					// Check for required transaction fields
					requiredFields := []string{"hash", "from", "gas", "gasPrice", "nonce"}
					for _, field := range requiredFields {
						if _, exists := txMap[field]; !exists {
							validationErrors = append(validationErrors, fmt.Sprintf("missing field '%s' in transaction %d", field, i))
							break // Only report first missing field per transaction
						}
					}
				} else {
					validationErrors = append(validationErrors, fmt.Sprintf("transaction %d is not a valid object", i))
					break // Don't check more if structure is wrong
				}

				// Only validate first few transactions to avoid spam
				if i >= 2 {
					break
				}
			}
		}
	} else {
		validationErrors = append(validationErrors, "response is not an array or null")
	}

	if len(validationErrors) > 0 {
		result := &types.RpcResult{
			Method:   MethodNameEthPendingTransactions,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Response validation failed: %s", strings.Join(validationErrors, ", ")),
			Category: NamespaceEth,
		}
		return result, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameEthPendingTransactions,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Retrieved %d pending transactions (go-ethereum compatible method)", txCount),
		Category: NamespaceEth,
	}
	return result, nil
}

// WebSocket subscription request/response structures
type SubscriptionRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type SubscriptionResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

type NotificationMessage struct {
	JSONRPC string            `json:"jsonrpc"`
	Method  string            `json:"method"`
	Params  NotificationParam `json:"params"`
}

type NotificationParam struct {
	Subscription string      `json:"subscription"`
	Result       interface{} `json:"result"`
}

// EthSubscribe tests WebSocket subscription functionality
func EthSubscribe(rCtx *types.RPCContext) (*types.RpcResult, error) {
	wsURL := rCtx.Conf.EvmdWsEndpoint

	// Test all 4 subscription types
	subscriptionTypes := []struct {
		name        string
		params      []interface{}
		description string
	}{
		{
			name:        "newHeads",
			params:      []interface{}{"newHeads"},
			description: "New block headers subscription",
		},
		{
			name:        "logs",
			params:      []interface{}{"logs", map[string]interface{}{}}, // Empty filter for all logs
			description: "Event logs subscription",
		},
		{
			name:        "newPendingTransactions",
			params:      []interface{}{"newPendingTransactions"},
			description: "Pending transactions subscription",
		},
		{
			name:        "syncing",
			params:      []interface{}{"syncing"},
			description: "Synchronization status subscription",
		},
	}

	var results []string
	var failedTests []string

	for _, subType := range subscriptionTypes {
		success, err := testWebSocketSubscription(wsURL, subType.params)
		if success {
			results = append(results, fmt.Sprintf("✓ %s", subType.name))
		} else {
			failedTests = append(failedTests, fmt.Sprintf("✗ %s: %v", subType.name, err))
			results = append(results, fmt.Sprintf("✗ %s", subType.name))
		}
	}

	// Determine overall result
	switch {
	case len(failedTests) == 0:
		return &types.RpcResult{
			Method:   MethodNameEthSubscribe,
			Status:   types.Ok,
			Value:    fmt.Sprintf("All 4 subscription types working: %v", results),
			Category: NamespaceEth,
		}, nil
	case len(failedTests) < len(subscriptionTypes):
		return &types.RpcResult{
			Method:   MethodNameEthSubscribe,
			Status:   types.Ok,
			Value:    fmt.Sprintf("Partial support (%d/%d): %v", len(subscriptionTypes)-len(failedTests), len(subscriptionTypes), results),
			Category: NamespaceEth,
		}, nil
	default:
		return &types.RpcResult{
			Method:   MethodNameEthSubscribe,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("All subscription types failed: %v", failedTests),
			Category: NamespaceEth,
		}, nil
	}
}

// EthUnsubscribe tests WebSocket unsubscription functionality
func EthUnsubscribe(rCtx *types.RPCContext) (*types.RpcResult, error) {
	wsURL := rCtx.Conf.EvmdWsEndpoint

	// Test unsubscription by creating a subscription first, then unsubscribing
	success, subscriptionID, err := testWebSocketUnsubscribe(wsURL)
	if success {
		return &types.RpcResult{
			Method:   MethodNameEthUnsubscribe,
			Status:   types.Ok,
			Value:    fmt.Sprintf("Successfully unsubscribed from subscription: %s", subscriptionID),
			Category: NamespaceEth,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameEthUnsubscribe,
		Status:   types.Error,
		ErrMsg:   fmt.Sprintf("Failed to test unsubscribe: %v", err),
		Category: NamespaceEth,
	}, nil
}

// testWebSocketSubscription tests a specific subscription type
func testWebSocketSubscription(wsURL string, params []interface{}) (bool, error) {
	// Parse the WebSocket URL
	u, err := url.Parse(wsURL)
	if err != nil {
		return false, fmt.Errorf("failed to parse WebSocket URL: %v", err)
	}

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return false, fmt.Errorf("failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Set connection timeout
	err = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err != nil {
		return false, fmt.Errorf("failed to set read deadline")
	}
	err = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		return false, fmt.Errorf("failed to set write deadline")
	}

	// Send subscription request
	request := SubscriptionRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "eth_subscribe",
		Params:  params,
	}

	if err := conn.WriteJSON(request); err != nil {
		return false, fmt.Errorf("failed to send subscription request: %v", err)
	}

	// Read response
	var response SubscriptionResponse
	if err := conn.ReadJSON(&response); err != nil {
		return false, fmt.Errorf("failed to read subscription response: %v", err)
	}

	// Check if subscription was successful
	if response.Error != nil {
		return false, fmt.Errorf("subscription failed: %v", response.Error)
	}

	if response.Result == nil {
		return false, fmt.Errorf("no subscription ID returned")
	}

	// Subscription was successful
	return true, nil
}

// testWebSocketUnsubscribe tests unsubscription functionality
func testWebSocketUnsubscribe(wsURL string) (bool, string, error) {
	// Parse the WebSocket URL
	u, err := url.Parse(wsURL)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse WebSocket URL: %v", err)
	}

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return false, "", fmt.Errorf("failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Set connection timeout
	err = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err != nil {
		return false, "", fmt.Errorf("failed to set read deadline")
	}
	err = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err != nil {
		return false, "", fmt.Errorf("failed to set write deadline")
	}

	// First, create a subscription
	subscribeRequest := SubscriptionRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "eth_subscribe",
		Params:  []interface{}{"newHeads"}, // Use newHeads as test subscription
	}

	if err := conn.WriteJSON(subscribeRequest); err != nil {
		return false, "", fmt.Errorf("failed to send subscription request: %v", err)
	}

	// Read subscription response
	var subscribeResponse SubscriptionResponse
	if err := conn.ReadJSON(&subscribeResponse); err != nil {
		return false, "", fmt.Errorf("failed to read subscription response: %v", err)
	}

	if subscribeResponse.Error != nil {
		return false, "", fmt.Errorf("subscription failed: %v", subscribeResponse.Error)
	}

	subscriptionID, ok := subscribeResponse.Result.(string)
	if !ok {
		return false, "", fmt.Errorf("invalid subscription ID type")
	}

	// Now test unsubscription
	unsubscribeRequest := SubscriptionRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "eth_unsubscribe",
		Params:  []interface{}{subscriptionID},
	}

	if err := conn.WriteJSON(unsubscribeRequest); err != nil {
		return false, subscriptionID, fmt.Errorf("failed to send unsubscribe request: %v", err)
	}

	// Read unsubscribe response
	var unsubscribeResponse SubscriptionResponse
	if err := conn.ReadJSON(&unsubscribeResponse); err != nil {
		return false, subscriptionID, fmt.Errorf("failed to read unsubscribe response: %v", err)
	}

	if unsubscribeResponse.Error != nil {
		return false, subscriptionID, fmt.Errorf("unsubscribe failed: %v", unsubscribeResponse.Error)
	}

	// Check if unsubscribe returned true
	result, ok := unsubscribeResponse.Result.(bool)
	if !ok {
		return false, subscriptionID, fmt.Errorf("invalid unsubscribe result type")
	}

	if !result {
		return false, subscriptionID, fmt.Errorf("unsubscribe returned false")
	}

	return true, subscriptionID, nil
}
