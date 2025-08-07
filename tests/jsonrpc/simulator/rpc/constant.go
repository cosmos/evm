package rpc

import (
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

const (
	// Web3 namespace
	MethodNameWeb3ClientVersion types.RpcName = "web3_clientVersion"
	MethodNameWeb3Sha3          types.RpcName = "web3_sha3"

	// Net namespace
	MethodNameNetVersion   types.RpcName = "net_version"
	MethodNameNetPeerCount types.RpcName = "net_peerCount"
	MethodNameNetListening types.RpcName = "net_listening"

	// Eth namespace - client subcategory
	MethodNameEthChainId     types.RpcName = "eth_chainId"
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

	// Personal namespace (deprecated in favor of Clef)
	MethodNamePersonalListAccounts     types.RpcName = "personal_listAccounts"
	MethodNamePersonalDeriveAccount    types.RpcName = "personal_deriveAccount"
	MethodNamePersonalEcRecover        types.RpcName = "personal_ecRecover"
	MethodNamePersonalImportRawKey     types.RpcName = "personal_importRawKey"
	MethodNamePersonalListWallets      types.RpcName = "personal_listWallets"
	MethodNamePersonalNewAccount       types.RpcName = "personal_newAccount"
	MethodNamePersonalOpenWallet       types.RpcName = "personal_openWallet"
	MethodNamePersonalSendTransaction  types.RpcName = "personal_sendTransaction"
	MethodNamePersonalSign             types.RpcName = "personal_sign"
	MethodNamePersonalSignTransaction  types.RpcName = "personal_signTransaction"
	MethodNamePersonalSignTypedData    types.RpcName = "personal_signTypedData"
	MethodNamePersonalUnlockAccount    types.RpcName = "personal_unlockAccount"
	MethodNamePersonalLockAccount      types.RpcName = "personal_lockAccount"
	MethodNamePersonalUnpair           types.RpcName = "personal_unpair"
	MethodNamePersonalInitializeWallet types.RpcName = "personal_initializeWallet"

	// Miner namespace (deprecated)
	MethodNameMinerStart        types.RpcName = "miner_start"
	MethodNameMinerStop         types.RpcName = "miner_stop"
	MethodNameMinerSetEtherbase types.RpcName = "miner_setEtherbase"
	MethodNameMinerSetExtra     types.RpcName = "miner_setExtra"
	MethodNameMinerSetGasPrice  types.RpcName = "miner_setGasPrice"
	MethodNameMinerSetGasLimit  types.RpcName = "miner_setGasLimit"
	MethodNameMinerGetHashrate  types.RpcName = "miner_getHashrate"

	// TxPool namespace
	MethodNameTxPoolContent     types.RpcName = "txpool_content"
	MethodNameTxPoolContentFrom types.RpcName = "txpool_contentFrom"
	MethodNameTxPoolInspect     types.RpcName = "txpool_inspect"
	MethodNameTxPoolStatus      types.RpcName = "txpool_status"

	// Debug namespace - tracing subcategory
	MethodNameDebugTraceTransaction  types.RpcName = "debug_traceTransaction"
	MethodNameDebugTraceBlock        types.RpcName = "debug_traceBlock"
	MethodNameDebugTraceBlockByHash  types.RpcName = "debug_traceBlockByHash"
	MethodNameDebugTraceBlockByNumber types.RpcName = "debug_traceBlockByNumber"
	MethodNameDebugTraceCall         types.RpcName = "debug_traceCall"
	MethodNameDebugIntermediateRoots types.RpcName = "debug_intermediateRoots"

	// Debug namespace - database subcategory
	MethodNameDebugDbGet               types.RpcName = "debug_dbGet"
	MethodNameDebugDbAncient           types.RpcName = "debug_dbAncient"
	MethodNameDebugChaindbCompact      types.RpcName = "debug_chaindbCompact"
	MethodNameDebugGetModifiedAccounts types.RpcName = "debug_getModifiedAccounts"
	MethodNameDebugDumpBlock           types.RpcName = "debug_dumpBlock"

	// Debug namespace - profiling subcategory
	MethodNameDebugBlockProfile            types.RpcName = "debug_blockProfile"
	MethodNameDebugCpuProfile              types.RpcName = "debug_cpuProfile"
	MethodNameDebugGoTrace                 types.RpcName = "debug_goTrace"
	MethodNameDebugMemStats                types.RpcName = "debug_memStats"
	MethodNameDebugMutexProfile            types.RpcName = "debug_mutexProfile"
	MethodNameDebugSetBlockProfileRate     types.RpcName = "debug_setBlockProfileRate"
	MethodNameDebugSetMutexProfileFraction types.RpcName = "debug_setMutexProfileFraction"

	// Debug namespace - diagnostics subcategory
	MethodNameDebugBacktraceAt  types.RpcName = "debug_backtraceAt"
	MethodNameDebugStacks       types.RpcName = "debug_stacks"
	MethodNameDebugGetBadBlocks types.RpcName = "debug_getBadBlocks"
	MethodNameDebugPreimage     types.RpcName = "debug_preimage"
	MethodNameDebugFreeOSMemory types.RpcName = "debug_freeOSMemory"
	MethodNameDebugSetHead      types.RpcName = "debug_setHead"

	// Additional debug methods from Geth documentation
	MethodNameDebugSetGCPercent                types.RpcName = "debug_setGCPercent"
	MethodNameDebugAccountRange                types.RpcName = "debug_accountRange"
	MethodNameDebugChaindbProperty             types.RpcName = "debug_chaindbProperty"
	MethodNameDebugDbAncients                  types.RpcName = "debug_dbAncients"
	MethodNameDebugFreezeClient                types.RpcName = "debug_freezeClient"
	MethodNameDebugGcStats                     types.RpcName = "debug_gcStats"
	MethodNameDebugGetAccessibleState          types.RpcName = "debug_getAccessibleState"
	MethodNameDebugGetRawBlock                 types.RpcName = "debug_getRawBlock"
	MethodNameDebugGetRawHeader                types.RpcName = "debug_getRawHeader"
	MethodNameDebugGetRawTransaction           types.RpcName = "debug_getRawTransaction"
	MethodNameDebugGetModifiedAccountsByHash   types.RpcName = "debug_getModifiedAccountsByHash"
	MethodNameDebugGetModifiedAccountsByNumber types.RpcName = "debug_getModifiedAccountsByNumber"
	MethodNameDebugGetRawReceipts              types.RpcName = "debug_getRawReceipts"
	MethodNameDebugPrintBlock                  types.RpcName = "debug_printBlock"

	// Engine API namespace (not applicable for Cosmos chains)
	MethodNameEngineNewPayloadV1        types.RpcName = "engine_newPayloadV1"
	MethodNameEngineNewPayloadV2        types.RpcName = "engine_newPayloadV2"
	MethodNameEngineNewPayloadV3        types.RpcName = "engine_newPayloadV3"
	MethodNameEngineForkchoiceUpdatedV1 types.RpcName = "engine_forkchoiceUpdatedV1"
	MethodNameEngineForkchoiceUpdatedV2 types.RpcName = "engine_forkchoiceUpdatedV2"
	MethodNameEngineForkchoiceUpdatedV3 types.RpcName = "engine_forkchoiceUpdatedV3"
	MethodNameEngineGetPayloadV1        types.RpcName = "engine_getPayloadV1"
	MethodNameEngineGetPayloadV2        types.RpcName = "engine_getPayloadV2"
	MethodNameEngineGetPayloadV3        types.RpcName = "engine_getPayloadV3"

	// Trace namespace (OpenEthereum/Erigon specific, not in standard execution-apis)
	MethodNameTraceCall        types.RpcName = "trace_call"
	MethodNameTraceCallMany    types.RpcName = "trace_callMany"
	MethodNameTraceTransaction types.RpcName = "trace_transaction"
	MethodNameTraceBlock       types.RpcName = "trace_block"

	// Admin namespace (Geth specific administrative methods)
	MethodNameAdminAddPeer           types.RpcName = "admin_addPeer"
	MethodNameAdminAddTrustedPeer    types.RpcName = "admin_addTrustedPeer"
	MethodNameAdminDatadir           types.RpcName = "admin_datadir"
	MethodNameAdminExportChain       types.RpcName = "admin_exportChain"
	MethodNameAdminImportChain       types.RpcName = "admin_importChain"
	MethodNameAdminNodeInfo          types.RpcName = "admin_nodeInfo"
	MethodNameAdminPeerEvents        types.RpcName = "admin_peerEvents"
	MethodNameAdminPeers             types.RpcName = "admin_peers"
	MethodNameAdminRemovePeer        types.RpcName = "admin_removePeer"
	MethodNameAdminRemoveTrustedPeer types.RpcName = "admin_removeTrustedPeer"
	MethodNameAdminStartHTTP         types.RpcName = "admin_startHTTP"
	MethodNameAdminStartWS           types.RpcName = "admin_startWS"
	MethodNameAdminStopHTTP          types.RpcName = "admin_stopHTTP"
	MethodNameAdminStopWS            types.RpcName = "admin_stopWS"

	// LES namespace (Light Ethereum Subprotocol)
	MethodNameLesServerInfo                   types.RpcName = "les_serverInfo"
	MethodNameLesClientInfo                   types.RpcName = "les_clientInfo"
	MethodNameLesPriorityClientInfo           types.RpcName = "les_priorityClientInfo"
	MethodNameLesAddBalance                   types.RpcName = "les_addBalance"
	MethodNameLesSetClientParams              types.RpcName = "les_setClientParams"
	MethodNameLesSetDefaultParams             types.RpcName = "les_setDefaultParams"
	MethodNameLesLatestCheckpoint             types.RpcName = "les_latestCheckpoint"
	MethodNameLesGetCheckpoint                types.RpcName = "les_getCheckpoint"
	MethodNameLesGetCheckpointContractAddress types.RpcName = "les_getCheckpointContractAddress"
)
