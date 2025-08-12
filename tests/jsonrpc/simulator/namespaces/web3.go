package namespaces

import (
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

const (
	NamespaceWeb3 = "web3"

	// Web3 namespace
	MethodNameWeb3ClientVersion types.RpcName = "web3_clientVersion"
	MethodNameWeb3Sha3          types.RpcName = "web3_sha3"
)

// Web3 method handlers
func Web3ClientVersion(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.Evmd.RPCClient().Call(&result, "web3_clientVersion")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameWeb3ClientVersion,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceWeb3,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameWeb3ClientVersion,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceWeb3,
	}, nil
}

func Web3Sha3(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.Evmd.RPCClient().Call(&result, "web3_sha3", "0x68656c6c6f20776f726c64")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameWeb3Sha3,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceWeb3,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameWeb3Sha3,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceWeb3,
	}, nil
}
