package utils

import "github.com/cosmos/evm/tests/jsonrpc/simulator/types"

// Generic test handler that makes an actual RPC call to determine if an API is implemented
func GenericTest(rCtx *types.RPCContext, methodName types.RpcName, category string) (*types.RpcResult, error) {
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
