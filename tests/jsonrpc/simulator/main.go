package main

import (
	_ "embed"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/contracts"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/report"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/rpc"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/utils"
)

func main() {
	verbose := flag.Bool("v", false, "Enable verbose output")
	outputExcel := flag.Bool("xlsx", false, "Save output as xlsx")
	fundAccounts := flag.Bool("fund-geth", false, "Fund standard dev accounts in geth using coinbase")
	flag.Parse()

	// Load configuration from conf.yaml
	conf := config.MustLoadConfig("config.yaml")

	// Handle account funding if requested
	if *fundAccounts {
		log.Println("Funding standard dev accounts in geth...")
		err := fundGethAccounts(conf)
		if err != nil {
			log.Fatalf("Failed to fund geth accounts: %v", err)
		}
		log.Println("✓ Account funding completed successfully!")
		return
	}

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
				{Name: rpc.MethodNameWeb3ClientVersion, Handler: rpc.Web3ClientVersion},
				{Name: rpc.MethodNameWeb3Sha3, Handler: rpc.Web3Sha3},
			},
		},
		{
			Name:        "net",
			Description: "Net namespace network methods",
			Methods: []types.TestMethod{
				{Name: rpc.MethodNameNetVersion, Handler: rpc.NetVersion},
				{Name: rpc.MethodNameNetPeerCount, Handler: rpc.NetPeerCount},
				{Name: rpc.MethodNameNetListening, Handler: rpc.NetListening},
			},
		},
		{
			Name:        "eth",
			Description: "Ethereum namespace methods from execution-apis",
			Methods: []types.TestMethod{
				// Client subcategory
				{Name: rpc.MethodNameEthChainId, Handler: rpc.EthChainId},
				{Name: rpc.MethodNameEthSyncing, Handler: rpc.EthSyncing},
				{Name: rpc.MethodNameEthCoinbase, Handler: rpc.EthCoinbase},
				{Name: rpc.MethodNameEthAccounts, Handler: rpc.EthAccounts},
				{Name: rpc.MethodNameEthBlockNumber, Handler: rpc.EthBlockNumber},
				{Name: rpc.MethodNameEthMining, Handler: rpc.EthMining},
				// Fee market subcategory
				{Name: rpc.MethodNameEthGasPrice, Handler: rpc.EthGasPrice},
				{Name: rpc.MethodNameEthMaxPriorityFeePerGas, Handler: rpc.EthMaxPriorityFeePerGas},
				// State subcategory
				{Name: rpc.MethodNameEthGetBalance, Handler: rpc.EthGetBalance},
				{Name: rpc.MethodNameEthGetTransactionCount, Handler: rpc.EthGetTransactionCount},
				{Name: rpc.MethodNameEthGetCode, Handler: rpc.EthGetCode},
				{Name: rpc.MethodNameEthGetStorageAt, Handler: rpc.EthGetStorageAt},
				// Block subcategory
				{Name: rpc.MethodNameEthGetBlockByHash, Handler: rpc.EthGetBlockByHash},
				{Name: rpc.MethodNameEthGetBlockByNumber, Handler: rpc.EthGetBlockByNumber},
				{Name: rpc.MethodNameEthGetBlockTransactionCountByHash, Handler: rpc.EthGetBlockTransactionCountByHash},
				{Name: rpc.MethodNameEthGetBlockReceipts, Handler: rpc.EthGetBlockReceipts},
				// Transaction subcategory
				{Name: rpc.MethodNameEthGetTransactionByHash, Handler: rpc.EthGetTransactionByHash},
				{Name: rpc.MethodNameEthGetTransactionByBlockHashAndIndex, Handler: rpc.EthGetTransactionByBlockHashAndIndex},
				{Name: rpc.MethodNameEthGetTransactionByBlockNumberAndIndex, Handler: rpc.EthGetTransactionByBlockNumberAndIndex},
				{Name: rpc.MethodNameEthGetTransactionReceipt, Handler: rpc.EthGetTransactionReceipt},
				{Name: rpc.MethodNameEthGetTransactionCountByHash, Handler: rpc.EthGetTransactionCountByHash},
				// Execute subcategory
				{Name: rpc.MethodNameEthCall, Handler: rpc.EthCall},
				{Name: rpc.MethodNameEthEstimateGas, Handler: rpc.EthEstimateGas},
				// Submit subcategory
				{Name: rpc.MethodNameEthSendRawTransaction, Handler: rpc.EthSendRawTransactionTransferValue, Description: "Transfer value"},
				{Name: rpc.MethodNameEthSendRawTransaction, Handler: rpc.EthSendRawTransactionDeployContract, Description: "Deploy contract"},
				{Name: rpc.MethodNameEthSendRawTransaction, Handler: rpc.EthSendRawTransactionTransferERC20, Description: "Transfer ERC20"},
				// Filter subcategory
				{Name: rpc.MethodNameEthNewFilter, Handler: rpc.EthNewFilter},
				{Name: rpc.MethodNameEthGetFilterLogs, Handler: rpc.EthGetFilterLogs},
				{Name: rpc.MethodNameEthNewBlockFilter, Handler: rpc.EthNewBlockFilter},
				{Name: rpc.MethodNameEthGetFilterChanges, Handler: rpc.EthGetFilterChanges},
				{Name: rpc.MethodNameEthUninstallFilter, Handler: rpc.EthUninstallFilter},
				{Name: rpc.MethodNameEthGetLogs, Handler: rpc.EthGetLogs},
				// Other/not implemented methods
				{Name: rpc.MethodNameEthBlobBaseFee, Handler: nil, SkipReason: "EIP-4844 blob base fee (post-Cancun)"},
				{Name: rpc.MethodNameEthFeeHistory, Handler: nil, SkipReason: "Fee history not implemented"},
				{Name: rpc.MethodNameEthGetProof, Handler: nil, SkipReason: "State proof not implemented"},
				{Name: rpc.MethodNameEthProtocolVersion, Handler: nil, SkipReason: "Protocol version deprecated"},
				{Name: rpc.MethodNameEthCreateAccessList, Handler: nil, SkipReason: "Access list creation not implemented"},
			},
		},
		{
			Name:        "personal",
			Description: "Personal namespace methods (deprecated in favor of Clef)",
			Methods: []types.TestMethod{
				// Account Management subcategory
				{Name: rpc.MethodNamePersonalListAccounts, Handler: rpc.PersonalListAccounts},
				{Name: rpc.MethodNamePersonalNewAccount, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNamePersonalNewAccount, "personal")
				}},
				{Name: rpc.MethodNamePersonalDeriveAccount, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNamePersonalDeriveAccount, "personal")
				}},
				// Wallet Management subcategory
				{Name: rpc.MethodNamePersonalListWallets, Handler: rpc.PersonalListWallets},
				{Name: rpc.MethodNamePersonalOpenWallet, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNamePersonalOpenWallet, "personal")
				}},
				{Name: rpc.MethodNamePersonalInitializeWallet, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNamePersonalInitializeWallet, "personal")
				}},
				{Name: rpc.MethodNamePersonalUnpair, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNamePersonalUnpair, "personal")
				}},
				// Key Management subcategory
				{Name: rpc.MethodNamePersonalImportRawKey, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNamePersonalImportRawKey, "personal")
				}},
				{Name: rpc.MethodNamePersonalUnlockAccount, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNamePersonalUnlockAccount, "personal")
				}},
				{Name: rpc.MethodNamePersonalLockAccount, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNamePersonalLockAccount, "personal")
				}},
				// Signing subcategory
				{Name: rpc.MethodNamePersonalSign, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNamePersonalSign, "personal")
				}},
				{Name: rpc.MethodNamePersonalSignTransaction, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNamePersonalSignTransaction, "personal")
				}},
				{Name: rpc.MethodNamePersonalSignTypedData, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNamePersonalSignTypedData, "personal")
				}},
				{Name: rpc.MethodNamePersonalEcRecover, Handler: rpc.PersonalEcRecover},
				// Transaction subcategory
				{Name: rpc.MethodNamePersonalSendTransaction, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNamePersonalSendTransaction, "personal")
				}},
			},
		},
		{
			Name:        "miner",
			Description: "Miner namespace methods (deprecated)",
			Methods: []types.TestMethod{
				{Name: rpc.MethodNameMinerStart, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: rpc.MethodNameMinerStop, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: rpc.MethodNameMinerSetEtherbase, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: rpc.MethodNameMinerSetExtra, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: rpc.MethodNameMinerSetGasPrice, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: rpc.MethodNameMinerSetGasLimit, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: rpc.MethodNameMinerGetHashrate, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
			},
		},
		{
			Name:        "txpool",
			Description: "TxPool namespace methods",
			Methods: []types.TestMethod{
				{Name: rpc.MethodNameTxPoolContent, Handler: rpc.TxPoolContent},
				{Name: rpc.MethodNameTxPoolContentFrom, Handler: rpc.TxPoolContentFrom},
				{Name: rpc.MethodNameTxPoolInspect, Handler: rpc.TxPoolInspect},
				{Name: rpc.MethodNameTxPoolStatus, Handler: rpc.TxPoolStatus},
			},
		},
		{
			Name:        "debug",
			Description: "Debug namespace methods from Geth",
			Methods: []types.TestMethod{
				// Tracing subcategory
				{Name: rpc.MethodNameDebugTraceTransaction, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugTraceTransaction, "debug")
				}},
				{Name: rpc.MethodNameDebugTraceBlock, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugTraceBlock, "debug")
				}},
				{Name: rpc.MethodNameDebugTraceCall, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugTraceCall, "debug")
				}},
				{Name: rpc.MethodNameDebugIntermediateRoots, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugIntermediateRoots, "debug")
				}},
				// Database subcategory
				{Name: rpc.MethodNameDebugDbGet, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugDbGet, "debug")
				}},
				{Name: rpc.MethodNameDebugDbAncient, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugDbAncient, "debug")
				}},
				{Name: rpc.MethodNameDebugDbAncients, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugDbAncients, "debug")
				}},
				{Name: rpc.MethodNameDebugChaindbCompact, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugChaindbCompact, "debug")
				}},
				{Name: rpc.MethodNameDebugChaindbProperty, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugChaindbProperty, "debug")
				}},
				{Name: rpc.MethodNameDebugGetModifiedAccounts, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugGetModifiedAccounts, "debug")
				}},
				{Name: rpc.MethodNameDebugGetModifiedAccountsByHash, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugGetModifiedAccountsByHash, "debug")
				}},
				{Name: rpc.MethodNameDebugGetModifiedAccountsByNumber, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugGetModifiedAccountsByNumber, "debug")
				}},
				{Name: rpc.MethodNameDebugDumpBlock, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugDumpBlock, "debug")
				}},
				// Profiling subcategory
				{Name: rpc.MethodNameDebugBlockProfile, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugBlockProfile, "debug")
				}},
				{Name: rpc.MethodNameDebugCpuProfile, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugCpuProfile, "debug")
				}},
				{Name: rpc.MethodNameDebugGoTrace, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugGoTrace, "debug")
				}},
				{Name: rpc.MethodNameDebugMemStats, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugMemStats, "debug")
				}},
				{Name: rpc.MethodNameDebugMutexProfile, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugMutexProfile, "debug")
				}},
				{Name: rpc.MethodNameDebugSetBlockProfileRate, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugSetBlockProfileRate, "debug")
				}},
				{Name: rpc.MethodNameDebugSetMutexProfileFraction, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugSetMutexProfileFraction, "debug")
				}},
				{Name: rpc.MethodNameDebugGcStats, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugGcStats, "debug")
				}},
				// Diagnostics subcategory
				{Name: rpc.MethodNameDebugBacktraceAt, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugBacktraceAt, "debug")
				}},
				{Name: rpc.MethodNameDebugStacks, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugStacks, "debug")
				}},
				{Name: rpc.MethodNameDebugGetBadBlocks, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugGetBadBlocks, "debug")
				}},
				{Name: rpc.MethodNameDebugPreimage, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugPreimage, "debug")
				}},
				{Name: rpc.MethodNameDebugFreeOSMemory, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugFreeOSMemory, "debug")
				}},
				{Name: rpc.MethodNameDebugSetHead, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugSetHead, "debug")
				}},
				{Name: rpc.MethodNameDebugGetAccessibleState, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugGetAccessibleState, "debug")
				}},
				{Name: rpc.MethodNameDebugFreezeClient, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugFreezeClient, "debug")
				}},
				// New debug methods (including debug_setGCPercent)
				{Name: rpc.MethodNameDebugSetGCPercent, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugSetGCPercent, "debug")
				}},
				{Name: rpc.MethodNameDebugAccountRange, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugAccountRange, "debug")
				}},
				{Name: rpc.MethodNameDebugGetRawBlock, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugGetRawBlock, "debug")
				}},
				{Name: rpc.MethodNameDebugGetRawHeader, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugGetRawHeader, "debug")
				}},
				{Name: rpc.MethodNameDebugGetRawTransaction, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugGetRawTransaction, "debug")
				}},
				{Name: rpc.MethodNameDebugGetRawReceipts, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugGetRawReceipts, "debug")
				}},
				{Name: rpc.MethodNameDebugPrintBlock, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameDebugPrintBlock, "debug")
				}},
			},
		},
		{
			Name:        "engine",
			Description: "Engine API methods (not applicable for Cosmos chains)",
			Methods: []types.TestMethod{
				{Name: rpc.MethodNameEngineNewPayloadV1, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: rpc.MethodNameEngineForkchoiceUpdatedV1, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: rpc.MethodNameEngineGetPayloadV1, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: rpc.MethodNameEngineNewPayloadV2, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: rpc.MethodNameEngineForkchoiceUpdatedV2, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: rpc.MethodNameEngineGetPayloadV2, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
			},
		},
		{
			Name:        "admin",
			Description: "Admin namespace methods (Geth administrative)",
			Methods: []types.TestMethod{
				// Test all admin methods to see if they're implemented
				{Name: rpc.MethodNameAdminAddPeer, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameAdminAddPeer, "admin")
				}},
				{Name: rpc.MethodNameAdminAddTrustedPeer, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameAdminAddTrustedPeer, "admin")
				}},
				{Name: rpc.MethodNameAdminDatadir, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameAdminDatadir, "admin")
				}},
				{Name: rpc.MethodNameAdminExportChain, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameAdminExportChain, "admin")
				}},
				{Name: rpc.MethodNameAdminImportChain, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameAdminImportChain, "admin")
				}},
				{Name: rpc.MethodNameAdminNodeInfo, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameAdminNodeInfo, "admin")
				}},
				{Name: rpc.MethodNameAdminPeerEvents, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameAdminPeerEvents, "admin")
				}},
				{Name: rpc.MethodNameAdminPeers, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameAdminPeers, "admin")
				}},
				{Name: rpc.MethodNameAdminRemovePeer, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameAdminRemovePeer, "admin")
				}},
				{Name: rpc.MethodNameAdminRemoveTrustedPeer, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameAdminRemoveTrustedPeer, "admin")
				}},
				{Name: rpc.MethodNameAdminStartHTTP, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameAdminStartHTTP, "admin")
				}},
				{Name: rpc.MethodNameAdminStartWS, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameAdminStartWS, "admin")
				}},
				{Name: rpc.MethodNameAdminStopHTTP, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameAdminStopHTTP, "admin")
				}},
				{Name: rpc.MethodNameAdminStopWS, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameAdminStopWS, "admin")
				}},
			},
		},
		{
			Name:        "les",
			Description: "LES namespace methods (Light Ethereum Subprotocol)",
			Methods: []types.TestMethod{
				// Test all LES methods to see if they're implemented
				{Name: rpc.MethodNameLesServerInfo, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameLesServerInfo, "les")
				}},
				{Name: rpc.MethodNameLesClientInfo, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameLesClientInfo, "les")
				}},
				{Name: rpc.MethodNameLesPriorityClientInfo, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameLesPriorityClientInfo, "les")
				}},
				{Name: rpc.MethodNameLesAddBalance, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameLesAddBalance, "les")
				}},
				{Name: rpc.MethodNameLesSetClientParams, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameLesSetClientParams, "les")
				}},
				{Name: rpc.MethodNameLesSetDefaultParams, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameLesSetDefaultParams, "les")
				}},
				{Name: rpc.MethodNameLesLatestCheckpoint, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameLesLatestCheckpoint, "les")
				}},
				{Name: rpc.MethodNameLesGetCheckpoint, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameLesGetCheckpoint, "les")
				}},
				{Name: rpc.MethodNameLesGetCheckpointContractAddress, Handler: func(rCtx *rpc.RpcContext) (*types.RpcResult, error) {
					return rpc.GenericTest(rCtx, rpc.MethodNameLesGetCheckpointContractAddress, "les")
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
					result, _ := rpc.Skipped(method.Name, category.Name, method.SkipReason)
					results = append(results, result)
				} else {
					// Test the method to see if it's actually implemented
					result, _ := rpc.GenericTest(rCtx, method.Name, category.Name)
					results = append(results, result)
				}
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

// fundGethAccounts funds the standard dev accounts in geth using coinbase balance
func fundGethAccounts(conf *config.Config) error {
	// For now, assume geth is running on localhost:8547 (based on our setup)
	gethURL := "http://localhost:8547"
	
	// Connect to geth
	client, err := ethclient.Dial(gethURL)
	if err != nil {
		return fmt.Errorf("failed to connect to geth at %s: %w", gethURL, err)
	}

	// Fund the accounts
	results, err := utils.FundStandardAccounts(client, gethURL)
	if err != nil {
		return fmt.Errorf("failed to fund accounts: %w", err)
	}

	// Print results
	fmt.Println("\nFunding Results:")
	for _, result := range results {
		if result.Success {
			fmt.Printf("✓ %s (%s): %s ETH - TX: %s\n", 
				result.Account, 
				result.Address.Hex(), 
				"1000", // We know it's 1000 ETH
				result.TxHash.Hex())
		} else {
			fmt.Printf("✗ %s (%s): Failed - %s\n", 
				result.Account, 
				result.Address.Hex(), 
				result.Error)
		}
	}

	// Wait for transactions to be mined
	fmt.Println("\nWaiting for transactions to be mined...")
	time.Sleep(15 * time.Second) // Dev mode mines every 12 seconds

	// Check final balances
	fmt.Println("\nChecking final balances:")
	balances, err := utils.CheckAccountBalances(client)
	if err != nil {
		return fmt.Errorf("failed to check balances: %w", err)
	}

	for name, balance := range balances {
		address := utils.StandardDevAccounts[name]
		ethBalance := new(big.Int).Div(balance, big.NewInt(1e18)) // Convert wei to ETH
		fmt.Printf("%s (%s): %s ETH\n", name, address.Hex(), ethBalance.String())
	}

	fmt.Println("\n✓ Now dev1-dev4 accounts have matching balances on both evmd and geth")
	return nil
}
