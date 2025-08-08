package rpc

import (
	"strings"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

// ExecuteAllTests runs all RPC tests and returns the results
func ExecuteAllTests(rCtx *RpcContext) []*types.RpcResult {
	var results []*types.RpcResult

	// Get test categories
	testCategories := GetTestCategories()

	// Execute tests by category
	for _, category := range testCategories {
		for _, method := range category.Methods {
			if method.Handler == nil {
				// Handle methods with no handler - only skip engine methods, test others
				if category.Name == "engine" {
					result, _ := Skipped(method.Name, category.Name, method.SkipReason)
					if result != nil {
						result.Description = method.Description
					}
					results = append(results, result)
				} else {
					// Test the method to see if it's actually implemented
					result, _ := GenericTest(rCtx, method.Name, category.Name)
					if result != nil {
						result.Description = method.Description
					}
					results = append(results, result)
				}
				continue
			}

			// Execute the test
			handler := method.Handler.(func(*RpcContext) (*types.RpcResult, error))
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

	// Add results from transaction tests that were automatically added (avoid duplicates)
	alreadyTested := make(map[types.RpcName]bool)
	for _, result := range results {
		alreadyTested[result.Method] = true
	}

	for _, result := range rCtx.AlreadyTestedRPCs {
		// Skip if we already tested this method in the categorized tests
		if alreadyTested[result.Method] {
			continue
		}

		if result.Category == "" {
			// Categorize based on method name using the namespace
			result.Category = categorizeMethodByNamespace(string(result.Method))
		}
		results = append(results, result)
	}

	return results
}

// categorizeMethodByNamespace categorizes RPC methods based on their namespace prefix
func categorizeMethodByNamespace(methodStr string) string {
	if strings.HasPrefix(methodStr, "eth_") {
		return "eth"
	} else if strings.HasPrefix(methodStr, "web3_") {
		return "web3"
	} else if strings.HasPrefix(methodStr, "net_") {
		return "net"
	} else if strings.HasPrefix(methodStr, "personal_") {
		return "personal"
	} else if strings.HasPrefix(methodStr, "debug_") {
		return "debug"
	} else if strings.HasPrefix(methodStr, "txpool_") {
		return "txpool"
	} else if strings.HasPrefix(methodStr, "miner_") {
		return "miner"
	} else if strings.HasPrefix(methodStr, "admin_") {
		return "admin"
	} else if strings.HasPrefix(methodStr, "engine_") {
		return "engine"
	} else if strings.HasPrefix(methodStr, "les_") {
		return "les"
	} else {
		return "Uncategorized"
	}
}