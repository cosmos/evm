package namespaces

import "github.com/cosmos/evm/tests/jsonrpc/simulator/types"

const (
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
