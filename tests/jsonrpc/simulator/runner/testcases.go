package runner

import (
	ns "github.com/cosmos/evm/tests/jsonrpc/simulator/namespaces"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/utils"
)

// GetTestCase returns the comprehensive test configuration organized by namespace
// based on the execution-apis structure
func GetTestCases() []types.TestCase {
	return []types.TestCase{
		{
			Name:        "web3",
			Description: "Web3 namespace utility methods",
			Methods: []types.TestMethod{
				{Name: ns.MethodNameWeb3ClientVersion, Handler: ns.Web3ClientVersion},
				{Name: ns.MethodNameWeb3Sha3, Handler: ns.Web3Sha3},
			},
		},
		{
			Name:        "net",
			Description: "Net namespace network methods",
			Methods: []types.TestMethod{
				{Name: ns.MethodNameNetVersion, Handler: ns.NetVersion},
				{Name: ns.MethodNameNetPeerCount, Handler: ns.NetPeerCount},
				{Name: ns.MethodNameNetListening, Handler: ns.NetListening},
			},
		},
		{
			Name:        "eth",
			Description: "Ethereum namespace methods from execution-apis",
			Methods: []types.TestMethod{
				// Client subcategory
				{Name: ns.MethodNameEthChainID, Handler: ns.EthChainID},
				{Name: ns.MethodNameEthSyncing, Handler: ns.EthSyncing},
				{Name: ns.MethodNameEthCoinbase, Handler: ns.EthCoinbase},
				{Name: ns.MethodNameEthAccounts, Handler: ns.EthAccounts},
				{Name: ns.MethodNameEthBlockNumber, Handler: ns.EthBlockNumber},
				{Name: ns.MethodNameEthMining, Handler: ns.EthMining},
				{Name: ns.MethodNameEthHashrate, Handler: ns.EthHashrate},
				// Fee market subcategory
				{Name: ns.MethodNameEthGasPrice, Handler: ns.EthGasPrice},
				{Name: ns.MethodNameEthMaxPriorityFeePerGas, Handler: ns.EthMaxPriorityFeePerGas},
				// State subcategory
				{Name: ns.MethodNameEthGetBalance, Handler: ns.EthGetBalance},
				{Name: ns.MethodNameEthGetTransactionCount, Handler: ns.EthGetTransactionCount},
				{Name: ns.MethodNameEthGetCode, Handler: ns.EthGetCode},
				{Name: ns.MethodNameEthGetStorageAt, Handler: ns.EthGetStorageAt},
				// Block subcategory
				{Name: ns.MethodNameEthGetBlockByHash, Handler: ns.EthGetBlockByHash},
				{Name: ns.MethodNameEthGetBlockByNumber, Handler: ns.EthGetBlockByNumber},
				{Name: ns.MethodNameEthGetBlockTransactionCountByHash, Handler: ns.EthGetBlockTransactionCountByHash},
				{Name: ns.MethodNameEthGetBlockReceipts, Handler: ns.EthGetBlockReceipts},
				{Name: ns.MethodNameEthGetHeaderByHash, Handler: ns.EthGetHeaderByHash},
				{Name: ns.MethodNameEthGetHeaderByNumber, Handler: ns.EthGetHeaderByNumber},
				// Uncle subcategory (uncles don't exist in CometBFT, should return 0/nil)
				{Name: ns.MethodNameEthGetUncleCountByBlockHash, Handler: ns.EthGetUncleCountByBlockHash},
				{Name: ns.MethodNameEthGetUncleCountByBlockNumber, Handler: ns.EthGetUncleCountByBlockNumber},
				{Name: ns.MethodNameEthGetUncleByBlockHashAndIndex, Handler: ns.EthGetUncleByBlockHashAndIndex},
				{Name: ns.MethodNameEthGetUncleByBlockNumberAndIndex, Handler: ns.EthGetUncleByBlockNumberAndIndex},
				// Transaction subcategory
				{Name: ns.MethodNameEthGetTransactionByHash, Handler: ns.EthGetTransactionByHash},
				{Name: ns.MethodNameEthGetTransactionByBlockHashAndIndex, Handler: ns.EthGetTransactionByBlockHashAndIndex},
				{Name: ns.MethodNameEthGetTransactionByBlockNumberAndIndex, Handler: ns.EthGetTransactionByBlockNumberAndIndex},
				{Name: ns.MethodNameEthGetTransactionReceipt, Handler: ns.EthGetTransactionReceipt},
				{Name: ns.MethodNameEthGetBlockTransactionCountByNumber, Handler: ns.EthGetBlockTransactionCountByNumber},
				{Name: ns.MethodNameEthGetPendingTransactions, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.Legacy(rCtx, ns.MethodNameEthGetPendingTransactions, "eth", "Use eth_newPendingTransactionFilter + eth_getFilterChanges instead")
				}},
				{Name: ns.MethodNameEthCreateAccessList, Handler: ns.EthCreateAccessList},
				{Name: ns.MethodNameEthPendingTransactions, Handler: ns.EthPendingTransactions, Description: "Go-ethereum compatible pending transactions method"},
				// Execute subcategory
				{Name: ns.MethodNameEthCall, Handler: ns.EthCall},
				{Name: ns.MethodNameEthEstimateGas, Handler: ns.EthEstimateGas},
				{Name: ns.MethodNameEthSimulateV1, Handler: ns.EthSimulateV1},
				// Submit subcategory
				{Name: ns.MethodNameEthSendRawTransaction, Handler: ns.EthSendRawTransaction, Description: "Combined test: Transfer value, Deploy contract, Transfer ERC20"},
				// Filter subcategory
				{Name: ns.MethodNameEthNewFilter, Handler: ns.EthNewFilter},
				{Name: ns.MethodNameEthGetFilterLogs, Handler: ns.EthGetFilterLogs},
				{Name: ns.MethodNameEthNewBlockFilter, Handler: ns.EthNewBlockFilter},
				{Name: ns.MethodNameEthNewPendingTransactionFilter, Handler: nil},
				{Name: ns.MethodNameEthGetFilterChanges, Handler: ns.EthGetFilterChanges},
				{Name: ns.MethodNameEthUninstallFilter, Handler: ns.EthUninstallFilter},
				{Name: ns.MethodNameEthGetLogs, Handler: ns.EthGetLogs},
				// Other/not implemented methods
				{Name: ns.MethodNameEthBlobBaseFee, Handler: nil, SkipReason: "EIP-4844 blob base fee (post-Cancun)"},
				{Name: ns.MethodNameEthFeeHistory, Handler: ns.EthFeeHistory},
				{Name: ns.MethodNameEthGetProof, Handler: ns.EthGetProof},
				{Name: ns.MethodNameEthProtocolVersion, Handler: nil, SkipReason: "Protocol version deprecated"},
				// Standard methods that should be implemented
				{Name: ns.MethodNameEthSendTransaction, Handler: ns.EthSendTransaction},
				{Name: ns.MethodNameEthSign, Handler: ns.EthSign},
				{Name: ns.MethodNameEthSignTransaction, Handler: nil},
				// WebSocket subscription methods (part of eth namespace)
				{Name: ns.MethodNameEthSubscribe, Handler: ns.EthSubscribe, Description: "WebSocket subscription with all 4 subscription types: newHeads, logs, newPendingTransactions, syncing"},
				{Name: ns.MethodNameEthUnsubscribe, Handler: ns.EthUnsubscribe, Description: "WebSocket unsubscription functionality"},
			},
		},
		{
			Name:        "personal",
			Description: "Personal namespace methods (deprecated in favor of Clef)",
			Methods: []types.TestMethod{
				// Account Management subcategory
				{Name: ns.MethodNamePersonalListAccounts, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.Legacy(rCtx, ns.MethodNamePersonalListAccounts, "personal", "Personal namespace deprecated - use external signers like Clef")
				}},
				{Name: ns.MethodNamePersonalNewAccount, Handler: ns.PersonalNewAccount},
				{Name: ns.MethodNamePersonalDeriveAccount, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNamePersonalDeriveAccount, "personal")
				}},
				// Wallet Management subcategory
				{Name: ns.MethodNamePersonalListWallets, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.Legacy(rCtx, ns.MethodNamePersonalListWallets, "personal", "Personal namespace deprecated - use external signers like Clef")
				}},
				{Name: ns.MethodNamePersonalOpenWallet, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNamePersonalOpenWallet, "personal")
				}},
				{Name: ns.MethodNamePersonalInitializeWallet, Handler: func(_ *types.RPCContext) (*types.RpcResult, error) {
					return utils.Skip(ns.MethodNamePersonalInitializeWallet, "personal", "Cosmos EVM always returns false for personal namespace methods")
				}},
				{Name: ns.MethodNamePersonalUnpair, Handler: func(_ *types.RPCContext) (*types.RpcResult, error) {
					return utils.Skip(ns.MethodNamePersonalUnpair, "personal", "Cosmos EVM always returns false for personal namespace methods")
				}},
				// Key Management subcategory
				{Name: ns.MethodNamePersonalImportRawKey, Handler: ns.PersonalImportRawKey},
				{Name: ns.MethodNamePersonalUnlockAccount, Handler: func(_ *types.RPCContext) (*types.RpcResult, error) {
					return utils.Skip(ns.MethodNamePersonalUnlockAccount, "personal", "Cosmos EVM always returns false for personal namespace methods")
				}},
				{Name: ns.MethodNamePersonalLockAccount, Handler: func(_ *types.RPCContext) (*types.RpcResult, error) {
					return utils.Skip(ns.MethodNamePersonalLockAccount, "personal", "Cosmos EVM always returns false for personal namespace methods")
				}},
				// Signing subcategory
				{Name: ns.MethodNamePersonalSign, Handler: ns.PersonalSign},
				{Name: ns.MethodNamePersonalSignTransaction, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNamePersonalSignTransaction, "personal")
				}},
				{Name: ns.MethodNamePersonalSignTypedData, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNamePersonalSignTypedData, "personal")
				}},
				{Name: ns.MethodNamePersonalEcRecover, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.Legacy(rCtx, ns.MethodNamePersonalEcRecover, "personal", "Personal namespace deprecated - use external signers like Clef")
				}},
				// Transaction subcategory
				{Name: ns.MethodNamePersonalSendTransaction, Handler: ns.PersonalSendTransaction},
			},
		},
		{
			Name:        "miner",
			Description: "Miner namespace methods (deprecated)",
			Methods: []types.TestMethod{
				{Name: ns.MethodNameMinerStart, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: ns.MethodNameMinerStop, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: ns.MethodNameMinerSetEtherbase, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: ns.MethodNameMinerSetExtra, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: ns.MethodNameMinerSetGasPrice, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: ns.MethodNameMinerSetGasLimit, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
				{Name: ns.MethodNameMinerGetHashrate, Handler: nil, SkipReason: "Mining deprecated in Ethereum 2.0"},
			},
		},
		{
			Name:        "txpool",
			Description: "TxPool namespace methods",
			Methods: []types.TestMethod{
				{Name: ns.MethodNameTxPoolContent, Handler: ns.TxPoolContent},
				{Name: ns.MethodNameTxPoolContentFrom, Handler: ns.TxPoolContentFrom},
				{Name: ns.MethodNameTxPoolInspect, Handler: ns.TxPoolInspect},
				{Name: ns.MethodNameTxPoolStatus, Handler: ns.TxPoolStatus},
			},
		},
		{
			Name:        "debug",
			Description: "Debug namespace methods from Geth",
			Methods: []types.TestMethod{
				// Tracing subcategory
				{Name: ns.MethodNameDebugTraceTransaction, Handler: ns.DebugTraceTransaction},
				{Name: ns.MethodNameDebugTraceBlock, Handler: ns.DebugTraceBlock},
				{Name: ns.MethodNameDebugTraceBlockByHash, Handler: ns.DebugTraceBlockByHash},
				{Name: ns.MethodNameDebugTraceBlockByNumber, Handler: ns.DebugTraceBlockByNumber},
				{Name: ns.MethodNameDebugTraceCall, Handler: ns.DebugTraceCall},
				{Name: ns.MethodNameDebugIntermediateRoots, Handler: ns.DebugIntermediateRoots},
				// Database subcategory
				{Name: ns.MethodNameDebugDbGet, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugDbGet, "debug")
				}},
				{Name: ns.MethodNameDebugDbAncient, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugDbAncient, "debug")
				}},
				{Name: ns.MethodNameDebugDbAncients, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugDbAncients, "debug")
				}},
				{Name: ns.MethodNameDebugChaindbCompact, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugChaindbCompact, "debug")
				}},
				{Name: ns.MethodNameDebugChaindbProperty, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugChaindbProperty, "debug")
				}},
				{Name: ns.MethodNameDebugGetModifiedAccounts, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugGetModifiedAccounts, "debug")
				}},
				{Name: ns.MethodNameDebugGetModifiedAccountsByHash, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugGetModifiedAccountsByHash, "debug")
				}},
				{Name: ns.MethodNameDebugGetModifiedAccountsByNumber, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugGetModifiedAccountsByNumber, "debug")
				}},
				{Name: ns.MethodNameDebugDumpBlock, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugDumpBlock, "debug")
				}},
				// Profiling subcategory
				{Name: ns.MethodNameDebugBlockProfile, Handler: ns.DebugBlockProfile},
				{Name: ns.MethodNameDebugCPUProfile, Handler: ns.DebugCPUProfile},
				{Name: ns.MethodNameDebugGoTrace, Handler: ns.DebugGoTrace},
				{Name: ns.MethodNameDebugMemStats, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugMemStats, "debug")
				}},
				{Name: ns.MethodNameDebugMutexProfile, Handler: ns.DebugMutexProfile},
				{Name: ns.MethodNameDebugSetBlockProfileRate, Handler: ns.DebugSetBlockProfileRate},
				{Name: ns.MethodNameDebugSetMutexProfileFraction, Handler: ns.DebugSetMutexProfileFraction},
				{Name: ns.MethodNameDebugGcStats, Handler: ns.DebugGcStats},
				// Diagnostics subcategory
				{Name: ns.MethodNameDebugBacktraceAt, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugBacktraceAt, "debug")
				}},
				{Name: ns.MethodNameDebugStacks, Handler: ns.DebugStacks},
				{Name: ns.MethodNameDebugGetBadBlocks, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugGetBadBlocks, "debug")
				}},
				{Name: ns.MethodNameDebugPreimage, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugPreimage, "debug")
				}},
				{Name: ns.MethodNameDebugFreeOSMemory, Handler: ns.DebugFreeOSMemory},
				{Name: ns.MethodNameDebugSetHead, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugSetHead, "debug")
				}},
				{Name: ns.MethodNameDebugGetAccessibleState, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugGetAccessibleState, "debug")
				}},
				{Name: ns.MethodNameDebugFreezeClient, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugFreezeClient, "debug")
				}},
				// New debug methods (including debug_setGCPercent)
				{Name: ns.MethodNameDebugSetGCPercent, Handler: ns.DebugSetGCPercent},
				{Name: ns.MethodNameDebugAccountRange, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugAccountRange, "debug")
				}},
				{Name: ns.MethodNameDebugGetRawBlock, Handler: ns.DebugGetRawBlock},
				{Name: ns.MethodNameDebugGetRawHeader, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugGetRawHeader, "debug")
				}},
				{Name: ns.MethodNameDebugGetRawTransaction, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugGetRawTransaction, "debug")
				}},
				{Name: ns.MethodNameDebugGetRawReceipts, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugGetRawReceipts, "debug")
				}},
				{Name: ns.MethodNameDebugPrintBlock, Handler: ns.DebugPrintBlock},
				// Additional debug methods from Geth documentation
				{Name: ns.MethodNameDebugStartCPUProfile, Handler: ns.DebugStartCPUProfile, Description: "Start CPU profiling"},
				{Name: ns.MethodNameDebugStopCPUProfile, Handler: ns.DebugStopCPUProfile, Description: "Stop CPU profiling"},
				{Name: ns.MethodNameDebugStartGoTrace, Handler: ns.DebugStartGoTrace, Description: "Start Go execution tracing"},
				{Name: ns.MethodNameDebugStopGoTrace, Handler: ns.DebugStopGoTrace, Description: "Stop Go execution tracing"},
				{Name: ns.MethodNameDebugTraceBadBlock, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugTraceBadBlock, "debug")
				}, Description: "Trace bad blocks"},
				{Name: ns.MethodNameDebugStandardTraceBlockToFile, Handler: ns.DebugStandardTraceBlockToFile, Description: "Standard trace block to file"},
				{Name: ns.MethodNameDebugStandardTraceBadBlockToFile, Handler: ns.DebugStandardTraceBadBlockToFile, Description: "Standard trace bad block to file"},
				{Name: ns.MethodNameDebugTraceBlockFromFile, Handler: ns.DebugTraceBlockFromFile, Description: "Trace block from file"},
				{Name: ns.MethodNameDebugTraceChain, Handler: ns.DebugTraceChain, Description: "Trace a range of blocks in the chain"},
				{Name: ns.MethodNameDebugStorageRangeAt, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugStorageRangeAt, "debug")
				}, Description: "Get storage range at specific position"},
				{Name: ns.MethodNameDebugSetTrieFlushInterval, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugSetTrieFlushInterval, "debug")
				}, Description: "Set trie flush interval"},
				{Name: ns.MethodNameDebugVmodule, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugVmodule, "debug")
				}, Description: "Set logging verbosity pattern"},
				{Name: ns.MethodNameDebugWriteBlockProfile, Handler: ns.DebugWriteBlockProfile, Description: "Write block profile to file"},
				{Name: ns.MethodNameDebugWriteMemProfile, Handler: ns.DebugWriteMemProfile, Description: "Write memory profile to file"},
				{Name: ns.MethodNameDebugWriteMutexProfile, Handler: ns.DebugWriteMutexProfile, Description: "Write mutex profile to file"},
				{Name: ns.MethodNameDebugVerbosity, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameDebugVerbosity, "debug")
				}, Description: "Set log verbosity level"},
			},
		},
		{
			Name:        "engine",
			Description: "Engine API methods (not applicable for Cosmos chains)",
			Methods: []types.TestMethod{
				{Name: ns.MethodNameEngineNewPayloadV1, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: ns.MethodNameEngineForkchoiceUpdatedV1, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: ns.MethodNameEngineGetPayloadV1, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: ns.MethodNameEngineNewPayloadV2, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: ns.MethodNameEngineForkchoiceUpdatedV2, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
				{Name: ns.MethodNameEngineGetPayloadV2, Handler: nil, SkipReason: "Not applicable for Cosmos chains using CometBFT"},
			},
		},
		{
			Name:        "admin",
			Description: "Admin namespace methods (Geth administrative)",
			Methods: []types.TestMethod{
				// Test all admin methods to see if they're implemented
				{Name: ns.MethodNameAdminAddPeer, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameAdminAddPeer, "admin")
				}},
				{Name: ns.MethodNameAdminAddTrustedPeer, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameAdminAddTrustedPeer, "admin")
				}},
				{Name: ns.MethodNameAdminDatadir, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameAdminDatadir, "admin")
				}},
				{Name: ns.MethodNameAdminExportChain, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameAdminExportChain, "admin")
				}},
				{Name: ns.MethodNameAdminImportChain, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameAdminImportChain, "admin")
				}},
				{Name: ns.MethodNameAdminNodeInfo, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameAdminNodeInfo, "admin")
				}},
				{Name: ns.MethodNameAdminPeerEvents, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameAdminPeerEvents, "admin")
				}},
				{Name: ns.MethodNameAdminPeers, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameAdminPeers, "admin")
				}},
				{Name: ns.MethodNameAdminRemovePeer, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameAdminRemovePeer, "admin")
				}},
				{Name: ns.MethodNameAdminRemoveTrustedPeer, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameAdminRemoveTrustedPeer, "admin")
				}},
				{Name: ns.MethodNameAdminStartHTTP, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameAdminStartHTTP, "admin")
				}},
				{Name: ns.MethodNameAdminStartWS, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameAdminStartWS, "admin")
				}},
				{Name: ns.MethodNameAdminStopHTTP, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameAdminStopHTTP, "admin")
				}},
				{Name: ns.MethodNameAdminStopWS, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameAdminStopWS, "admin")
				}},
			},
		},
		{
			Name:        "les",
			Description: "LES namespace methods (Light Ethereum Subprotocol)",
			Methods: []types.TestMethod{
				// Test all LES methods to see if they're implemented
				{Name: ns.MethodNameLesServerInfo, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameLesServerInfo, "les")
				}},
				{Name: ns.MethodNameLesClientInfo, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameLesClientInfo, "les")
				}},
				{Name: ns.MethodNameLesPriorityClientInfo, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameLesPriorityClientInfo, "les")
				}},
				{Name: ns.MethodNameLesAddBalance, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameLesAddBalance, "les")
				}},
				{Name: ns.MethodNameLesSetClientParams, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameLesSetClientParams, "les")
				}},
				{Name: ns.MethodNameLesSetDefaultParams, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameLesSetDefaultParams, "les")
				}},
				{Name: ns.MethodNameLesLatestCheckpoint, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameLesLatestCheckpoint, "les")
				}},
				{Name: ns.MethodNameLesGetCheckpoint, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameLesGetCheckpoint, "les")
				}},
				{Name: ns.MethodNameLesGetCheckpointContractAddress, Handler: func(rCtx *types.RPCContext) (*types.RpcResult, error) {
					return utils.CallEthClient(rCtx, ns.MethodNameLesGetCheckpointContractAddress, "les")
				}},
			},
		},
	}
}
