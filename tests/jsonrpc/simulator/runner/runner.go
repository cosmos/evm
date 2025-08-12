package runner

import (
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/utils"
)

// ExecuteAllTests runs all RPC tests and returns the results
func ExecuteAllTests(rCtx *types.RPCContext) []*types.RpcResult {
	var results []*types.RpcResult

	// Get test categories
	testCategories := GetTestCases()

	// Execute tests by category
	for _, category := range testCategories {
		for _, method := range category.Methods {
			if method.Handler == nil {
				// Handle methods with no handler - only skip engine methods, test others
				if category.Name == "engine" {
					result, _ := utils.Skip(method.Name, category.Name, method.SkipReason)
					if result != nil {
						result.Description = method.Description
					}
					results = append(results, result)
				} else {
					// Test the method to see if it's actually implemented
					result, _ := utils.CallEthClient(rCtx, method.Name, category.Name)
					if result != nil {
						result.Description = method.Description
					}
					results = append(results, result)
				}
				continue
			}

			// Execute the test
			handler := method.Handler.(func(*types.RPCContext) (*types.RpcResult, error))
			result, err := handler(rCtx)
			if err != nil {
				result = &types.RpcResult{
					Method:      method.Name,
					Status:      types.Error,
					ErrMsg:      err.Error(),
					Category:    category.Name,
					Description: method.Description,
				}
			}
			// Ensure category and description are set
			if result.Category == "" {
				result.Category = category.Name
			}
			if result.Description == "" {
				result.Description = method.Description
			}

			results = append(results, result)
		}
	}

	return results
}
