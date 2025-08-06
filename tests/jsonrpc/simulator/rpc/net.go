package rpc

import (
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

// Net method handlers
func NetVersion(rCtx *RpcContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.EthCli.Client().Call(&result, "net_version")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameNetVersion,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "net",
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameNetVersion,
		Status:   types.Ok,
		Value:    result,
		Category: "net",
	}, nil
}

func NetPeerCount(rCtx *RpcContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.EthCli.Client().Call(&result, "net_peerCount")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameNetPeerCount,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "net",
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameNetPeerCount,
		Status:   types.Ok,
		Value:    result,
		Category: "net",
	}, nil
}

func NetListening(rCtx *RpcContext) (*types.RpcResult, error) {
	var result bool
	err := rCtx.EthCli.Client().Call(&result, "net_listening")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameNetListening,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "net",
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameNetListening,
		Status:   types.Ok,
		Value:    result,
		Category: "net",
	}, nil
}
