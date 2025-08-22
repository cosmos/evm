package namespaces

import "github.com/cosmos/evm/tests/jsonrpc/simulator/types"

const (
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
)
