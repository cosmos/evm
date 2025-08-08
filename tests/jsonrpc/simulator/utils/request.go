package utils

import (
	"fmt"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

// Generic test handler that makes an actual RPC call to determine if an API is implemented
func CallEthClient(rCtx *types.RPCContext, methodName types.RpcName, category string) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, string(methodName))

	if err != nil {
		// Check if it's a "method not found" error (API not implemented)
		if err.Error() == "the method "+string(methodName)+" does not exist/is not available" ||
			err.Error() == "Method not found" ||
			err.Error() == string(methodName)+" method not found" {
			return &types.RpcResult{
				Method:   methodName,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: category,
			}, nil
		}
		// Other errors mean the method exists but failed (could be parameter issues, etc.)
		return &types.RpcResult{
			Method:   methodName,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: category,
		}, nil
	}

	// Method exists and returned a result
	return &types.RpcResult{
		Method:   methodName,
		Status:   types.Ok,
		Value:    result,
		Category: category,
	}, nil
}

func Legacy(rCtx *types.RPCContext, methodName types.RpcName, category string, replacementInfo string) (*types.RpcResult, error) {
	// First test if the API is actually implemented
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, string(methodName))

	if err != nil {
		// Check if it's a "method not found" error (API not implemented)
		if err.Error() == "the method "+string(methodName)+" does not exist/is not available" ||
			err.Error() == "Method not found" ||
			err.Error() == string(methodName)+" method not found" {
			// API is not implemented, so it should be NOT_IMPL, not LEGACY
			return &types.RpcResult{
				Method:   methodName,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: category,
			}, nil
		}
		// API exists but failed with parameters (could be legacy with wrong params)
		// Still mark as legacy since the method exists
	}

	// API exists (either succeeded or failed with parameter issues), mark as LEGACY
	return &types.RpcResult{
		Method:   methodName,
		Status:   types.Legacy,
		Value:    fmt.Sprintf("Legacy API implemented in Cosmos EVM. %s", replacementInfo),
		ErrMsg:   replacementInfo,
		Category: category,
	}, nil
}

func Skip(methodName types.RpcName, category string, reason string) (*types.RpcResult, error) {
	return &types.RpcResult{
		Method:   methodName,
		Status:   types.Skipped,
		ErrMsg:   reason,
		Category: category,
	}, nil
}
