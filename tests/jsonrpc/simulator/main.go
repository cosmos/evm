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
				{Name: rpc.EthCall, Handler: rpc.RpcEthCall},
				{Name: rpc.EthEstimateGas, Handler: rpc.RpcEthEstimateGas},
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
			Description: "Personal namespace methods (deprecated in favor of Clef)",
			Methods: []types.TestMethod{
				// Account Management subcategory
				{Name: rpc.PersonalListAccounts, Handler: rpc.RpcPersonalListAccounts},
				{Name: rpc.PersonalNewAccount, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.PersonalNewAccount, "personal")
				}},
				{Name: rpc.PersonalDeriveAccount, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.PersonalDeriveAccount, "personal")
				}},
				// Wallet Management subcategory
				{Name: rpc.PersonalListWallets, Handler: rpc.RpcPersonalListWallets},
				{Name: rpc.PersonalOpenWallet, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.PersonalOpenWallet, "personal")
				}},
				{Name: rpc.PersonalInitializeWallet, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.PersonalInitializeWallet, "personal")
				}},
				{Name: rpc.PersonalUnpair, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.PersonalUnpair, "personal")
				}},
				// Key Management subcategory
				{Name: rpc.PersonalImportRawKey, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.PersonalImportRawKey, "personal")
				}},
				{Name: rpc.PersonalUnlockAccount, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.PersonalUnlockAccount, "personal")
				}},
				{Name: rpc.PersonalLockAccount, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.PersonalLockAccount, "personal")
				}},
				// Signing subcategory
				{Name: rpc.PersonalSign, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.PersonalSign, "personal")
				}},
				{Name: rpc.PersonalSignTransaction, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.PersonalSignTransaction, "personal")
				}},
				{Name: rpc.PersonalSignTypedData, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.PersonalSignTypedData, "personal")
				}},
				{Name: rpc.PersonalEcRecover, Handler: rpc.RpcPersonalEcRecover},
				// Transaction subcategory
				{Name: rpc.PersonalSendTransaction, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.PersonalSendTransaction, "personal")
				}},
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
				{Name: rpc.TxPoolContentFrom, Handler: rpc.RpcTxPoolContentFrom},
				{Name: rpc.TxPoolInspect, Handler: rpc.RpcTxPoolInspect},
				{Name: rpc.TxPoolStatus, Handler: rpc.RpcTxPoolStatus},
			},
		},
		{
			Name:        "debug",
			Description: "Debug namespace methods from Geth",
			Methods: []types.TestMethod{
				// Tracing subcategory
				{Name: rpc.DebugTraceTransaction, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugTraceTransaction, "debug")
				}},
				{Name: rpc.DebugTraceBlock, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugTraceBlock, "debug")
				}},
				{Name: rpc.DebugTraceCall, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugTraceCall, "debug")
				}},
				{Name: rpc.DebugIntermediateRoots, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugIntermediateRoots, "debug")
				}},
				// Database subcategory
				{Name: rpc.DebugDbGet, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugDbGet, "debug")
				}},
				{Name: rpc.DebugDbAncient, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugDbAncient, "debug")
				}},
				{Name: rpc.DebugDbAncients, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugDbAncients, "debug")
				}},
				{Name: rpc.DebugChaindbCompact, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugChaindbCompact, "debug")
				}},
				{Name: rpc.DebugChaindbProperty, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugChaindbProperty, "debug")
				}},
				{Name: rpc.DebugGetModifiedAccounts, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugGetModifiedAccounts, "debug")
				}},
				{Name: rpc.DebugGetModifiedAccountsByHash, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugGetModifiedAccountsByHash, "debug")
				}},
				{Name: rpc.DebugGetModifiedAccountsByNumber, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugGetModifiedAccountsByNumber, "debug")
				}},
				{Name: rpc.DebugDumpBlock, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugDumpBlock, "debug")
				}},
				// Profiling subcategory
				{Name: rpc.DebugBlockProfile, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugBlockProfile, "debug")
				}},
				{Name: rpc.DebugCpuProfile, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugCpuProfile, "debug")
				}},
				{Name: rpc.DebugGoTrace, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugGoTrace, "debug")
				}},
				{Name: rpc.DebugMemStats, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugMemStats, "debug")
				}},
				{Name: rpc.DebugMutexProfile, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugMutexProfile, "debug")
				}},
				{Name: rpc.DebugSetBlockProfileRate, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugSetBlockProfileRate, "debug")
				}},
				{Name: rpc.DebugSetMutexProfileFraction, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugSetMutexProfileFraction, "debug")
				}},
				{Name: rpc.DebugGcStats, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugGcStats, "debug")
				}},
				// Diagnostics subcategory
				{Name: rpc.DebugBacktraceAt, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugBacktraceAt, "debug")
				}},
				{Name: rpc.DebugStacks, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugStacks, "debug")
				}},
				{Name: rpc.DebugGetBadBlocks, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugGetBadBlocks, "debug")
				}},
				{Name: rpc.DebugPreimage, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugPreimage, "debug")
				}},
				{Name: rpc.DebugFreeOSMemory, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugFreeOSMemory, "debug")
				}},
				{Name: rpc.DebugSetHead, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugSetHead, "debug")
				}},
				{Name: rpc.DebugGetAccessibleState, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugGetAccessibleState, "debug")
				}},
				{Name: rpc.DebugFreezeClient, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugFreezeClient, "debug")
				}},
				// New debug methods (including debug_setGCPercent)
				{Name: rpc.DebugSetGCPercent, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugSetGCPercent, "debug")
				}},
				{Name: rpc.DebugAccountRange, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugAccountRange, "debug")
				}},
				{Name: rpc.DebugGetRawBlock, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugGetRawBlock, "debug")
				}},
				{Name: rpc.DebugGetRawHeader, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugGetRawHeader, "debug")
				}},
				{Name: rpc.DebugGetRawTransaction, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugGetRawTransaction, "debug")
				}},
				{Name: rpc.DebugGetRawReceipts, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugGetRawReceipts, "debug")
				}},
				{Name: rpc.DebugPrintBlock, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.DebugPrintBlock, "debug")
				}},
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
			Name:        "admin",
			Description: "Admin namespace methods (Geth administrative)",
			Methods: []types.TestMethod{
				// Test all admin methods to see if they're implemented
				{Name: rpc.AdminAddPeer, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.AdminAddPeer, "admin")
				}},
				{Name: rpc.AdminAddTrustedPeer, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.AdminAddTrustedPeer, "admin")
				}},
				{Name: rpc.AdminDatadir, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.AdminDatadir, "admin")
				}},
				{Name: rpc.AdminExportChain, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.AdminExportChain, "admin")
				}},
				{Name: rpc.AdminImportChain, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.AdminImportChain, "admin")
				}},
				{Name: rpc.AdminNodeInfo, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.AdminNodeInfo, "admin")
				}},
				{Name: rpc.AdminPeerEvents, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.AdminPeerEvents, "admin")
				}},
				{Name: rpc.AdminPeers, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.AdminPeers, "admin")
				}},
				{Name: rpc.AdminRemovePeer, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.AdminRemovePeer, "admin")
				}},
				{Name: rpc.AdminRemoveTrustedPeer, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.AdminRemoveTrustedPeer, "admin")
				}},
				{Name: rpc.AdminStartHTTP, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.AdminStartHTTP, "admin")
				}},
				{Name: rpc.AdminStartWS, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.AdminStartWS, "admin")
				}},
				{Name: rpc.AdminStopHTTP, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.AdminStopHTTP, "admin")
				}},
				{Name: rpc.AdminStopWS, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.AdminStopWS, "admin")
				}},
			},
		},
		{
			Name:        "les",
			Description: "LES namespace methods (Light Ethereum Subprotocol)",
			Methods: []types.TestMethod{
				// Test all LES methods to see if they're implemented
				{Name: rpc.LesServerInfo, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.LesServerInfo, "les")
				}},
				{Name: rpc.LesClientInfo, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.LesClientInfo, "les")
				}},
				{Name: rpc.LesPriorityClientInfo, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.LesPriorityClientInfo, "les")
				}},
				{Name: rpc.LesAddBalance, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.LesAddBalance, "les")
				}},
				{Name: rpc.LesSetClientParams, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.LesSetClientParams, "les")
				}},
				{Name: rpc.LesSetDefaultParams, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.LesSetDefaultParams, "les")
				}},
				{Name: rpc.LesLatestCheckpoint, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.LesLatestCheckpoint, "les")
				}},
				{Name: rpc.LesGetCheckpoint, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.LesGetCheckpoint, "les")
				}},
				{Name: rpc.LesGetCheckpointContractAddress, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.RpcGenericTest(rCtx, rpc.LesGetCheckpointContractAddress, "les")
				}},
			},
		},
	}

	// Execute tests by category
	for _, category := range testCategories {
		for _, method := range category.Methods {
			if method.Handler == nil {
				// Handle methods with no handler - only skip engine methods, test others
				if category.Name == "engine" {
					result, _ := rpc.RpcSkipped(method.Name, category.Name, method.SkipReason)
					results = append(results, result)
				} else {
					// Test the method to see if it's actually implemented
					result, _ := rpc.RpcGenericTest(rCtx, method.Name, category.Name)
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
				}
			}
			// Ensure category is set
			if result.Category == "" {
				result.Category = category.Name
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
			// Categorize based on method name using the namespace
			methodStr := string(result.Method)
			if strings.HasPrefix(methodStr, "eth_") {
				result.Category = "eth"
			} else if strings.HasPrefix(methodStr, "web3_") {
				result.Category = "web3"
			} else if strings.HasPrefix(methodStr, "net_") {
				result.Category = "net"
			} else if strings.HasPrefix(methodStr, "personal_") {
				result.Category = "personal"
			} else if strings.HasPrefix(methodStr, "debug_") {
				result.Category = "debug"
			} else if strings.HasPrefix(methodStr, "txpool_") {
				result.Category = "txpool"
			} else if strings.HasPrefix(methodStr, "miner_") {
				result.Category = "miner"
			} else if strings.HasPrefix(methodStr, "admin_") {
				result.Category = "admin"
			} else if strings.HasPrefix(methodStr, "engine_") {
				result.Category = "engine"
			} else if strings.HasPrefix(methodStr, "les_") {
				result.Category = "les"
			} else {
				result.Category = "Uncategorized"
			}
		}
		results = append(results, result)
	}

	report.ReportResults(results, *verbose, *outputExcel)
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
