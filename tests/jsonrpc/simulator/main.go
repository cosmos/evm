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

	// Define test categories and methods
	testCategories := []types.TestCategory{
		{
			Name:        "Web3",
			Description: "Web3 API methods",
			Methods: []types.TestMethod{
				{Name: rpc.Web3ClientVersion, Handler: rpc.RpcWeb3ClientVersion},
				{Name: rpc.Web3Sha3, Handler: rpc.RpcWeb3Sha3},
			},
		},
		{
			Name:        "Net",
			Description: "Network API methods",
			Methods: []types.TestMethod{
				{Name: rpc.NetVersion, Handler: rpc.RpcNetVersion},
				{Name: rpc.NetPeerCount, Handler: rpc.RpcNetPeerCount},
				{Name: rpc.NetListening, Handler: rpc.RpcNetListening},
			},
		},
		{
			Name:        "Core Eth",
			Description: "Core Ethereum API methods",
			Methods: []types.TestMethod{
				{Name: rpc.EthProtocolVersion, Handler: rpc.RpcEthProtocolVersion},
				{Name: rpc.EthSyncing, Handler: rpc.RpcEthSyncing},
				{Name: rpc.GetGasPrice, Handler: rpc.RpcGetGasPrice},
				{Name: rpc.EthAccounts, Handler: rpc.RpcEthAccounts},
				{Name: rpc.GetBlockNumber, Handler: rpc.RpcGetBlockNumber},
				{Name: rpc.GetChainId, Handler: rpc.RpcGetChainId},
			},
		},
		{
			Name:        "Account & State",
			Description: "Account and state related methods",
			Methods: []types.TestMethod{
				{Name: rpc.GetBalance, Handler: rpc.RpcGetBalance},
				{Name: rpc.GetTransactionCount, Handler: rpc.RpcGetTransactionCount},
				{Name: rpc.GetCode, Handler: rpc.RpcGetCode},
				{Name: rpc.GetStorageAt, Handler: rpc.RpcGetStorageAt},
			},
		},
		{
			Name:        "Block",
			Description: "Block related methods",
			Methods: []types.TestMethod{
				{Name: rpc.GetBlockByNumber, Handler: rpc.RpcGetBlockByNumber},
				{Name: rpc.GetBlockByHash, Handler: rpc.RpcGetBlockByHash},
				{Name: rpc.GetBlockTransactionCountByHash, Handler: rpc.RpcGetBlockTransactionCountByHash},
				{Name: rpc.GetBlockReceipts, Handler: rpc.RpcGetBlockReceipts},
			},
		},
		{
			Name:        "Transaction",
			Description: "Transaction related methods",
			Methods: []types.TestMethod{
				{Name: rpc.EstimateGas, Handler: rpc.RpcEstimateGas},
				{Name: rpc.Call, Handler: rpc.RPCCall},
				{Name: rpc.SendRawTransaction, Handler: rpc.RpcSendRawTransactionTransferValue, Description: "Transfer value"},
				{Name: rpc.SendRawTransaction, Handler: rpc.RpcSendRawTransactionDeployContract, Description: "Deploy contract"},
				{Name: rpc.SendRawTransaction, Handler: rpc.RpcSendRawTransactionTransferERC20, Description: "Transfer ERC20"},
				{Name: rpc.GetTransactionByHash, Handler: rpc.RpcGetTransactionByHash},
				{Name: rpc.GetTransactionByBlockHashAndIndex, Handler: rpc.RpcGetTransactionByBlockHashAndIndex},
				{Name: rpc.GetTransactionByBlockNumberAndIndex, Handler: rpc.RpcGetTransactionByBlockNumberAndIndex},
				{Name: rpc.GetTransactionReceipt, Handler: rpc.RpcGetTransactionReceipt},
				{Name: rpc.GetTransactionCountByHash, Handler: rpc.RpcGetTransactionCountByHash},
			},
		},
		{
			Name:        "Filter",
			Description: "Filter and logs methods",
			Methods: []types.TestMethod{
				{Name: rpc.NewFilter, Handler: rpc.RpcNewFilter},
				{Name: rpc.GetFilterLogs, Handler: rpc.RpcGetFilterLogs},
				{Name: rpc.NewBlockFilter, Handler: rpc.RpcNewBlockFilter},
				{Name: rpc.GetFilterChanges, Handler: rpc.RpcGetFilterChanges},
				{Name: rpc.UninstallFilter, Handler: rpc.RpcUninstallFilter},
				{Name: rpc.GetLogs, Handler: rpc.RpcGetLogs},
			},
		},
		{
			Name:        "Mining",
			Description: "Mining related methods",
			Methods: []types.TestMethod{
				{Name: rpc.EthMining, Handler: rpc.RpcEthMining},
			},
		},
		{
			Name:        "EIP-1559",
			Description: "EIP-1559 fee methods",
			Methods: []types.TestMethod{
				{Name: rpc.GetMaxPriorityFeePerGas, Handler: rpc.RpcGetMaxPriorityFeePerGas},
			},
		},
		{
			Name:        "Personal",
			Description: "Personal API methods",
			Methods: []types.TestMethod{
				{Name: rpc.PersonalListAccounts, Handler: rpc.RpcPersonalListAccounts},
			},
		},
		{
			Name:        "TxPool",
			Description: "Transaction pool methods",
			Methods: []types.TestMethod{
				{Name: rpc.TxPoolStatus, Handler: rpc.RpcTxPoolStatus},
			},
		},
		{
			Name:        "Not Implemented",
			Description: "Methods expected to be not implemented",
			Methods: []types.TestMethod{
				{Name: rpc.EngineNewPayloadV1, Handler: nil, SkipReason: "Expected to be not implemented"},
				{Name: rpc.EngineForkchoiceUpdatedV1, Handler: nil, SkipReason: "Expected to be not implemented"},
				{Name: rpc.EngineGetPayloadV1, Handler: nil, SkipReason: "Expected to be not implemented"},
				{Name: rpc.EthCreateAccessList, Handler: nil, SkipReason: "Expected to be not implemented"},
				{Name: rpc.TraceCall, Handler: nil, SkipReason: "Expected to be not implemented"},
				{Name: rpc.TraceCallMany, Handler: nil, SkipReason: "Expected to be not implemented"},
				{Name: rpc.TraceTransaction, Handler: nil, SkipReason: "Expected to be not implemented"},
				{Name: rpc.AdminAddPeer, Handler: nil, SkipReason: "Expected to be not implemented"},
				{Name: rpc.AdminNodeInfo, Handler: nil, SkipReason: "Expected to be not implemented"},
			},
		},
	}

	// Execute tests by category
	for _, category := range testCategories {
		for _, method := range category.Methods {
			if method.Handler == nil {
				// Handle not implemented methods
				result, _ := rpc.RpcNotImplemented(method.Name, category.Name)
				results = append(results, result)
				continue
			}

			// Execute the test
			handler := method.Handler.(func(*rpc.RpcContext) (*types.RpcResult, error))
			result, err := handler(rCtx)
			if err != nil {
				result = &types.RpcResult{
					Method:   method.Name,
					Status:   types.Error,
					ErrMsg:   err.Error(),
					Category: category.Name,
				}
			}
			// Ensure category is set
			if result.Category == "" {
				result.Category = category.Name
			}
			results = append(results, result)
		}
	}

	// Add results from transaction tests that were automatically added
	for _, result := range rCtx.AlreadyTestedRPCs {
		if result.Category == "" {
			// Categorize based on method name
			switch result.Method {
			case rpc.GetTransactionReceipt:
				result.Category = "Transaction"
			case rpc.GetBlockByNumber, rpc.GetBlockByHash:
				result.Category = "Block"
			default:
				result.Category = "Uncategorized"
			}
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
