package rpc

import (
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

// Personal method handlers
func PersonalListAccounts(rCtx *RpcContext) (*types.RpcResult, error) {
	var result []string
	err := rCtx.EthCli.Client().Call(&result, "personal_listAccounts")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNamePersonalListAccounts,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "personal",
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNamePersonalListAccounts,
		Status:   types.Ok,
		Value:    result,
		Category: "personal",
	}, nil
}

func PersonalEcRecover(rCtx *RpcContext) (*types.RpcResult, error) {
	// Test with known data
	var result string
	err := rCtx.EthCli.Client().Call(&result, "personal_ecRecover",
		"0xdeadbeaf",
		"0xf9ff74c86aefeb5f6019d77280bbb44fb695b4d45cfe97e6eed7acd62905f4a85034d5c68ed25a2e7a8eeb9baf1b8401e4f865d92ec48c1763bf649e354d900b1c")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNamePersonalEcRecover,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "personal",
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNamePersonalEcRecover,
		Status:   types.Ok,
		Value:    result,
		Category: "personal",
	}, nil
}

func PersonalListWallets(rCtx *RpcContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, "personal_listWallets")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNamePersonalListWallets,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "personal",
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNamePersonalListWallets,
		Status:   types.Ok,
		Value:    result,
		Category: "personal",
	}, nil
}
