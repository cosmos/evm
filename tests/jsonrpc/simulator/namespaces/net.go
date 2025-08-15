package namespaces

import (
	"context"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

const (
	NamespaceNet = "net"

	// Net namespace
	MethodNameNetVersion   types.RpcName = "net_version"
	MethodNameNetPeerCount types.RpcName = "net_peerCount"
	MethodNameNetListening types.RpcName = "net_listening"
)

// Net method handlers
func NetVersion(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.Evmd.RPCClient().Call(&result, "net_version")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameNetVersion,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceNet,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameNetVersion,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceNet,
	}, nil
}

func NetPeerCount(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.Evmd.RPCClient().CallContext(context.Background(), &result, "net_peerCount")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameNetPeerCount,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceNet,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameNetPeerCount,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceNet,
	}, nil
}

func NetListening(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result bool
	err := rCtx.Evmd.RPCClient().Call(&result, "net_listening")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameNetListening,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceNet,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameNetListening,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceNet,
	}, nil
}
