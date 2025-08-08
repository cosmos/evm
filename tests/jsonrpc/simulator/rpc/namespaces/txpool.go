package namespaces

import (
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

const (
	// TxPool namespace
	MethodNameTxPoolContent     types.RpcName = "txpool_content"
	MethodNameTxPoolContentFrom types.RpcName = "txpool_contentFrom"
	MethodNameTxPoolInspect     types.RpcName = "txpool_inspect"
	MethodNameTxPoolStatus      types.RpcName = "txpool_status"
)

// TxPool method handlers
func TxPoolStatus(rCtx *types.RpcContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, "txpool_status")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameTxPoolStatus,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "txpool",
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameTxPoolStatus,
		Status:   types.Ok,
		Value:    result,
		Category: "txpool",
	}, nil
}

func TxPoolContent(rCtx *types.RpcContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, "txpool_content")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameTxPoolContent,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "txpool",
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameTxPoolContent,
		Status:   types.Ok,
		Value:    result,
		Category: "txpool",
	}, nil
}

func TxPoolInspect(rCtx *types.RpcContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, "txpool_inspect")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameTxPoolInspect,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "txpool",
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameTxPoolInspect,
		Status:   types.Ok,
		Value:    result,
		Category: "txpool",
	}, nil
}

// RpcTxPoolContentFrom returns the transactions pool content for a specific account
func TxPoolContentFrom(rCtx *types.RpcContext) (*types.RpcResult, error) {
	var result interface{}
	// Use a sample address for testing - in real usage this would be parameterized
	testAddress := "0x407d73d8a49eeb85d32cf465507dd71d507100c1"
	err := rCtx.EthCli.Client().Call(&result, "txpool_contentFrom", testAddress)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameTxPoolContentFrom,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "txpool",
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameTxPoolContentFrom,
		Status:   types.Ok,
		Value:    result,
		Category: "txpool",
	}, nil
}
