package rpc

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/status-im/keycard-go/hexutils"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/contracts"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/utils"
)

func EthBlockNumber(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthBlockNumber); result != nil {
		return result, nil
	}
	blockNumber, err := rCtx.EthCli.BlockNumber(context.Background())
	if err != nil {
		return nil, err
	}

	// Block number 0 is valid for fresh chains
	status := types.Ok

	result := &types.RpcResult{
		Method:   MethodNameEthBlockNumber,
		Status:   status,
		Value:    blockNumber,
		Category: "eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGasPrice(rCtx *RpcContext) (*types.RpcResult, error) {
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

	result := &types.RpcResult{
		Method:   MethodNameEthGasPrice,
		Status:   status,
		Value:    gasPrice.String(),
		Category: "eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthMaxPriorityFeePerGas(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthMaxPriorityFeePerGas); result != nil {
		return result, nil
	}

	maxPriorityFeePerGas, err := rCtx.EthCli.SuggestGasTipCap(context.Background())
	if err != nil {
		return nil, err
	}

	// Zero maxPriorityFeePerGas is valid (legacy transactions don't use tips)
	status := types.Ok

	result := &types.RpcResult{
		Method:   MethodNameEthMaxPriorityFeePerGas,
		Status:   status,
		Value:    maxPriorityFeePerGas.String(),
		Category: "eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthChainId(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthChainId); result != nil {
		return result, nil
	}

	chainId, err := rCtx.EthCli.ChainID(context.Background())
	if err != nil {
		return nil, err
	}

	// chainId should never be zero
	if chainId.Cmp(big.NewInt(0)) == 0 {
		return nil, fmt.Errorf("chainId is zero")
	}

	status := types.Ok

	result := &types.RpcResult{
		Method:   MethodNameEthChainId,
		Status:   status,
		Value:    chainId.String(),
		Category: "eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetBalance(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetBalance); result != nil {
		return result, nil
	}

	balance, err := rCtx.EthCli.BalanceAt(context.Background(), rCtx.Acc.Address, nil)
	if err != nil {
		return nil, err
	}

	// Zero balance is valid for unused addresses
	status := types.Ok

	result := &types.RpcResult{
		Method:   MethodNameEthGetBalance,
		Status:   status,
		Value:    balance.String(),
		Category: "eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetTransactionCount(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetTransactionCount); result != nil {
		return result, nil
	}

	nonce, err := rCtx.EthCli.PendingNonceAt(context.Background(), rCtx.Acc.Address)
	if err != nil {
		return nil, err
	}

	// Zero nonce is valid for unused addresses
	status := types.Ok

	return &types.RpcResult{
		Method:   MethodNameEthGetTransactionCount,
		Status:   status,
		Value:    nonce,
		Category: "eth",
	}, nil
}

func EthGetBlockByHash(rCtx *RpcContext) (*types.RpcResult, error) {
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

	result := &types.RpcResult{
		Method: MethodNameEthGetBlockByHash,
		Status: types.Ok,
		Value:  utils.MustBeautifyBlock(types.NewRpcBlock(block)),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetBlockByNumber(rCtx *RpcContext) (*types.RpcResult, error) {
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

	result := &types.RpcResult{
		Method: MethodNameEthGetBlockByNumber,
		Status: types.Ok,
		Value:  utils.MustBeautifyBlock(types.NewRpcBlock(blk)),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthSendRawTransactionTransferValue(rCtx *RpcContext) (*types.RpcResult, error) {
	// testedRPCs is a slice of RpcResult that will be appended to rCtx.AlreadyTestedRPCs
	// if the transaction is successfully sent
	var testedRPCs []*types.RpcResult
	var err error
	// Create a new transaction
	if rCtx.ChainId, err = rCtx.EthCli.ChainID(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthChainId,
		Status: types.Ok,
		Value:  rCtx.ChainId.String(),
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
		ChainID:   rCtx.ChainId,
		Nonce:     nonce,
		GasTipCap: rCtx.MaxPriorityFeePerGas,
		GasFeeCap: new(big.Int).Add(rCtx.GasPrice, big.NewInt(1000000000)),
		Gas:       21000, // fixed gas limit for transfer
		To:        &randomRecipient,
		Value:     value,
	})

	// TODO: Make signer using types.MakeSigner with chain params
	signer := gethtypes.NewLondonSigner(rCtx.ChainId)
	signedTx, err := gethtypes.SignTx(tx, signer, rCtx.Acc.PrivKey)
	if err != nil {
		return nil, err
	}

	if err = rCtx.EthCli.SendTransaction(context.Background(), signedTx); err != nil {
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
	if err = WaitForTx(rCtx, signedTx.Hash(), tout); err != nil {
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

func EthSendRawTransactionDeployContract(rCtx *RpcContext) (*types.RpcResult, error) {
	// testedRPCs is a slice of RpcResult that will be appended to rCtx.AlreadyTestedRPCs
	// if the transaction is successfully sent
	var testedRPCs []*types.RpcResult
	var err error
	// Create a new transaction
	if rCtx.ChainId, err = rCtx.EthCli.ChainID(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthChainId,
		Status: types.Ok,
		Value:  rCtx.ChainId.String(),
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
		ChainID:   rCtx.ChainId,
		Nonce:     nonce,
		GasTipCap: rCtx.MaxPriorityFeePerGas,
		GasFeeCap: new(big.Int).Add(rCtx.GasPrice, big.NewInt(1000000000)),
		Gas:       10000000,
		Data:      common.FromHex(string(contracts.ContractByteCode)),
	})

	// TODO: Make signer using types.MakeSigner with chain params
	signer := gethtypes.NewLondonSigner(rCtx.ChainId)
	signedTx, err := gethtypes.SignTx(tx, signer, rCtx.Acc.PrivKey)
	if err != nil {
		return nil, err
	}

	if err = rCtx.EthCli.SendTransaction(context.Background(), signedTx); err != nil {
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
	if err = WaitForTx(rCtx, signedTx.Hash(), tout); err != nil {
		return nil, err
	}

	if rCtx.ERC20Addr == (common.Address{}) {
		return nil, errors.New("contract address is empty, failed to deploy")
	}

	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, testedRPCs...)

	return result, nil
}

// EthSendRawTransaction unified test that combines all scenarios
func EthSendRawTransaction(rCtx *RpcContext) (*types.RpcResult, error) {
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
		Category:    "eth",
	}, nil
}

func EthSendRawTransactionTransferERC20(rCtx *RpcContext) (*types.RpcResult, error) {
	// testedRPCs is a slice of RpcResult that will be appended to rCtx.AlreadyTestedRPCs
	// if the transaction is successfully sent
	var testedRPCs []*types.RpcResult
	var err error
	// Create a new transaction
	if rCtx.ChainId, err = rCtx.EthCli.ChainID(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: MethodNameEthChainId,
		Status: types.Ok,
		Value:  rCtx.ChainId.String(),
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
		ChainID:   rCtx.ChainId,
		Nonce:     nonce,
		GasTipCap: rCtx.MaxPriorityFeePerGas,
		GasFeeCap: new(big.Int).Add(rCtx.GasPrice, big.NewInt(1000000000)),
		Gas:       10000000,
		To:        &rCtx.ERC20Addr,
		Data:      data,
	})

	// TODO: Make signer using types.MakeSigner with chain params
	signer := gethtypes.NewLondonSigner(rCtx.ChainId)
	signedTx, err := gethtypes.SignTx(tx, signer, rCtx.Acc.PrivKey)
	if err != nil {
		return nil, err
	}

	if err = rCtx.EthCli.SendTransaction(context.Background(), signedTx); err != nil {
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
	if err = WaitForTx(rCtx, signedTx.Hash(), tout); err != nil {
		return nil, err
	}

	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, testedRPCs...)

	return result, nil
}

func EthGetBlockReceipts(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetBlockReceipts); result != nil {
		return result, nil
	}

	if len(rCtx.BlockNumsIncludingTx) == 0 {
		return nil, errors.New("no blocks with transactions")

	}

	// TODO: Random pick
	// pick a block with transactions
	blkNum := rCtx.BlockNumsIncludingTx[0]
	rpcBlockNum := rpc.BlockNumber(blkNum)
	receipts, err := rCtx.EthCli.BlockReceipts(context.Background(), rpc.BlockNumberOrHash{BlockNumber: &rpcBlockNum})
	if err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: MethodNameEthGetBlockReceipts,
		Status: types.Ok,
		Value:  utils.MustBeautifyReceipts(receipts),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetTransactionByHash(rCtx *RpcContext) (*types.RpcResult, error) {
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

	result := &types.RpcResult{
		Method: MethodNameEthGetTransactionByHash,
		Status: types.Ok,
		Value:  utils.MustBeautifyTransaction(tx),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetTransactionByBlockHashAndIndex(rCtx *RpcContext) (*types.RpcResult, error) {
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

	result := &types.RpcResult{
		Method: MethodNameEthGetTransactionByBlockHashAndIndex,
		Status: types.Ok,
		Value:  utils.MustBeautifyTransaction(tx),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetTransactionByBlockNumberAndIndex(rCtx *RpcContext) (*types.RpcResult, error) {
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

	result := &types.RpcResult{
		Method: MethodNameEthGetTransactionByBlockNumberAndIndex,
		Status: types.Ok,
		Value:  utils.MustBeautifyTransaction(&tx),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetBlockTransactionCountByNumber(rCtx *RpcContext) (*types.RpcResult, error) {
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
	block, err := rCtx.EthCli.BlockByNumber(context.Background(), big.NewInt(int64(targetBlockNum)))
	if err != nil {
		return nil, fmt.Errorf("failed to get block %d: %w", targetBlockNum, err)
	}

	// Get transaction count for the block
	count := uint64(len(block.Transactions()))

	result := &types.RpcResult{
		Method:   MethodNameEthGetBlockTransactionCountByNumber,
		Status:   types.Ok,
		Value:    count,
		Category: "eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

// Uncle methods - these should always return 0 or nil in Cosmos EVM (no uncles in PoS)
func EthGetUncleCountByBlockHash(rCtx *RpcContext) (*types.RpcResult, error) {
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

	// Should always be 0 in Cosmos EVM
	result := &types.RpcResult{
		Method:   MethodNameEthGetUncleCountByBlockHash,
		Status:   types.Ok,
		Value:    uncleCount,
		Category: "eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetUncleCountByBlockNumber(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetUncleCountByBlockNumber); result != nil {
		return result, nil
	}

	var uncleCount string
	err := rCtx.EthCli.Client().CallContext(context.Background(), &uncleCount, string(MethodNameEthGetUncleCountByBlockNumber), "latest")
	if err != nil {
		return nil, fmt.Errorf("eth_getUncleCountByBlockNumber call failed: %w", err)
	}

	// Should always be 0 in Cosmos EVM
	result := &types.RpcResult{
		Method:   MethodNameEthGetUncleCountByBlockNumber,
		Status:   types.Ok,
		Value:    uncleCount,
		Category: "eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetUncleByBlockHashAndIndex(rCtx *RpcContext) (*types.RpcResult, error) {
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

	// Should always be nil in Cosmos EVM
	result := &types.RpcResult{
		Method:   MethodNameEthGetUncleByBlockHashAndIndex,
		Status:   types.Ok,
		Value:    uncle,
		Category: "eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetUncleByBlockNumberAndIndex(rCtx *RpcContext) (*types.RpcResult, error) {
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

	// Should always be nil in Cosmos EVM
	result := &types.RpcResult{
		Method:   MethodNameEthGetUncleByBlockNumberAndIndex,
		Status:   types.Ok,
		Value:    uncle,
		Category: "eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetTransactionCountByHash(rCtx *RpcContext) (*types.RpcResult, error) {
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

	result := &types.RpcResult{
		Method: MethodNameEthGetTransactionCountByHash,
		Status: types.Ok,
		Value:  count,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetTransactionReceipt(rCtx *RpcContext) (*types.RpcResult, error) {
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

	result := &types.RpcResult{
		Method: MethodNameEthGetTransactionReceipt,
		Status: types.Ok,
		Value:  utils.MustBeautifyReceipt(receipt),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetBlockTransactionCountByHash(rCtx *RpcContext) (*types.RpcResult, error) {
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

	result := &types.RpcResult{
		Method: MethodNameEthGetBlockTransactionCountByHash,
		Status: types.Ok,
		Value:  count,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetCode(rCtx *RpcContext) (*types.RpcResult, error) {
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

	result := &types.RpcResult{
		Method: MethodNameEthGetCode,
		Status: types.Ok,
		Value:  hexutils.BytesToHex(code),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetStorageAt(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthGetStorageAt); result != nil {
		return result, nil
	}

	if rCtx.ERC20Addr == (common.Address{}) {
		return nil, errors.New("no contract address, must be deployed first")
	}

	key := utils.MustCalculateSlotKey(rCtx.Acc.Address, 4)
	storage, err := rCtx.EthCli.StorageAt(context.Background(), rCtx.ERC20Addr, key, nil)
	if err != nil {
		return nil, err
	}

	// Zero storage is valid - most storage slots are empty
	status := types.Ok

	result := &types.RpcResult{
		Method:   MethodNameEthGetStorageAt,
		Status:   status,
		Value:    hexutils.BytesToHex(storage),
		Category: "eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthNewFilter(rCtx *RpcContext) (*types.RpcResult, error) {
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
	var rpcId string
	if err = rCtx.EthCli.Client().CallContext(context.Background(), &rpcId, string(MethodNameEthNewFilter), args); err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: MethodNameEthNewFilter,
		Status: types.Ok,
		Value:  rpcId,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	rCtx.FilterId = rpcId
	rCtx.FilterQuery = fErc20Transfer

	return result, nil
}

func EthGetFilterLogs(rCtx *RpcContext) (*types.RpcResult, error) {
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

	result := &types.RpcResult{
		Method: MethodNameEthGetFilterLogs,
		Status: types.Ok,
		Value:  utils.MustBeautifyLogs(logs),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthNewBlockFilter(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthNewBlockFilter); result != nil {
		return result, nil
	}

	var rpcId string
	if err := rCtx.EthCli.Client().CallContext(context.Background(), &rpcId, string(MethodNameEthNewBlockFilter)); err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: MethodNameEthNewBlockFilter,
		Status: types.Ok,
		Value:  rpcId,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	rCtx.BlockFilterId = rpcId

	return result, nil
}

func EthGetFilterChanges(rCtx *RpcContext) (*types.RpcResult, error) {
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

	status := types.Ok
	// Empty results are valid - no warnings needed

	result := &types.RpcResult{
		Method:   MethodNameEthGetFilterChanges,
		Status:   status,
		Value:    changes,
		Category: "eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthUninstallFilter(rCtx *RpcContext) (*types.RpcResult, error) {
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

	result := &types.RpcResult{
		Method: MethodNameEthUninstallFilter,
		Status: types.Ok,
		Value:  rCtx.FilterId,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func EthGetLogs(rCtx *RpcContext) (*types.RpcResult, error) {
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

	status := types.Ok
	// Empty results are valid - no warnings needed

	result := &types.RpcResult{
		Method:   MethodNameEthGetLogs,
		Status:   status,
		Value:    utils.MustBeautifyLogs(logs),
		Category: "eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

// Additional Eth method handlers
func EthProtocolVersion(rCtx *RpcContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.EthCli.Client().Call(&result, "eth_protocolVersion")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthProtocolVersion,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "eth",
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameEthProtocolVersion,
		Status:   types.Ok,
		Value:    result,
		Category: "eth",
	}, nil
}

func EthSyncing(rCtx *RpcContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, "eth_syncing")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthSyncing,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "eth",
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameEthSyncing,
		Status:   types.Ok,
		Value:    result,
		Category: "eth",
	}, nil
}

func EthAccounts(rCtx *RpcContext) (*types.RpcResult, error) {
	var result []string
	err := rCtx.EthCli.Client().Call(&result, "eth_accounts")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthAccounts,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "eth",
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameEthAccounts,
		Status:   types.Ok,
		Value:    result,
		Category: "eth",
	}, nil
}

// Mining method handlers
func EthMining(rCtx *RpcContext) (*types.RpcResult, error) {
	var result bool
	err := rCtx.EthCli.Client().Call(&result, "eth_mining")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthMining,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "eth",
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameEthMining,
		Status:   types.Ok,
		Value:    result,
		Category: "eth",
	}, nil
}

func EthCall(rCtx *RpcContext) (*types.RpcResult, error) {
	// Simple eth_call test
	callMsg := ethereum.CallMsg{
		To:   &rCtx.Acc.Address,
		Data: []byte{},
	}

	result, err := rCtx.EthCli.CallContract(context.Background(), callMsg, nil)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameEthCall,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "eth",
		}, nil
	}

	return &types.RpcResult{
		Method:   MethodNameEthCall,
		Status:   types.Ok,
		Value:    "0x" + hex.EncodeToString(result),
		Category: "eth",
	}, nil
}

func EthEstimateGas(rCtx *RpcContext) (*types.RpcResult, error) {
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
			Category: "eth",
		}, nil
	}

	return &types.RpcResult{
		Method:   MethodNameEthEstimateGas,
		Status:   types.Ok,
		Value:    fmt.Sprintf("0x%x", gasLimit),
		Category: "eth",
	}, nil
}

func EthFeeHistory(rCtx *RpcContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, string(MethodNameEthFeeHistory), "0x2", "latest", []float64{25.0, 50.0, 75.0})

	if err != nil {
		if err.Error() == "the method "+string(MethodNameEthFeeHistory)+" does not exist/is not available" ||
			err.Error() == "Method not found" {
			return &types.RpcResult{
				Method:   MethodNameEthFeeHistory,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: "eth",
			}, nil
		}
		return &types.RpcResult{
			Method:   MethodNameEthFeeHistory,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "eth",
		}, nil
	}

	return &types.RpcResult{
		Method:   MethodNameEthFeeHistory,
		Status:   types.Ok,
		Value:    result,
		Category: "eth",
	}, nil
}

func EthBlobBaseFee(rCtx *RpcContext) (*types.RpcResult, error) {
	return GenericTest(rCtx, MethodNameEthBlobBaseFee, "eth")
}

func EthGetProof(rCtx *RpcContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, string(MethodNameEthGetProof), rCtx.Acc.Address.Hex(), []string{}, "latest")

	if err != nil {
		if err.Error() == "the method "+string(MethodNameEthGetProof)+" does not exist/is not available" ||
			err.Error() == "Method not found" {
			return &types.RpcResult{
				Method:   MethodNameEthGetProof,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: "eth",
			}, nil
		}
		return &types.RpcResult{
			Method:   MethodNameEthGetProof,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "eth",
		}, nil
	}

	return &types.RpcResult{
		Method:   MethodNameEthGetProof,
		Status:   types.Ok,
		Value:    result,
		Category: "eth",
	}, nil
}

// EthSendTransaction sends a transaction using eth_sendTransaction
// This requires the account to be unlocked or managed by the node
func EthSendTransaction(rCtx *RpcContext) (*types.RpcResult, error) {
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
		// Check for expected "key not found" errors which indicate the API is working
		// but the account is not managed by the node (which is normal/secure)
		if strings.Contains(err.Error(), "key not found") ||
			strings.Contains(err.Error(), "failed to find key in the node's keyring") ||
			strings.Contains(err.Error(), "account not unlocked") {
			return &types.RpcResult{
				Method:   MethodNameEthSendTransaction,
				Status:   types.Ok, // API works correctly, just account not available
				Value:    fmt.Sprintf("API functional - expected key management error: %s", err.Error()),
				Category: "eth",
			}, nil
		}
		// Other errors indicate actual API problems
		return &types.RpcResult{
			Method:   MethodNameEthSendTransaction,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "eth",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameEthSendTransaction,
		Status:   types.Ok,
		Value:    txHash,
		Category: "eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

// EthSign signs data using eth_sign
// This requires the account to be unlocked or managed by the node
func EthSign(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameEthSign); result != nil {
		return result, nil
	}

	// Test data to sign (32-byte hash)
	testData := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	var signature string
	err := rCtx.EthCli.Client().Call(&signature, string(MethodNameEthSign), rCtx.Acc.Address.Hex(), testData)
	if err != nil {
		// Check for expected "key not found" errors which indicate the API is working
		// but the account is not managed by the node (which is normal/secure)
		if strings.Contains(err.Error(), "key not found") ||
			strings.Contains(err.Error(), "failed to find key in the node's keyring") ||
			strings.Contains(err.Error(), "account not unlocked") ||
			strings.Contains(err.Error(), "eth_sign is disabled") {
			return &types.RpcResult{
				Method:   MethodNameEthSign,
				Status:   types.Ok, // API works correctly, just account not available or disabled
				Value:    fmt.Sprintf("API functional - expected key management/security error: %s", err.Error()),
				Category: "eth",
			}, nil
		}
		// Other errors indicate actual API problems
		return &types.RpcResult{
			Method:   MethodNameEthSign,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "eth",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameEthSign,
		Status:   types.Ok,
		Value:    signature,
		Category: "eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

func EthCreateAccessList(rCtx *RpcContext) (*types.RpcResult, error) {
	var result interface{}
	callData := map[string]interface{}{
		"to":   rCtx.Acc.Address.Hex(),
		"data": "0x",
	}
	err := rCtx.EthCli.Client().Call(&result, string(MethodNameEthCreateAccessList), callData, "latest")

	if err != nil {
		if err.Error() == "the method "+string(MethodNameEthCreateAccessList)+" does not exist/is not available" ||
			err.Error() == "Method not found" {
			return &types.RpcResult{
				Method:   MethodNameEthCreateAccessList,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: "eth",
			}, nil
		}
		return &types.RpcResult{
			Method:   MethodNameEthCreateAccessList,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "eth",
		}, nil
	}

	return &types.RpcResult{
		Method:   MethodNameEthCreateAccessList,
		Status:   types.Ok,
		Value:    result,
		Category: "eth",
	}, nil
}
