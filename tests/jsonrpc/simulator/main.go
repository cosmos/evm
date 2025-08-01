package main

import (
	_ "embed"
	"encoding/hex"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/contracts"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/report"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/rpc"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

func main() {
	verbose := flag.Bool("v", false, "Enable verbose output")
	outputExcel := flag.Bool("xlsx", false, "Save output as xlsx")
	flag.Parse()

	// Load configuration from conf.yaml
	conf := config.MustLoadConfig("config.yaml")

	rCtx, err := rpc.NewContext(conf)
	if err != nil {
		log.Fatalf("Failed to create context: %v", err)
	}

	rCtx = MustLoadContractInfo(rCtx)

	// Collect json rpc results
	var results []*types.RpcResult

	rpcs := []struct {
		name types.RpcName
		test rpc.CallRPC
	}{
		{rpc.SendRawTransaction, rpc.RpcSendRawTransactionTransferValue},
		{rpc.SendRawTransaction, rpc.RpcSendRawTransactionDeployContract},
		{rpc.SendRawTransaction, rpc.RpcSendRawTransactionTransferERC20},
		{rpc.GetBlockNumber, rpc.RpcGetBlockNumber},
		{rpc.GetGasPrice, rpc.RpcGetGasPrice},
		{rpc.GetMaxPriorityFeePerGas, rpc.RpcGetMaxPriorityFeePerGas},
		{rpc.GetChainId, rpc.RpcGetChainId},
		{rpc.GetBalance, rpc.RpcGetBalance},
		{rpc.GetTransactionCount, rpc.RpcGetTransactionCount},
		{rpc.GetBlockByHash, rpc.RpcGetBlockByHash},
		{rpc.GetBlockByNumber, rpc.RpcGetBlockByNumber},
		{rpc.GetBlockReceipts, rpc.RpcGetBlockReceipts},
		{rpc.GetTransactionByHash, rpc.RpcGetTransactionByHash},
		{rpc.GetTransactionByBlockHashAndIndex, rpc.RpcGetTransactionByBlockHashAndIndex},
		{rpc.GetTransactionByBlockNumberAndIndex, rpc.RpcGetTransactionByBlockNumberAndIndex},
		{rpc.GetTransactionReceipt, rpc.RpcGetTransactionReceipt},
		{rpc.GetTransactionCountByHash, rpc.RpcGetTransactionCountByHash},
		{rpc.GetBlockTransactionCountByHash, rpc.RpcGetBlockTransactionCountByHash},
		{rpc.GetCode, rpc.RpcGetCode},
		{rpc.GetStorageAt, rpc.RpcGetStorageAt},
		{rpc.NewFilter, rpc.RpcNewFilter},
		{rpc.GetFilterLogs, rpc.RpcGetFilterLogs},
		{rpc.NewBlockFilter, rpc.RpcNewBlockFilter},
		{rpc.GetFilterChanges, rpc.RpcGetFilterChanges},
		{rpc.UninstallFilter, rpc.RpcUninstallFilter},
		{rpc.GetLogs, rpc.RpcGetLogs},
		{rpc.EstimateGas, rpc.RpcEstimateGas},
		{rpc.Call, rpc.RPCCall},
	}

	for _, r := range rpcs {
		_, err := r.test(rCtx)
		if err != nil {
			// add error to results
			results = append(results, &types.RpcResult{
				Method: r.name,
				Status: types.Error,
				ErrMsg: err.Error(),
			})
			continue
		}
	}
	results = append(results, rCtx.AlreadyTestedRPCs...)

	report.ReportResults(results, *verbose, *outputExcel)
}

func MustLoadContractInfo(rCtx *rpc.RpcContext) *rpc.RpcContext {
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
	rCtx.ERC20Abi = &parsedABI
	// Read the compiled contract bytecode
	contractBytecode := common.FromHex(hex.EncodeToString(contracts.ContractByteCode))
	rCtx.ERC20ByteCode = contractBytecode

	return rCtx
}
