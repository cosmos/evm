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

	// Define test categories by namespace based on execution-apis structure
	testCategories := []types.TestCategory{
		{
			Name:        "web3",
			Description: "Web3 namespace utility methods",
			Methods: []types.TestMethod{
				{Name: rpc.Web3ClientVersion, Handler: rpc.RpcWeb3ClientVersion},
				{Name: rpc.Web3Sha3, Handler: rpc.RpcWeb3Sha3},
			},
		},
		{
			Name:        "net",
			Description: "Net namespace network methods",
			Methods: []types.TestMethod{
				{Name: rpc.NetVersion, Handler: rpc.RpcNetVersion},
				{Name: rpc.NetPeerCount, Handler: rpc.RpcNetPeerCount},
				{Name: rpc.NetListening, Handler: rpc.RpcNetListening},
			},
		},
		{
			Name:        "eth",
			Description: "Ethereum namespace methods from execution-apis",
			Methods: []types.TestMethod{
				// Client subcategory
				{Name: rpc.EthChainId, Handler: rpc.RpcEthChainId},
				{Name: rpc.EthSyncing, Handler: rpc.RpcEthSyncing},
				{Name: rpc.EthCoinbase, Handler: rpc.RpcEthCoinbase},
				{Name: rpc.EthAccounts, Handler: rpc.RpcEthAccounts},
				{Name: rpc.EthBlockNumber, Handler: rpc.RpcEthBlockNumber},
				{Name: rpc.EthMining, Handler: rpc.RpcEthMining},
				// Fee market subcategory
				{Name: rpc.EthGasPrice, Handler: rpc.RpcEthGasPrice},
				{Name: rpc.EthMaxPriorityFeePerGas, Handler: rpc.RpcEthMaxPriorityFeePerGas},
				// State subcategory
				{Name: rpc.EthGetBalance, Handler: rpc.RpcEthGetBalance},
				{Name: rpc.EthGetTransactionCount, Handler: rpc.RpcEthGetTransactionCount},
				{Name: rpc.EthGetCode, Handler: rpc.RpcEthGetCode},
				{Name: rpc.EthGetStorageAt, Handler: rpc.RpcEthGetStorageAt},
				// Block subcategory
				{Name: rpc.EthGetBlockByHash, Handler: rpc.RpcEthGetBlockByHash},
				{Name: rpc.EthGetBlockByNumber, Handler: rpc.RpcEthGetBlockByNumber},
				{Name: rpc.EthGetBlockTransactionCountByHash, Handler: rpc.RpcEthGetBlockTransactionCountByHash},
				{Name: rpc.EthGetBlockReceipts, Handler: rpc.RpcEthGetBlockReceipts},
				// Transaction subcategory
				{Name: rpc.EthGetTransactionByHash, Handler: rpc.RpcEthGetTransactionByHash},
				{Name: rpc.EthGetTransactionByBlockHashAndIndex, Handler: rpc.RpcEthGetTransactionByBlockHashAndIndex},
				{Name: rpc.EthGetTransactionByBlockNumberAndIndex, Handler: rpc.RpcEthGetTransactionByBlockNumberAndIndex},
				{Name: rpc.EthGetTransactionReceipt, Handler: rpc.RpcEthGetTransactionReceipt},
				{Name: rpc.EthGetTransactionCountByHash, Handler: rpc.RpcEthGetTransactionCountByHash},
				// Execute subcategory
				{Name: rpc.Call, Handler: rpc.RpcEthCall},
				{Name: rpc.EstimateGas, Handler: rpc.RpcEthEstimateGas},
				// Submit subcategory
				{Name: rpc.EthSendRawTransaction, Handler: rpc.RpcEthSendRawTransactionTransferValue, Description: "Transfer value"},
				{Name: rpc.EthSendRawTransaction, Handler: rpc.RpcEthSendRawTransactionDeployContract, Description: "Deploy contract"},
				{Name: rpc.EthSendRawTransaction, Handler: rpc.RpcEthSendRawTransactionTransferERC20, Description: "Transfer ERC20"},
				// Filter subcategory
				{Name: rpc.EthNewFilter, Handler: rpc.RpcEthNewFilter},
				{Name: rpc.EthGetFilterLogs, Handler: rpc.RpcEthGetFilterLogs},
				{Name: rpc.EthNewBlockFilter, Handler: rpc.RpcEthNewBlockFilter},
				{Name: rpc.EthGetFilterChanges, Handler: rpc.RpcEthGetFilterChanges},
				{Name: rpc.EthUninstallFilter, Handler: rpc.RpcEthUninstallFilter},
				{Name: rpc.EthGetLogs, Handler: rpc.RpcEthGetLogs},
				// Other/not implemented methods
				{Name: rpc.EthBlobBaseFee, Handler: nil, SkipReason: "EIP-4844 blob base fee (post-Cancun)"},
				{Name: rpc.EthFeeHistory, Handler: nil, SkipReason: "Fee history not implemented"},
				{Name: rpc.EthGetProof, Handler: nil, SkipReason: "State proof not implemented"},
				{Name: rpc.EthProtocolVersion, Handler: nil, SkipReason: "Protocol version deprecated"},
				{Name: rpc.EthCreateAccessList, Handler: nil, SkipReason: "Access list creation not implemented"},
			},
		},
		{
			Name:        "personal",
			Description: "Personal namespace methods (deprecated)",
			Methods: []types.TestMethod{
				{Name: rpc.PersonalListAccounts, Handler: rpc.RpcPersonalListAccounts},
				{Name: rpc.PersonalEcRecover, Handler: rpc.RpcPersonalEcRecover},
				{Name: rpc.PersonalListWallets, Handler: rpc.RpcPersonalListWallets},
				{Name: rpc.PersonalNewAccount, Handler: nil, SkipReason: "Requires password, modifies state"},
				{Name: rpc.PersonalImportRawKey, Handler: nil, SkipReason: "Requires key and password, modifies state"},
				{Name: rpc.PersonalUnlockAccount, Handler: nil, SkipReason: "Requires password"},
				{Name: rpc.PersonalLockAccount, Handler: nil, SkipReason: "Requires unlocked account"},
				{Name: rpc.PersonalSign, Handler: nil, SkipReason: "Requires unlocked account"},
				{Name: rpc.PersonalSignTypedData, Handler: nil, SkipReason: "Requires specific typed data format"},
			},
		},
		{
			Name:        "miner", 
			Description: "Miner namespace methods (deprecated)",
			Methods: []types.TestMethod{
				{Name: rpc.MinerStart, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: rpc.MinerStop, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: rpc.MinerSetEtherbase, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: rpc.MinerSetExtra, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: rpc.MinerSetGasPrice, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: rpc.MinerSetGasLimit, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: rpc.MinerGetHashrate, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
			},
		},
		{
			Name:        "txpool",
			Description: "TxPool namespace methods",
			Methods: []types.TestMethod{
				{Name: rpc.TxPoolContent, Handler: rpc.RpcTxPoolContent},
				{Name: rpc.TxPoolInspect, Handler: rpc.RpcTxPoolInspect},
				{Name: rpc.TxPoolStatus, Handler: rpc.RpcTxPoolStatus},
			},
		},
		{
			Name:        "debug",
			Description: "Debug namespace methods from Geth",
			Methods: []types.TestMethod{
				// Tracing subcategory
				{Name: rpc.DebugTraceTransaction, Handler: nil, SkipReason: "Complex tracing method"},
				{Name: rpc.DebugTraceBlock, Handler: nil, SkipReason: "Complex tracing method"},
				{Name: rpc.DebugTraceCall, Handler: nil, SkipReason: "Complex tracing method"},
				// Database subcategory  
				{Name: rpc.DebugDbGet, Handler: nil, SkipReason: "Database access method"},
				{Name: rpc.DebugDbAncient, Handler: nil, SkipReason: "Ancient database access"},
				{Name: rpc.DebugChaindbCompact, Handler: nil, SkipReason: "Database compaction"},
				// Diagnostics subcategory
				{Name: rpc.DebugGetBadBlocks, Handler: nil, SkipReason: "Diagnostic method"},
				{Name: rpc.DebugFreeOSMemory, Handler: nil, SkipReason: "System management method"},
				{Name: rpc.DebugStacks, Handler: nil, SkipReason: "Stack inspection method"},
				// Profiling subcategory
				{Name: rpc.DebugBlockProfile, Handler: nil, SkipReason: "Profiling method"},
				{Name: rpc.DebugCpuProfile, Handler: nil, SkipReason: "CPU profiling method"},
				{Name: rpc.DebugMemStats, Handler: nil, SkipReason: "Memory statistics method"},
			},
		},
		{
			Name:        "engine",
			Description: "Engine API methods (not applicable for Cosmos chains)",
			Methods: []types.TestMethod{
				{Name: rpc.EngineNewPayloadV1, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: rpc.EngineForkchoiceUpdatedV1, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: rpc.EngineGetPayloadV1, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: rpc.EngineNewPayloadV2, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: rpc.EngineForkchoiceUpdatedV2, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: rpc.EngineGetPayloadV2, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
			},
		},
		{
			Name:        "trace",
			Description: "Trace namespace methods (OpenEthereum/Erigon specific)",
			Methods: []types.TestMethod{
				{Name: rpc.TraceCall, Handler: nil, SkipReason: "Trace methods not implemented in standard Geth"},
				{Name: rpc.TraceCallMany, Handler: nil, SkipReason: "Trace methods not implemented in standard Geth"},
				{Name: rpc.TraceTransaction, Handler: nil, SkipReason: "Trace methods not implemented in standard Geth"},
				{Name: rpc.TraceBlock, Handler: nil, SkipReason: "Trace methods not implemented in standard Geth"},
			},
		},
		{
			Name:        "admin",
			Description: "Admin namespace methods (Geth administrative)",
			Methods: []types.TestMethod{
				{Name: rpc.AdminAddPeer, Handler: nil, SkipReason: "Administrative method not exposed"},
				{Name: rpc.AdminNodeInfo, Handler: nil, SkipReason: "Administrative method not exposed"},
				{Name: rpc.AdminPeers, Handler: nil, SkipReason: "Administrative method not exposed"},
				{Name: rpc.AdminDatadir, Handler: nil, SkipReason: "Administrative method not exposed"},
			},
		},
	}

	// Execute tests by category
	for _, category := range testCategories {
		for _, method := range category.Methods {
			if method.Handler == nil {
				// Handle methods with no handler
				if category.Name == "engine" {
					result, _ := rpc.RpcSkipped(method.Name, category.Name, method.SkipReason)
					results = append(results, result)
				} else {
					result, _ := rpc.RpcNotImplemented(method.Name, category.Name)
					results = append(results, result)
				}
				continue
			}

			// Execute the test
			handler := method.Handler.(func(*rpc.RpcContext) (*types.RpcResult, error))
			result, err := handler(rCtx)
			if err != nil {
				result = &types.RpcResult{
					Method:      method.Name,
					Status:      types.Error,
					ErrMsg:      err.Error(),
					Category:    category.Name,
					Subcategory: getSubcategory(method.Name),
				}
			}
			// Ensure category is set and add subcategory
			if result.Category == "" {
				result.Category = category.Name
			}
			if result.Subcategory == "" {
				result.Subcategory = getSubcategory(method.Name)
			}
			
			// Mark personal/miner methods as deprecated if they pass
			if (category.Name == "personal" || isDeprecatedMethod(method.Name)) && result.Status == types.Ok {
				result.Status = types.Deprecated
			}
			
			results = append(results, result)
		}
	}

	// Add results from transaction tests that were automatically added (avoid duplicates)
	alreadyTested := make(map[types.RpcName]bool)
	for _, result := range results {
		alreadyTested[result.Method] = true
	}
	
	for _, result := range rCtx.AlreadyTestedRPCs {
		// Skip if we already tested this method in the categorized tests
		if alreadyTested[result.Method] {
			continue
		}
		
		if result.Category == "" {
			// Categorize based on method name
			switch result.Method {
			case rpc.EthGetTransactionReceipt:
				result.Category = "Transaction"
			case rpc.EthGetBlockByNumber, rpc.EthGetBlockByHash:
				result.Category = "Block"
			default:
				result.Category = "Uncategorized"
			}
		}
		results = append(results, result)
	}

	report.ReportResults(results, *verbose, *outputExcel)
}

// getSubcategory determines the functional subcategory of a method based on execution-apis structure
func getSubcategory(methodName types.RpcName) string {
	switch methodName {
	// Eth namespace - client subcategory
	case rpc.EthChainId, rpc.EthSyncing, rpc.EthCoinbase, rpc.EthAccounts, rpc.EthBlockNumber, rpc.EthMining, rpc.EthHashrate:
		return "client"
	
	// Eth namespace - fee_market subcategory  
	case rpc.EthGasPrice, rpc.EthBlobBaseFee, rpc.EthMaxPriorityFeePerGas, rpc.EthFeeHistory:
		return "fee_market"
	
	// Eth namespace - state subcategory
	case rpc.EthGetBalance, rpc.EthGetStorageAt, rpc.EthGetTransactionCount, rpc.EthGetCode, rpc.EthGetProof:
		return "state"
	
	// Eth namespace - block subcategory
	case rpc.EthGetBlockByHash, rpc.EthGetBlockByNumber, rpc.EthGetBlockTransactionCountByHash, rpc.EthGetBlockTransactionCountByNumber,
		 rpc.EthGetUncleCountByBlockHash, rpc.EthGetUncleCountByBlockNumber, rpc.EthGetUncleByBlockHashAndIndex, rpc.EthGetUncleByBlockNumberAndIndex,
		 rpc.EthGetBlockReceipts:
		return "block"
	
	// Eth namespace - transaction subcategory
	case rpc.EthGetTransactionByHash, rpc.EthGetTransactionByBlockHashAndIndex, rpc.EthGetTransactionByBlockNumberAndIndex,
		 rpc.EthGetTransactionReceipt, rpc.EthGetTransactionCountByHash, rpc.EthPendingTransactions:
		return "transaction"
	
	// Eth namespace - filter subcategory
	case rpc.EthNewFilter, rpc.EthNewBlockFilter, rpc.EthNewPendingTransactionFilter, rpc.EthGetFilterChanges, 
		 rpc.EthGetFilterLogs, rpc.EthUninstallFilter, rpc.EthGetLogs:
		return "filter"
	
	// Eth namespace - execute subcategory
	case rpc.Call, rpc.EstimateGas:
		return "execute"
	
	// Eth namespace - submit subcategory
	case rpc.EthSendTransaction, rpc.EthSendRawTransaction:
		return "submit"
	
	// Eth namespace - sign subcategory (deprecated)
	case rpc.EthSign, rpc.EthSignTransaction:
		return "sign"
	
	// Eth namespace - deprecated/other methods
	case rpc.EthProtocolVersion, rpc.EthGetCompilers, rpc.EthCompileSolidity, rpc.EthGetWork, rpc.EthSubmitWork, rpc.EthSubmitHashrate, rpc.EthCreateAccessList:
		return "deprecated"
	
	// Debug namespace subcategories
	case rpc.DebugTraceTransaction, rpc.DebugTraceBlock, rpc.DebugTraceCall, rpc.DebugIntermediateRoots:
		return "tracing"
	case rpc.DebugDbGet, rpc.DebugDbAncient, rpc.DebugChaindbCompact, rpc.DebugGetModifiedAccounts, rpc.DebugDumpBlock:
		return "database"
	case rpc.DebugBlockProfile, rpc.DebugCpuProfile, rpc.DebugGoTrace, rpc.DebugMemStats, rpc.DebugMutexProfile, rpc.DebugSetBlockProfileRate, rpc.DebugSetMutexProfileFraction:
		return "profiling"
	case rpc.DebugBacktraceAt, rpc.DebugStacks, rpc.DebugGetBadBlocks, rpc.DebugPreimage, rpc.DebugFreeOSMemory, rpc.DebugSetHead:
		return "diagnostics"
	
	// Miner methods (deprecated)
	case rpc.MinerStart, rpc.MinerStop, rpc.MinerSetEtherbase, rpc.MinerSetExtra, rpc.MinerSetGasPrice, rpc.MinerSetGasLimit, rpc.MinerGetHashrate:
		return "mining"
	
	// Personal methods (deprecated)
	case rpc.PersonalListAccounts, rpc.PersonalEcRecover, rpc.PersonalListWallets, rpc.PersonalNewAccount, rpc.PersonalImportRawKey, 
		 rpc.PersonalUnlockAccount, rpc.PersonalLockAccount, rpc.PersonalSign, rpc.PersonalSignTypedData:
		return "account_management"
	
	// TxPool methods
	case rpc.TxPoolContent, rpc.TxPoolInspect, rpc.TxPoolStatus:
		return "mempool"
	
	// Engine API methods  
	case rpc.EngineNewPayloadV1, rpc.EngineNewPayloadV2, rpc.EngineNewPayloadV3, rpc.EngineForkchoiceUpdatedV1, rpc.EngineForkchoiceUpdatedV2, rpc.EngineForkchoiceUpdatedV3,
		 rpc.EngineGetPayloadV1, rpc.EngineGetPayloadV2, rpc.EngineGetPayloadV3:
		return "consensus"
	
	// Trace methods
	case rpc.TraceCall, rpc.TraceCallMany, rpc.TraceTransaction, rpc.TraceBlock:
		return "tracing"
	
	// Admin methods
	case rpc.AdminAddPeer, rpc.AdminNodeInfo, rpc.AdminPeers, rpc.AdminDatadir:
		return "node_management"
	
	// Web3 methods
	case rpc.Web3ClientVersion, rpc.Web3Sha3:
		return "utility"
	
	// Net methods
	case rpc.NetVersion, rpc.NetPeerCount, rpc.NetListening:
		return "network"
	
	default:
		return "other"
	}
}

// isDeprecatedMethod checks if a method is deprecated
func isDeprecatedMethod(methodName types.RpcName) bool {
	// Miner methods are deprecated
	switch methodName {
	case rpc.MinerStart, rpc.MinerStop, rpc.MinerSetEtherbase, rpc.MinerSetExtra, rpc.MinerSetGasPrice, rpc.MinerSetGasLimit, rpc.MinerGetHashrate:
		return true
	// Personal methods are deprecated (checked by category in main logic)
	default:
		return false
	}
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
