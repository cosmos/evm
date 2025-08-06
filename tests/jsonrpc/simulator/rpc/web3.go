package rpc

import (
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

// Web3 method handlers
func Web3ClientVersion(rCtx *RpcContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.EthCli.Client().Call(&result, "web3_clientVersion")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameWeb3ClientVersion,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "web3",
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameWeb3ClientVersion,
		Status:   types.Ok,
		Value:    result,
		Category: "web3",
	}, nil
}

func Web3Sha3(rCtx *RpcContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.EthCli.Client().Call(&result, "web3_sha3", "0x68656c6c6f20776f726c64")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameWeb3Sha3,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "web3",
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameWeb3Sha3,
		Status:   types.Ok,
		Value:    result,
		Category: "web3",
	}, nil
}
