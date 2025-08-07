package rpc

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

// Debug API implementations
func DebugTraceTransaction(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameDebugTraceTransaction); result != nil {
		return result, nil
	}

	// Need a transaction hash - use one from our processed transactions
	if len(rCtx.ProcessedTransactions) == 0 {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceTransaction,
			Status:   types.Error,
			ErrMsg:   "No processed transactions available for tracing",
			Category: "debug",
		}, nil
	}

	txHash := rCtx.ProcessedTransactions[0]
	
	// Test with callTracer configuration to get structured result
	traceConfig := map[string]interface{}{
		"tracer":        "callTracer",
		"disableStorage": false,
		"disableMemory":  false, 
		"disableStack":   false,
		"timeout":        "10s",
	}

	var traceResult map[string]interface{}
	err := rCtx.EthCli.Client().CallContext(context.Background(), &traceResult, string(MethodNameDebugTraceTransaction), txHash, traceConfig)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceTransaction,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "debug",
		}, nil
	}

	// Validate trace result structure based on real network responses
	validationErrors := []string{}
	
	if traceResult == nil {
		validationErrors = append(validationErrors, "trace result is null")
	} else {
		// Check for callTracer format fields: {from, gas, gasUsed, input, output, to, type, value}
		requiredFields := []string{"from", "gas", "gasUsed", "to", "type"}
		for _, field := range requiredFields {
			if _, exists := traceResult[field]; !exists {
				validationErrors = append(validationErrors, fmt.Sprintf("missing callTracer field '%s'", field))
			}
		}
		
		// Validate specific field types and formats
		if gasStr, ok := traceResult["gas"].(string); ok {
			if !strings.HasPrefix(gasStr, "0x") {
				validationErrors = append(validationErrors, "gas field should be hex string with 0x prefix")
			}
		}
		
		if gasUsedStr, ok := traceResult["gasUsed"].(string); ok {
			if !strings.HasPrefix(gasUsedStr, "0x") {
				validationErrors = append(validationErrors, "gasUsed field should be hex string with 0x prefix")
			}
		}

		if typeStr, ok := traceResult["type"].(string); ok {
			validTypes := []string{"CALL", "DELEGATECALL", "STATICCALL", "CREATE", "CREATE2"}
			isValidType := false
			for _, vt := range validTypes {
				if typeStr == vt {
					isValidType = true
					break
				}
			}
			if !isValidType {
				validationErrors = append(validationErrors, fmt.Sprintf("invalid call type '%s'", typeStr))
			}
		}
	}

	// Get transaction receipt to validate consistency
	receipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), txHash)
	if err == nil && receipt != nil {
		// Validate that trace gas matches receipt gas
		if gasUsedStr, ok := traceResult["gasUsed"].(string); ok {
			expectedGas := fmt.Sprintf("0x%x", receipt.GasUsed)
			if gasUsedStr != expectedGas {
				validationErrors = append(validationErrors, fmt.Sprintf("gas mismatch: trace=%s, receipt=%s", gasUsedStr, expectedGas))
			}
		}
	}

	// Return validation results
	if len(validationErrors) > 0 {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceTransaction,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Trace validation failed: %s", strings.Join(validationErrors, ", ")),
			Category: "debug",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugTraceTransaction,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Transaction traced and validated (tx: %s, type: %v, gas: %v)", txHash.Hex()[:10]+"...", traceResult["type"], traceResult["gasUsed"]),
		Category: "debug",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

func DebugPrintBlock(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameDebugPrintBlock); result != nil {
		return result, nil
	}

	// Get current block number
	blockNumber, err := rCtx.EthCli.BlockNumber(context.Background())
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugPrintBlock,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Failed to get block number: %v", err),
			Category: "debug",
		}, nil
	}

	var blockString string
	err = rCtx.EthCli.Client().CallContext(context.Background(), &blockString, string(MethodNameDebugPrintBlock), blockNumber)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugPrintBlock,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "debug",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugPrintBlock,
		Status:   types.Ok,
		Value:    "Block printed successfully",
		Category: "debug",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

func DebugSetBlockProfileRate(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameDebugSetBlockProfileRate); result != nil {
		return result, nil
	}

	// Set a test profile rate (1 for enabled, 0 for disabled)
	rate := 1
	
	err := rCtx.EthCli.Client().CallContext(context.Background(), nil, string(MethodNameDebugSetBlockProfileRate), rate)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugSetBlockProfileRate,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "debug",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugSetBlockProfileRate,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Block profile rate set to %d", rate),
		Category: "debug",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

func DebugSetMutexProfileFraction(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameDebugSetMutexProfileFraction); result != nil {
		return result, nil
	}

	// Set a test mutex profile fraction (1 for enabled, 0 for disabled)
	fraction := 1
	
	err := rCtx.EthCli.Client().CallContext(context.Background(), nil, string(MethodNameDebugSetMutexProfileFraction), fraction)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugSetMutexProfileFraction,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "debug",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugSetMutexProfileFraction,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Mutex profile fraction set to %d", fraction),
		Category: "debug",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

func DebugSetGCPercent(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameDebugSetGCPercent); result != nil {
		return result, nil
	}

	// Set a test GC percentage (100 is default)
	percent := 100
	
	var previousPercent int
	err := rCtx.EthCli.Client().CallContext(context.Background(), &previousPercent, string(MethodNameDebugSetGCPercent), percent)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugSetGCPercent,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "debug",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugSetGCPercent,
		Status:   types.Ok,
		Value:    fmt.Sprintf("GC percent set to %d (previous: %d)", percent, previousPercent),
		Category: "debug",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

func DebugIntermediateRoots(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameDebugIntermediateRoots); result != nil {
		return result, nil
	}

	// Need a block hash - use one from our processed transactions
	if len(rCtx.ProcessedTransactions) == 0 {
		return &types.RpcResult{
			Method:   MethodNameDebugIntermediateRoots,
			Status:   types.Error,
			ErrMsg:   "No processed transactions available",
			Category: "debug",
		}, nil
	}

	receipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), rCtx.ProcessedTransactions[0])
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugIntermediateRoots,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Failed to get transaction receipt: %v", err),
			Category: "debug",
		}, nil
	}

	var roots []string
	err = rCtx.EthCli.Client().CallContext(context.Background(), &roots, string(MethodNameDebugIntermediateRoots), receipt.BlockHash, nil)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugIntermediateRoots,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "debug",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugIntermediateRoots,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Retrieved %d intermediate roots", len(roots)),
		Category: "debug",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

func DebugTraceBlockByHash(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameDebugTraceBlockByHash); result != nil {
		return result, nil
	}

	// Need a block hash - use one from our processed transactions
	if len(rCtx.ProcessedTransactions) == 0 {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlockByHash,
			Status:   types.Error,
			ErrMsg:   "No processed transactions available",
			Category: "debug",
		}, nil
	}

	receipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), rCtx.ProcessedTransactions[0])
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlockByHash,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Failed to get transaction receipt: %v", err),
			Category: "debug",
		}, nil
	}

	// Call the debug API with callTracer for structured output
	traceConfig := map[string]interface{}{
		"tracer": "callTracer",
	}
	
	var traceResults interface{}
	err = rCtx.EthCli.Client().CallContext(context.Background(), &traceResults, string(MethodNameDebugTraceBlockByHash), receipt.BlockHash, traceConfig)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlockByHash,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "debug",
		}, nil
	}

	// Simple validation - just check that we got a non-nil response
	if traceResults == nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlockByHash,
			Status:   types.Error,
			ErrMsg:   "trace result is null",
			Category: "debug",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugTraceBlockByHash,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Block traced successfully (hash: %s)", receipt.BlockHash.Hex()[:10]+"..."),
		Category: "debug",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

func DebugTraceBlockByNumber(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameDebugTraceBlockByNumber); result != nil {
		return result, nil
	}

	// Get current block number
	blockNumber, err := rCtx.EthCli.BlockNumber(context.Background())
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlockByNumber,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Failed to get block number: %v", err),
			Category: "debug",
		}, nil
	}

	blockNumberHex := fmt.Sprintf("0x%x", blockNumber)
	
	// Call the debug API
	var traceResults []interface{}
	traceConfig := map[string]interface{}{
		"tracer": "callTracer",
	}
	
	err = rCtx.EthCli.Client().CallContext(context.Background(), &traceResults, string(MethodNameDebugTraceBlockByNumber), blockNumberHex, traceConfig)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlockByNumber,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "debug",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugTraceBlockByNumber,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Traced block by number with %d results", len(traceResults)),
		Category: "debug",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

func DebugGcStats(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameDebugGcStats); result != nil {
		return result, nil
	}

	var gcStats interface{}
	err := rCtx.EthCli.Client().CallContext(context.Background(), &gcStats, string(MethodNameDebugGcStats))
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugGcStats,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "debug",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugGcStats,
		Status:   types.Ok,
		Value:    "GC statistics retrieved successfully",
		Category: "debug",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

func DebugFreeOSMemory(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameDebugFreeOSMemory); result != nil {
		return result, nil
	}

	err := rCtx.EthCli.Client().CallContext(context.Background(), nil, string(MethodNameDebugFreeOSMemory))
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugFreeOSMemory,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "debug",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugFreeOSMemory,
		Status:   types.Ok,
		Value:    "OS memory freed successfully",
		Category: "debug",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

func DebugStacks(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameDebugStacks); result != nil {
		return result, nil
	}

	var stacks string
	err := rCtx.EthCli.Client().CallContext(context.Background(), &stacks, string(MethodNameDebugStacks))
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugStacks,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "debug",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugStacks,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Stack trace retrieved (%d characters)", len(stacks)),
		Category: "debug",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

func DebugMutexProfile(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameDebugMutexProfile); result != nil {
		return result, nil
	}

	// Call debug_mutexProfile with test parameters
	filename := "/tmp/mutex_profile.out"
	duration := 1 // 1 second duration for testing
	
	err := rCtx.EthCli.Client().CallContext(context.Background(), nil, string(MethodNameDebugMutexProfile), filename, duration)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugMutexProfile,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "debug",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugMutexProfile,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Mutex profile written to %s for %d seconds", filename, duration),
		Category: "debug",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

func DebugCpuProfile(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameDebugCpuProfile); result != nil {
		return result, nil
	}

	// Call debug_cpuProfile with test parameters
	filename := "/tmp/cpu_profile.out"
	duration := 1 // 1 second duration for testing
	
	err := rCtx.EthCli.Client().CallContext(context.Background(), nil, string(MethodNameDebugCpuProfile), filename, duration)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugCpuProfile,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "debug",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugCpuProfile,
		Status:   types.Ok,
		Value:    fmt.Sprintf("CPU profile written to %s for %d seconds", filename, duration),
		Category: "debug",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

func DebugGoTrace(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameDebugGoTrace); result != nil {
		return result, nil
	}

	// Call debug_goTrace with test parameters
	filename := "/tmp/go_trace.out"
	duration := 1 // 1 second duration for testing
	
	err := rCtx.EthCli.Client().CallContext(context.Background(), nil, string(MethodNameDebugGoTrace), filename, duration)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugGoTrace,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "debug",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugGoTrace,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Go trace written to %s for %d seconds", filename, duration),
		Category: "debug",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}

func DebugBlockProfile(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(MethodNameDebugBlockProfile); result != nil {
		return result, nil
	}

	// Call debug_blockProfile with test parameters
	filename := "/tmp/block_profile.out"
	duration := 1 // 1 second duration for testing
	
	err := rCtx.EthCli.Client().CallContext(context.Background(), nil, string(MethodNameDebugBlockProfile), filename, duration)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugBlockProfile,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "debug",
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugBlockProfile,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Block profile written to %s for %d seconds", filename, duration),
		Category: "debug",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	return result, nil
}