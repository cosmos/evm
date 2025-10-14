package namespaces
import (
	"context"
	"fmt"
	"strings"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

const (
	NamespaceDebug = "debug"

	// Debug namespace - tracing subcategory
	MethodNameDebugTraceTransaction   types.RpcName = "debug_traceTransaction"
	MethodNameDebugTraceBlock         types.RpcName = "debug_traceBlock"
	MethodNameDebugTraceBlockByHash   types.RpcName = "debug_traceBlockByHash"
	MethodNameDebugTraceBlockByNumber types.RpcName = "debug_traceBlockByNumber"
	MethodNameDebugTraceCall          types.RpcName = "debug_traceCall"
	MethodNameDebugIntermediateRoots  types.RpcName = "debug_intermediateRoots"

	// Debug namespace - database subcategory
	MethodNameDebugDbGet               types.RpcName = "debug_dbGet"
	MethodNameDebugDbAncient           types.RpcName = "debug_dbAncient"
	MethodNameDebugChaindbCompact      types.RpcName = "debug_chaindbCompact"
	MethodNameDebugGetModifiedAccounts types.RpcName = "debug_getModifiedAccounts"
	MethodNameDebugDumpBlock           types.RpcName = "debug_dumpBlock"

	// Debug namespace - profiling subcategory
	MethodNameDebugBlockProfile            types.RpcName = "debug_blockProfile"
	MethodNameDebugCPUProfile              types.RpcName = "debug_cpuProfile"
	MethodNameDebugGoTrace                 types.RpcName = "debug_goTrace"
	MethodNameDebugMemStats                types.RpcName = "debug_memStats"
	MethodNameDebugMutexProfile            types.RpcName = "debug_mutexProfile"
	MethodNameDebugSetBlockProfileRate     types.RpcName = "debug_setBlockProfileRate"
	MethodNameDebugSetMutexProfileFraction types.RpcName = "debug_setMutexProfileFraction"

	// Debug namespace - diagnostics subcategory
	MethodNameDebugBacktraceAt  types.RpcName = "debug_backtraceAt"
	MethodNameDebugStacks       types.RpcName = "debug_stacks"
	MethodNameDebugGetBadBlocks types.RpcName = "debug_getBadBlocks"
	MethodNameDebugPreimage     types.RpcName = "debug_preimage"
	MethodNameDebugFreeOSMemory types.RpcName = "debug_freeOSMemory"
	MethodNameDebugSetHead      types.RpcName = "debug_setHead"

	// Additional debug methods from Geth documentation
	MethodNameDebugSetGCPercent                types.RpcName = "debug_setGCPercent"
	MethodNameDebugAccountRange                types.RpcName = "debug_accountRange"
	MethodNameDebugChaindbProperty             types.RpcName = "debug_chaindbProperty"
	MethodNameDebugDbAncients                  types.RpcName = "debug_dbAncients"
	MethodNameDebugFreezeClient                types.RpcName = "debug_freezeClient"
	MethodNameDebugGcStats                     types.RpcName = "debug_gcStats"
	MethodNameDebugGetAccessibleState          types.RpcName = "debug_getAccessibleState"
	MethodNameDebugGetRawBlock                 types.RpcName = "debug_getRawBlock"
	MethodNameDebugGetRawHeader                types.RpcName = "debug_getRawHeader"
	MethodNameDebugGetRawTransaction           types.RpcName = "debug_getRawTransaction"
	MethodNameDebugGetModifiedAccountsByHash   types.RpcName = "debug_getModifiedAccountsByHash"
	MethodNameDebugGetModifiedAccountsByNumber types.RpcName = "debug_getModifiedAccountsByNumber"
	MethodNameDebugGetRawReceipts              types.RpcName = "debug_getRawReceipts"
	MethodNameDebugPrintBlock                  types.RpcName = "debug_printBlock"

	// Missing debug methods from Geth documentation
	MethodNameDebugStartCPUProfile             types.RpcName = "debug_startCPUProfile"
	MethodNameDebugStopCPUProfile              types.RpcName = "debug_stopCPUProfile"
	MethodNameDebugStartGoTrace                types.RpcName = "debug_startGoTrace"
	MethodNameDebugStopGoTrace                 types.RpcName = "debug_stopGoTrace"
	MethodNameDebugTraceBadBlock               types.RpcName = "debug_traceBadBlock"
	MethodNameDebugStandardTraceBlockToFile    types.RpcName = "debug_standardTraceBlockToFile"
	MethodNameDebugStandardTraceBadBlockToFile types.RpcName = "debug_standardTraceBadBlockToFile"
	MethodNameDebugTraceBlockFromFile          types.RpcName = "debug_traceBlockFromFile"
	MethodNameDebugTraceChain                  types.RpcName = "debug_traceChain"
	MethodNameDebugStorageRangeAt              types.RpcName = "debug_storageRangeAt"
	MethodNameDebugSetTrieFlushInterval        types.RpcName = "debug_setTrieFlushInterval"
	MethodNameDebugVmodule                     types.RpcName = "debug_vmodule"
	MethodNameDebugWriteBlockProfile           types.RpcName = "debug_writeBlockProfile"
	MethodNameDebugWriteMemProfile             types.RpcName = "debug_writeMemProfile"
	MethodNameDebugWriteMutexProfile           types.RpcName = "debug_writeMutexProfile"
	MethodNameDebugVerbosity                   types.RpcName = "debug_verbosity"
)

// Debug API implementations
func DebugTraceTransaction(rCtx *types.RPCContext) (*types.RpcResult, error) {

	txHash := rCtx.Evmd.ProcessedTransactions[0]

	// Test with callTracer configuration to get structured result
	traceConfig := map[string]interface{}{
		"tracer":         "callTracer",
		"disableStorage": false,
		"disableMemory":  false,
		"disableStack":   false,
		"timeout":        "10s",
	}

	var traceResult map[string]interface{}
	err := rCtx.Evmd.RPCClient().CallContext(context.Background(), &traceResult, string(MethodNameDebugTraceTransaction), txHash, traceConfig)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceTransaction,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
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
	receipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), txHash)
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
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugTraceTransaction,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Transaction traced and validated (tx: %s, type: %v, gas: %v)", txHash.Hex()[:10]+"...", traceResult["type"], traceResult["gasUsed"]),
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugPrintBlock(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Get current block number
	blockNumber, err := rCtx.Evmd.BlockNumber(context.Background())
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugPrintBlock,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Failed to get block number: %v", err),
			Category: NamespaceDebug,
		}, nil
	}

	var blockString string
	err = rCtx.Evmd.RPCClient().CallContext(context.Background(), &blockString, string(MethodNameDebugPrintBlock), blockNumber)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugPrintBlock,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugPrintBlock,
		Status:   types.Ok,
		Value:    "Block printed successfully",
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugSetBlockProfileRate(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Set a test profile rate (1 for enabled, 0 for disabled)
	rate := 1

	err := rCtx.Evmd.RPCClient().CallContext(context.Background(), nil, string(MethodNameDebugSetBlockProfileRate), rate)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugSetBlockProfileRate,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugSetBlockProfileRate,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Block profile rate set to %d", rate),
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugSetMutexProfileFraction(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Set a test mutex profile fraction (1 for enabled, 0 for disabled)
	fraction := 1

	err := rCtx.Evmd.RPCClient().CallContext(context.Background(), nil, string(MethodNameDebugSetMutexProfileFraction), fraction)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugSetMutexProfileFraction,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugSetMutexProfileFraction,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Mutex profile fraction set to %d", fraction),
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugSetGCPercent(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Set a test GC percentage (100 is default)
	percent := 100

	var previousPercent int
	err := rCtx.Evmd.RPCClient().CallContext(context.Background(), &previousPercent, string(MethodNameDebugSetGCPercent), percent)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugSetGCPercent,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugSetGCPercent,
		Status:   types.Ok,
		Value:    fmt.Sprintf("GC percent set to %d (previous: %d)", percent, previousPercent),
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugIntermediateRoots(rCtx *types.RPCContext) (*types.RpcResult, error) {

	receipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), rCtx.Evmd.ProcessedTransactions[0])
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugIntermediateRoots,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Failed to get transaction receipt: %v", err),
			Category: NamespaceDebug,
		}, nil
	}

	var roots []string
	err = rCtx.Evmd.RPCClient().CallContext(context.Background(), &roots, string(MethodNameDebugIntermediateRoots), receipt.BlockHash, nil)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugIntermediateRoots,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugIntermediateRoots,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Retrieved %d intermediate roots", len(roots)),
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugTraceCall(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Prepare transaction args for the trace call
	fromAddr := rCtx.Evmd.Acc.Address
	toAddr := rCtx.Evmd.Acc.Address // simple transfer to self

	txArgs := map[string]interface{}{
		"from":  fromAddr.Hex(),
		"to":    toAddr.Hex(),
		"value": "0x0",
		"data":  "0x",
	}

	traceConfig := map[string]interface{}{
		"tracer": "callTracer",
	}

	// Perform dual API comparison if enabled
	rCtx.PerformComparison(MethodNameDebugTraceCall, txArgs, "latest", traceConfig)

	// Call debug_traceCall on evmd
	var traceResult map[string]interface{}
	err := rCtx.Evmd.RPCClient().CallContext(context.Background(), &traceResult,
		string(MethodNameDebugTraceCall), txArgs, "latest", traceConfig)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceCall,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugTraceCall,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Call traced successfully (type: %v)", traceResult["type"]),
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugGetRawBlock(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Get a block number from a processed transaction
	receipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), rCtx.Evmd.ProcessedTransactions[0])
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugGetRawBlock,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Failed to get transaction receipt: %v", err),
			Category: NamespaceDebug,
		}, nil
	}

	// Call debug_getRawBlock
	blockNumberOrHash := map[string]interface{}{
		"blockNumber": receipt.BlockNumber,
	}
	var blockRLP string
	err = rCtx.Evmd.RPCClient().CallContext(context.Background(), &blockRLP, string(MethodNameDebugGetRawBlock), blockNumberOrHash)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugGetRawBlock,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	// Validate that we got RLP data (should start with 0x)
	if !strings.HasPrefix(blockRLP, "0x") || len(blockRLP) < 10 {
		return &types.RpcResult{
			Method:   MethodNameDebugGetRawBlock,
			Status:   types.Error,
			ErrMsg:   "invalid RLP data format",
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugGetRawBlock,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Raw block retrieved successfully (%d bytes)", len(blockRLP)),
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugTraceBlockByHash(rCtx *types.RPCContext) (*types.RpcResult, error) {

	receipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), rCtx.Evmd.ProcessedTransactions[0])
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlockByHash,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Failed to get transaction receipt: %v", err),
			Category: NamespaceDebug,
		}, nil
	}

	// Call the debug API with callTracer for structured output
	traceConfig := map[string]interface{}{
		"tracer": "callTracer",
	}

	var traceResults interface{}
	err = rCtx.Evmd.RPCClient().CallContext(context.Background(), &traceResults, string(MethodNameDebugTraceBlockByHash), receipt.BlockHash, traceConfig)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlockByHash,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	// Simple validation - just check that we got a non-nil response
	if traceResults == nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlockByHash,
			Status:   types.Error,
			ErrMsg:   "trace result is null",
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugTraceBlockByHash,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Block traced successfully (hash: %s)", receipt.BlockHash.Hex()[:10]+"..."),
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugTraceBlock(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Get a block to trace - use the receipt's block from a processed transaction
	receipt, err := rCtx.Evmd.TransactionReceipt(context.Background(), rCtx.Evmd.ProcessedTransactions[0])
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlock,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Failed to get transaction receipt: %v", err),
			Category: NamespaceDebug,
		}, nil
	}

	// Get the full block by hash using eth_getBlockByHash
	var block map[string]interface{}
	err = rCtx.Evmd.RPCClient().CallContext(context.Background(), &block, "eth_getBlockByHash", receipt.BlockHash, true)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlock,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Failed to get block: %v", err),
			Category: NamespaceDebug,
		}, nil
	}

	// Get RLP-encoded block using debug_getRawBlock
	// Need to pass BlockNumberOrHash format
	blockNumberOrHash := map[string]interface{}{
		"blockNumber": receipt.BlockNumber,
	}
	var blockRLP string
	err = rCtx.Evmd.RPCClient().CallContext(context.Background(), &blockRLP, "debug_getRawBlock", blockNumberOrHash)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlock,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Failed to get raw block RLP: %v", err),
			Category: NamespaceDebug,
		}, nil
	}

	// Call debug_traceBlock with RLP-encoded block
	traceConfig := map[string]interface{}{
		"tracer": "callTracer",
	}

	var traceResults []interface{}
	err = rCtx.Evmd.RPCClient().CallContext(context.Background(), &traceResults, string(MethodNameDebugTraceBlock), blockRLP, traceConfig)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlock,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	// Validate trace results
	if traceResults == nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlock,
			Status:   types.Error,
			ErrMsg:   "trace result is null",
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugTraceBlock,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Block traced successfully with %d transactions", len(traceResults)),
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugTraceBlockByNumber(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Get current block number
	blockNumber, err := rCtx.Evmd.BlockNumber(context.Background())
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlockByNumber,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Failed to get block number: %v", err),
			Category: NamespaceDebug,
		}, nil
	}

	blockNumberHex := fmt.Sprintf("0x%x", blockNumber)

	// Call the debug API
	var traceResults []interface{}
	traceConfig := map[string]interface{}{
		"tracer": "callTracer",
	}

	err = rCtx.Evmd.RPCClient().CallContext(context.Background(), &traceResults, string(MethodNameDebugTraceBlockByNumber), blockNumberHex, traceConfig)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlockByNumber,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugTraceBlockByNumber,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Traced block by number with %d results", len(traceResults)),
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugGcStats(rCtx *types.RPCContext) (*types.RpcResult, error) {

	var gcStats interface{}
	err := rCtx.Evmd.RPCClient().CallContext(context.Background(), &gcStats, string(MethodNameDebugGcStats))
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugGcStats,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugGcStats,
		Status:   types.Ok,
		Value:    "GC statistics retrieved successfully",
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugFreeOSMemory(rCtx *types.RPCContext) (*types.RpcResult, error) {

	err := rCtx.Evmd.RPCClient().CallContext(context.Background(), nil, string(MethodNameDebugFreeOSMemory))
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugFreeOSMemory,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugFreeOSMemory,
		Status:   types.Ok,
		Value:    "OS memory freed successfully",
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugStacks(rCtx *types.RPCContext) (*types.RpcResult, error) {

	var stacks string
	err := rCtx.Evmd.RPCClient().CallContext(context.Background(), &stacks, string(MethodNameDebugStacks))
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugStacks,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugStacks,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Stack trace retrieved (%d characters)", len(stacks)),
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugMutexProfile(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Call debug_mutexProfile with test parameters
	filename := "/tmp/mutex_profile.out"
	duration := 1 // 1 second duration for testing

	err := rCtx.Evmd.RPCClient().CallContext(context.Background(), nil, string(MethodNameDebugMutexProfile), filename, duration)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugMutexProfile,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugMutexProfile,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Mutex profile written to %s for %d seconds", filename, duration),
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugCPUProfile(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Call debug_cpuProfile with test parameters
	filename := "/tmp/cpu_profile.out"
	duration := 1 // 1 second duration for testing

	err := rCtx.Evmd.RPCClient().CallContext(context.Background(), nil, string(MethodNameDebugCPUProfile), filename, duration)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugCPUProfile,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugCPUProfile,
		Status:   types.Ok,
		Value:    fmt.Sprintf("CPU profile written to %s for %d seconds", filename, duration),
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugGoTrace(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Call debug_goTrace with test parameters
	filename := "/tmp/go_trace.out"
	duration := 1 // 1 second duration for testing

	err := rCtx.Evmd.RPCClient().CallContext(context.Background(), nil, string(MethodNameDebugGoTrace), filename, duration)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugGoTrace,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugGoTrace,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Go trace written to %s for %d seconds", filename, duration),
		Category: NamespaceDebug,
	}
	return result, nil
}

func DebugBlockProfile(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Call debug_blockProfile with test parameters
	filename := "/tmp/block_profile.out"
	duration := 1 // 1 second duration for testing

	err := rCtx.Evmd.RPCClient().CallContext(context.Background(), nil, string(MethodNameDebugBlockProfile), filename, duration)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugBlockProfile,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}

	result := &types.RpcResult{
		Method:   MethodNameDebugBlockProfile,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Block profile written to %s for %d seconds", filename, duration),
		Category: NamespaceDebug,
	}
	return result, nil
}

// Additional debug methods from Geth documentation

// DebugStartCPUProfile starts CPU profiling
func DebugStartCPUProfile(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.Evmd.RPCClient().Call(&result, "debug_startCPUProfile", "/tmp/cpu_profile_start.out")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugStartCPUProfile,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameDebugStartCPUProfile,
		Status:   types.Ok,
		Value:    "CPU profiling started",
		Category: NamespaceDebug,
	}, nil
}

// DebugStopCPUProfile stops CPU profiling
func DebugStopCPUProfile(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.Evmd.RPCClient().Call(&result, "debug_stopCPUProfile")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugStopCPUProfile,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameDebugStopCPUProfile,
		Status:   types.Ok,
		Value:    "CPU profiling stopped",
		Category: NamespaceDebug,
	}, nil
}

// DebugTraceBadBlock traces bad blocks
func DebugTraceBadBlock(rCtx *types.RPCContext) (*types.RpcResult, error) {
	// Use a test hash to see if the method is implemented
	var result interface{}
	testHash := "0x0000000000000000000000000000000000000000000000000000000000000000"
	err := rCtx.Evmd.RPCClient().Call(&result, "debug_traceBadBlock", testHash)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBadBlock,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameDebugTraceBadBlock,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceDebug,
	}, nil
}

// DebugStandardTraceBlockToFile traces block to file
func DebugStandardTraceBlockToFile(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	testHash := "0x0000000000000000000000000000000000000000000000000000000000000000"
	config := map[string]interface{}{
		"tracer": "standardTracer",
	}
	err := rCtx.Evmd.RPCClient().Call(&result, "debug_standardTraceBlockToFile", testHash, config)
	if err != nil {
		// Check if it's a "method not found" error (API not implemented)
		if err.Error() == "the method "+string(MethodNameDebugStandardTraceBlockToFile)+" does not exist/is not available" ||
			err.Error() == "Method not found" ||
			err.Error() == string(MethodNameDebugStandardTraceBlockToFile)+" method not found" {
			return &types.RpcResult{
				Method:   MethodNameDebugStandardTraceBlockToFile,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: NamespaceDebug,
			}, nil
		}
		return &types.RpcResult{
			Method:   MethodNameDebugStandardTraceBlockToFile,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameDebugStandardTraceBlockToFile,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceDebug,
	}, nil
}

// DebugStandardTraceBadBlockToFile executes standard trace on a bad block and outputs to file
func DebugStandardTraceBadBlockToFile(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	// Test parameters for standard trace bad block to file
	testHash := "0x0000000000000000000000000000000000000000000000000000000000000000"
	config := map[string]interface{}{
		"tracer": "standardTracer",
	}

	err := rCtx.Evmd.RPCClient().Call(&result, "debug_standardTraceBadBlockToFile", testHash, config)
	if err != nil {
		// Check if it's a "method not found" error (API not implemented)
		if err.Error() == "the method "+string(MethodNameDebugStandardTraceBadBlockToFile)+" does not exist/is not available" ||
			err.Error() == "Method not found" ||
			err.Error() == string(MethodNameDebugStandardTraceBadBlockToFile)+" method not found" {
			return &types.RpcResult{
				Method:   MethodNameDebugStandardTraceBadBlockToFile,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: NamespaceDebug,
			}, nil
		}
		return &types.RpcResult{
			Method:   MethodNameDebugStandardTraceBadBlockToFile,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameDebugStandardTraceBadBlockToFile,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceDebug,
	}, nil
}

// DebugTraceBlockFromFile traces a block from file
func DebugTraceBlockFromFile(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	// Test parameters for trace block from file
	filename := "/tmp/block.rlp" // Example filename
	config := map[string]interface{}{
		"tracer": "callTracer",
	}

	err := rCtx.Evmd.RPCClient().Call(&result, "debug_traceBlockFromFile", filename, config)
	if err != nil {
		// Check if it's a "method not found" error (API not implemented)
		if err.Error() == "the method "+string(MethodNameDebugTraceBlockFromFile)+" does not exist/is not available" ||
			err.Error() == "Method not found" ||
			err.Error() == string(MethodNameDebugTraceBlockFromFile)+" method not found" {
			return &types.RpcResult{
				Method:   MethodNameDebugTraceBlockFromFile,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: NamespaceDebug,
			}, nil
		}
		return &types.RpcResult{
			Method:   MethodNameDebugTraceBlockFromFile,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameDebugTraceBlockFromFile,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceDebug,
	}, nil
}

// DebugTraceChain traces a range of blocks in the chain
func DebugTraceChain(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	// Test parameters for trace chain
	startBlock := "0x1" // Start from block 1
	endBlock := "0x2"   // End at block 2
	config := map[string]interface{}{
		"tracer":  "callTracer",
		"timeout": "10s",
	}

	err := rCtx.Evmd.RPCClient().Call(&result, "debug_traceChain", startBlock, endBlock, config)
	if err != nil {
		// Check if it's a "method not found" error (API not implemented)
		if err.Error() == "the method "+string(MethodNameDebugTraceChain)+" does not exist/is not available" ||
			err.Error() == "Method not found" ||
			err.Error() == string(MethodNameDebugTraceChain)+" method not found" {
			return &types.RpcResult{
				Method:   MethodNameDebugTraceChain,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: NamespaceDebug,
			}, nil
		}
		return &types.RpcResult{
			Method:   MethodNameDebugTraceChain,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameDebugTraceChain,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceDebug,
	}, nil
}

// DebugStorageRangeAt returns storage range at a given position
func DebugStorageRangeAt(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	// Test parameters for storage range
	testBlockHash := "0x0000000000000000000000000000000000000000000000000000000000000000"
	txIndex := 0
	contractAddr := "0x0000000000000000000000000000000000000000"
	keyStart := "0x0000000000000000000000000000000000000000000000000000000000000000"
	maxResult := 10

	err := rCtx.Evmd.RPCClient().Call(&result, "debug_storageRangeAt", testBlockHash, txIndex, contractAddr, keyStart, maxResult)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugStorageRangeAt,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameDebugStorageRangeAt,
		Status:   types.Ok,
		Value:    result,
		Category: NamespaceDebug,
	}, nil
}

// DebugSetTrieFlushInterval sets trie flush interval
func DebugSetTrieFlushInterval(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	interval := "10s" // Test interval
	err := rCtx.Evmd.RPCClient().Call(&result, "debug_setTrieFlushInterval", interval)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugSetTrieFlushInterval,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameDebugSetTrieFlushInterval,
		Status:   types.Ok,
		Value:    "Trie flush interval set to " + interval,
		Category: NamespaceDebug,
	}, nil
}

// DebugVmodule sets the logging verbosity pattern
func DebugVmodule(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	pattern := "eth/*=5" // Test verbosity pattern
	err := rCtx.Evmd.RPCClient().Call(&result, "debug_vmodule", pattern)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugVmodule,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameDebugVmodule,
		Status:   types.Ok,
		Value:    "Verbosity pattern set to " + pattern,
		Category: NamespaceDebug,
	}, nil
}

// DebugWriteBlockProfile writes block profile to file
func DebugWriteBlockProfile(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	filename := "/tmp/block_profile_write.out"
	err := rCtx.Evmd.RPCClient().Call(&result, "debug_writeBlockProfile", filename)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugWriteBlockProfile,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameDebugWriteBlockProfile,
		Status:   types.Ok,
		Value:    "Block profile written to " + filename,
		Category: NamespaceDebug,
	}, nil
}

// DebugWriteMemProfile writes memory profile to file
func DebugWriteMemProfile(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	filename := "/tmp/mem_profile_write.out"
	err := rCtx.Evmd.RPCClient().Call(&result, "debug_writeMemProfile", filename)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugWriteMemProfile,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameDebugWriteMemProfile,
		Status:   types.Ok,
		Value:    "Memory profile written to " + filename,
		Category: NamespaceDebug,
	}, nil
}

// DebugWriteMutexProfile writes mutex profile to file
func DebugWriteMutexProfile(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	filename := "/tmp/mutex_profile_write.out"
	err := rCtx.Evmd.RPCClient().Call(&result, "debug_writeMutexProfile", filename)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugWriteMutexProfile,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameDebugWriteMutexProfile,
		Status:   types.Ok,
		Value:    "Mutex profile written to " + filename,
		Category: NamespaceDebug,
	}, nil
}

// DebugVerbosity sets the log verbosity level
func DebugVerbosity(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	level := 3 // Test verbosity level (0-5)
	err := rCtx.Evmd.RPCClient().Call(&result, "debug_verbosity", level)
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNameDebugVerbosity,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNameDebugVerbosity,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Verbosity level set to %d", level),
		Category: NamespaceDebug,
	}, nil
}

// DebugStartGoTrace starts Go execution tracing
func DebugStartGoTrace(rCtx *types.RPCContext) (*types.RpcResult, error) {

	// Call debug_startGoTrace with test parameters
	filename := "/tmp/go_trace_start.out"

	var result any
	err := rCtx.Evmd.RPCClient().Call(&result, string(MethodNameDebugStartGoTrace), filename)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist/is not available") ||
			strings.Contains(err.Error(), "Method not found") ||
			strings.Contains(err.Error(), "method not found") {
			rpcResult := &types.RpcResult{
				Method:   MethodNameDebugStartGoTrace,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: NamespaceDebug,
			}
			return rpcResult, nil
		}
		rpcResult := &types.RpcResult{
			Method:   MethodNameDebugStartGoTrace,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}
		return rpcResult, nil
	}

	rpcResult := &types.RpcResult{
		Method:   MethodNameDebugStartGoTrace,
		Status:   types.Ok,
		Value:    fmt.Sprintf("Go tracing started, output to %s", filename),
		Category: NamespaceDebug,
	}
	return rpcResult, nil
}

// DebugStopGoTrace stops Go execution tracing
func DebugStopGoTrace(rCtx *types.RPCContext) (*types.RpcResult, error) {

	var result any
	err := rCtx.Evmd.RPCClient().Call(&result, string(MethodNameDebugStopGoTrace))
	if err != nil {
		if strings.Contains(err.Error(), "does not exist/is not available") ||
			strings.Contains(err.Error(), "Method not found") ||
			strings.Contains(err.Error(), "method not found") {
			rpcResult := &types.RpcResult{
				Method:   MethodNameDebugStopGoTrace,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: NamespaceDebug,
			}
			return rpcResult, nil
		}
		rpcResult := &types.RpcResult{
			Method:   MethodNameDebugStopGoTrace,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespaceDebug,
		}
		return rpcResult, nil
	}

	rpcResult := &types.RpcResult{
		Method:   MethodNameDebugStopGoTrace,
		Status:   types.Ok,
		Value:    "Go tracing stopped successfully",
		Category: NamespaceDebug,
	}
	return rpcResult, nil
}
