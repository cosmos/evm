package rpc

import (
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

// GetTestCategories returns the comprehensive test configuration organized by namespace
// based on the execution-apis structure
func GetTestCategories() []types.TestCategory {
	return []types.TestCategory{
		{
			Name:        "web3",
			Description: "Web3 namespace utility methods",
			Methods: []types.TestMethod{
				{Name: MethodNameWeb3ClientVersion, Handler: Web3ClientVersion},
				{Name: MethodNameWeb3Sha3, Handler: Web3Sha3},
			},
		},
		{
			Name:        "net",
			Description: "Net namespace network methods",
			Methods: []types.TestMethod{
				{Name: MethodNameNetVersion, Handler: NetVersion},
				{Name: MethodNameNetPeerCount, Handler: NetPeerCount},
				{Name: MethodNameNetListening, Handler: NetListening},
			},
		},
		{
			Name:        "eth",
			Description: "Ethereum namespace methods from execution-apis",
			Methods: []types.TestMethod{
				// Client subcategory
				{Name: MethodNameEthChainId, Handler: EthChainId},
				{Name: MethodNameEthSyncing, Handler: EthSyncing},
				{Name: MethodNameEthCoinbase, Handler: EthCoinbase},
				{Name: MethodNameEthAccounts, Handler: EthAccounts},
				{Name: MethodNameEthBlockNumber, Handler: EthBlockNumber},
				{Name: MethodNameEthMining, Handler: EthMining},
				{Name: MethodNameEthHashrate, Handler: nil},
				// Fee market subcategory
				{Name: MethodNameEthGasPrice, Handler: EthGasPrice},
				{Name: MethodNameEthMaxPriorityFeePerGas, Handler: EthMaxPriorityFeePerGas},
				// State subcategory
				{Name: MethodNameEthGetBalance, Handler: EthGetBalance},
				{Name: MethodNameEthGetTransactionCount, Handler: EthGetTransactionCount},
				{Name: MethodNameEthGetCode, Handler: EthGetCode},
				{Name: MethodNameEthGetStorageAt, Handler: EthGetStorageAt},
				// Block subcategory
				{Name: MethodNameEthGetBlockByHash, Handler: EthGetBlockByHash},
				{Name: MethodNameEthGetBlockByNumber, Handler: EthGetBlockByNumber},
				{Name: MethodNameEthGetBlockTransactionCountByHash, Handler: EthGetBlockTransactionCountByHash},
				{Name: MethodNameEthGetBlockReceipts, Handler: EthGetBlockReceipts},
				// Uncle subcategory (uncles don't exist in CometBFT, should return 0/nil)
				{Name: MethodNameEthGetUncleCountByBlockHash, Handler: EthGetUncleCountByBlockHash},
				{Name: MethodNameEthGetUncleCountByBlockNumber, Handler: EthGetUncleCountByBlockNumber},
				{Name: MethodNameEthGetUncleByBlockHashAndIndex, Handler: EthGetUncleByBlockHashAndIndex},
				{Name: MethodNameEthGetUncleByBlockNumberAndIndex, Handler: EthGetUncleByBlockNumberAndIndex},
				// Transaction subcategory
				{Name: MethodNameEthGetTransactionByHash, Handler: EthGetTransactionByHash},
				{Name: MethodNameEthGetTransactionByBlockHashAndIndex, Handler: EthGetTransactionByBlockHashAndIndex},
				{Name: MethodNameEthGetTransactionByBlockNumberAndIndex, Handler: EthGetTransactionByBlockNumberAndIndex},
				{Name: MethodNameEthGetTransactionReceipt, Handler: EthGetTransactionReceipt},
				{Name: MethodNameEthGetBlockTransactionCountByNumber, Handler: EthGetBlockTransactionCountByNumber},
				{Name: MethodNameEthGetPendingTransactions, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return Legacy(rCtx, MethodNameEthGetPendingTransactions, "eth", "Use eth_newPendingTransactionFilter + eth_getFilterChanges instead")
				}},
				// Execute subcategory
				{Name: MethodNameEthCall, Handler: EthCall},
				{Name: MethodNameEthEstimateGas, Handler: EthEstimateGas},
				// Submit subcategory
				{Name: MethodNameEthSendRawTransaction, Handler: EthSendRawTransaction, Description: "Combined test: Transfer value, Deploy contract, Transfer ERC20"},
				// Filter subcategory
				{Name: MethodNameEthNewFilter, Handler: EthNewFilter},
				{Name: MethodNameEthGetFilterLogs, Handler: EthGetFilterLogs},
				{Name: MethodNameEthNewBlockFilter, Handler: EthNewBlockFilter},
				{Name: MethodNameEthNewPendingTransactionFilter, Handler: nil},
				{Name: MethodNameEthGetFilterChanges, Handler: EthGetFilterChanges},
				{Name: MethodNameEthUninstallFilter, Handler: EthUninstallFilter},
				{Name: MethodNameEthGetLogs, Handler: EthGetLogs},
				// Other/not implemented methods
				{Name: MethodNameEthBlobBaseFee, Handler: nil, SkipReason: "EIP-4844 blob base fee (post-Cancun)"},
				{Name: MethodNameEthFeeHistory, Handler: EthFeeHistory},
				{Name: MethodNameEthGetProof, Handler: EthGetProof},
				{Name: MethodNameEthProtocolVersion, Handler: nil, SkipReason: "Protocol version deprecated"},
				{Name: MethodNameEthCreateAccessList, Handler: nil, SkipReason: "Access list creation not implemented"},
				// Standard methods that should be implemented
				{Name: MethodNameEthSendTransaction, Handler: EthSendTransaction},
				{Name: MethodNameEthSign, Handler: EthSign},
				{Name: MethodNameEthSignTransaction, Handler: nil},
				// WebSocket subscription methods (part of eth namespace)
				{Name: MethodNameEthSubscribe, Handler: EthSubscribe, Description: "WebSocket subscription with all 4 subscription types: newHeads, logs, newPendingTransactions, syncing"},
				{Name: MethodNameEthUnsubscribe, Handler: EthUnsubscribe, Description: "WebSocket unsubscription functionality"},
			},
		},
		{
			Name:        "personal",
			Description: "Personal namespace methods (deprecated in favor of Clef)",
			Methods: []types.TestMethod{
				// Account Management subcategory
				{Name: MethodNamePersonalListAccounts, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return Legacy(rCtx, MethodNamePersonalListAccounts, "personal", "Personal namespace deprecated - use external signers like Clef")
				}},
				{Name: MethodNamePersonalNewAccount, Handler: PersonalNewAccount},
				{Name: MethodNamePersonalDeriveAccount, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNamePersonalDeriveAccount, "personal")
				}},
				// Wallet Management subcategory
				{Name: MethodNamePersonalListWallets, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return Legacy(rCtx, MethodNamePersonalListWallets, "personal", "Personal namespace deprecated - use external signers like Clef")
				}},
				{Name: MethodNamePersonalOpenWallet, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNamePersonalOpenWallet, "personal")
				}},
				{Name: MethodNamePersonalInitializeWallet, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return Skipped(MethodNamePersonalInitializeWallet, "personal", "Cosmos EVM always returns false for personal namespace methods")
				}},
				{Name: MethodNamePersonalUnpair, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return Skipped(MethodNamePersonalUnpair, "personal", "Cosmos EVM always returns false for personal namespace methods")
				}},
				// Key Management subcategory
				{Name: MethodNamePersonalImportRawKey, Handler: PersonalImportRawKey},
				{Name: MethodNamePersonalUnlockAccount, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return Skipped(MethodNamePersonalUnlockAccount, "personal", "Cosmos EVM always returns false for personal namespace methods")
				}},
				{Name: MethodNamePersonalLockAccount, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return Skipped(MethodNamePersonalLockAccount, "personal", "Cosmos EVM always returns false for personal namespace methods")
				}},
				// Signing subcategory
				{Name: MethodNamePersonalSign, Handler: PersonalSign},
				{Name: MethodNamePersonalSignTransaction, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNamePersonalSignTransaction, "personal")
				}},
				{Name: MethodNamePersonalSignTypedData, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNamePersonalSignTypedData, "personal")
				}},
				{Name: MethodNamePersonalEcRecover, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return Legacy(rCtx, MethodNamePersonalEcRecover, "personal", "Personal namespace deprecated - use external signers like Clef")
				}},
				// Transaction subcategory
				{Name: MethodNamePersonalSendTransaction, Handler: PersonalSendTransaction},
			},
		},
		{
			Name:        "miner",
			Description: "Miner namespace methods (deprecated)",
			Methods: []types.TestMethod{
				{Name: MethodNameMinerStart, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: MethodNameMinerStop, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: MethodNameMinerSetEtherbase, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: MethodNameMinerSetExtra, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: MethodNameMinerSetGasPrice, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: MethodNameMinerSetGasLimit, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: MethodNameMinerGetHashrate, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
			},
		},
		{
			Name:        "txpool",
			Description: "TxPool namespace methods",
			Methods: []types.TestMethod{
				{Name: MethodNameTxPoolContent, Handler: TxPoolContent},
				{Name: MethodNameTxPoolContentFrom, Handler: TxPoolContentFrom},
				{Name: MethodNameTxPoolInspect, Handler: TxPoolInspect},
				{Name: MethodNameTxPoolStatus, Handler: TxPoolStatus},
			},
		},
		{
			Name:        "debug",
			Description: "Debug namespace methods from Geth",
			Methods: []types.TestMethod{
				// Tracing subcategory
				{Name: MethodNameDebugTraceTransaction, Handler: DebugTraceTransaction},
				{Name: MethodNameDebugTraceBlock, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugTraceBlock, "debug")
				}},
				{Name: MethodNameDebugTraceBlockByHash, Handler: DebugTraceBlockByHash},
				{Name: MethodNameDebugTraceBlockByNumber, Handler: DebugTraceBlockByNumber},
				{Name: MethodNameDebugTraceCall, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugTraceCall, "debug")
				}},
				{Name: MethodNameDebugIntermediateRoots, Handler: DebugIntermediateRoots},
				// Database subcategory
				{Name: MethodNameDebugDbGet, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugDbGet, "debug")
				}},
				{Name: MethodNameDebugDbAncient, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugDbAncient, "debug")
				}},
				{Name: MethodNameDebugDbAncients, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugDbAncients, "debug")
				}},
				{Name: MethodNameDebugChaindbCompact, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugChaindbCompact, "debug")
				}},
				{Name: MethodNameDebugChaindbProperty, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugChaindbProperty, "debug")
				}},
				{Name: MethodNameDebugGetModifiedAccounts, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugGetModifiedAccounts, "debug")
				}},
				{Name: MethodNameDebugGetModifiedAccountsByHash, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugGetModifiedAccountsByHash, "debug")
				}},
				{Name: MethodNameDebugGetModifiedAccountsByNumber, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugGetModifiedAccountsByNumber, "debug")
				}},
				{Name: MethodNameDebugDumpBlock, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugDumpBlock, "debug")
				}},
				// Profiling subcategory
				{Name: MethodNameDebugBlockProfile, Handler: DebugBlockProfile},
				{Name: MethodNameDebugCpuProfile, Handler: DebugCpuProfile},
				{Name: MethodNameDebugGoTrace, Handler: DebugGoTrace},
				{Name: MethodNameDebugMemStats, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugMemStats, "debug")
				}},
				{Name: MethodNameDebugMutexProfile, Handler: DebugMutexProfile},
				{Name: MethodNameDebugSetBlockProfileRate, Handler: DebugSetBlockProfileRate},
				{Name: MethodNameDebugSetMutexProfileFraction, Handler: DebugSetMutexProfileFraction},
				{Name: MethodNameDebugGcStats, Handler: DebugGcStats},
				// Diagnostics subcategory
				{Name: MethodNameDebugBacktraceAt, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugBacktraceAt, "debug")
				}},
				{Name: MethodNameDebugStacks, Handler: DebugStacks},
				{Name: MethodNameDebugGetBadBlocks, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugGetBadBlocks, "debug")
				}},
				{Name: MethodNameDebugPreimage, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugPreimage, "debug")
				}},
				{Name: MethodNameDebugFreeOSMemory, Handler: DebugFreeOSMemory},
				{Name: MethodNameDebugSetHead, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugSetHead, "debug")
				}},
				{Name: MethodNameDebugGetAccessibleState, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugGetAccessibleState, "debug")
				}},
				{Name: MethodNameDebugFreezeClient, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugFreezeClient, "debug")
				}},
				// New debug methods (including debug_setGCPercent)
				{Name: MethodNameDebugSetGCPercent, Handler: DebugSetGCPercent},
				{Name: MethodNameDebugAccountRange, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugAccountRange, "debug")
				}},
				{Name: MethodNameDebugGetRawBlock, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugGetRawBlock, "debug")
				}},
				{Name: MethodNameDebugGetRawHeader, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugGetRawHeader, "debug")
				}},
				{Name: MethodNameDebugGetRawTransaction, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugGetRawTransaction, "debug")
				}},
				{Name: MethodNameDebugGetRawReceipts, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugGetRawReceipts, "debug")
				}},
				{Name: MethodNameDebugPrintBlock, Handler: DebugPrintBlock},
				// Additional debug methods from Geth documentation
				{Name: MethodNameDebugStartCPUProfile, Handler: DebugStartCPUProfile, Description: "Start CPU profiling"},
				{Name: MethodNameDebugStopCPUProfile, Handler: DebugStopCPUProfile, Description: "Stop CPU profiling"},
				{Name: MethodNameDebugTraceBadBlock, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugTraceBadBlock, "debug")
				}, Description: "Trace bad blocks"},
				{Name: MethodNameDebugStandardTraceBlockToFile, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugStandardTraceBlockToFile, "debug")
				}, Description: "Standard trace block to file"},
				{Name: MethodNameDebugStorageRangeAt, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugStorageRangeAt, "debug")
				}, Description: "Get storage range at specific position"},
				{Name: MethodNameDebugSetTrieFlushInterval, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugSetTrieFlushInterval, "debug")
				}, Description: "Set trie flush interval"},
				{Name: MethodNameDebugVmodule, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugVmodule, "debug")
				}, Description: "Set logging verbosity pattern"},
				{Name: MethodNameDebugWriteBlockProfile, Handler: DebugWriteBlockProfile, Description: "Write block profile to file"},
				{Name: MethodNameDebugWriteMemProfile, Handler: DebugWriteMemProfile, Description: "Write memory profile to file"},
				{Name: MethodNameDebugWriteMutexProfile, Handler: DebugWriteMutexProfile, Description: "Write mutex profile to file"},
				{Name: MethodNameDebugVerbosity, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameDebugVerbosity, "debug")
				}, Description: "Set log verbosity level"},
			},
		},
		{
			Name:        "engine",
			Description: "Engine API methods (not applicable for Cosmos chains)",
			Methods: []types.TestMethod{
				{Name: MethodNameEngineNewPayloadV1, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: MethodNameEngineForkchoiceUpdatedV1, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: MethodNameEngineGetPayloadV1, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: MethodNameEngineNewPayloadV2, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: MethodNameEngineForkchoiceUpdatedV2, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: MethodNameEngineGetPayloadV2, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
			},
		},
		{
			Name:        "admin",
			Description: "Admin namespace methods (Geth administrative)",
			Methods: []types.TestMethod{
				// Test all admin methods to see if they're implemented
				{Name: MethodNameAdminAddPeer, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameAdminAddPeer, "admin")
				}},
				{Name: MethodNameAdminAddTrustedPeer, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameAdminAddTrustedPeer, "admin")
				}},
				{Name: MethodNameAdminDatadir, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameAdminDatadir, "admin")
				}},
				{Name: MethodNameAdminExportChain, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameAdminExportChain, "admin")
				}},
				{Name: MethodNameAdminImportChain, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameAdminImportChain, "admin")
				}},
				{Name: MethodNameAdminNodeInfo, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameAdminNodeInfo, "admin")
				}},
				{Name: MethodNameAdminPeerEvents, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameAdminPeerEvents, "admin")
				}},
				{Name: MethodNameAdminPeers, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameAdminPeers, "admin")
				}},
				{Name: MethodNameAdminRemovePeer, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameAdminRemovePeer, "admin")
				}},
				{Name: MethodNameAdminRemoveTrustedPeer, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameAdminRemoveTrustedPeer, "admin")
				}},
				{Name: MethodNameAdminStartHTTP, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameAdminStartHTTP, "admin")
				}},
				{Name: MethodNameAdminStartWS, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameAdminStartWS, "admin")
				}},
				{Name: MethodNameAdminStopHTTP, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameAdminStopHTTP, "admin")
				}},
				{Name: MethodNameAdminStopWS, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameAdminStopWS, "admin")
				}},
			},
		},
		{
			Name:        "les",
			Description: "LES namespace methods (Light Ethereum Subprotocol)",
			Methods: []types.TestMethod{
				// Test all LES methods to see if they're implemented
				{Name: MethodNameLesServerInfo, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameLesServerInfo, "les")
				}},
				{Name: MethodNameLesClientInfo, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameLesClientInfo, "les")
				}},
				{Name: MethodNameLesPriorityClientInfo, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameLesPriorityClientInfo, "les")
				}},
				{Name: MethodNameLesAddBalance, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameLesAddBalance, "les")
				}},
				{Name: MethodNameLesSetClientParams, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameLesSetClientParams, "les")
				}},
				{Name: MethodNameLesSetDefaultParams, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameLesSetDefaultParams, "les")
				}},
				{Name: MethodNameLesLatestCheckpoint, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameLesLatestCheckpoint, "les")
				}},
				{Name: MethodNameLesGetCheckpoint, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameLesGetCheckpoint, "les")
				}},
				{Name: MethodNameLesGetCheckpointContractAddress, Handler: func(rCtx *RpcContext) (*types.RpcResult, error) {
					return GenericTest(rCtx, MethodNameLesGetCheckpointContractAddress, "les")
				}},
			},
		},
	}
}