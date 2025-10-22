package types

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	ethrpc "github.com/ethereum/go-ethereum/rpc"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
)

type Account struct {
	Address common.Address
	PrivKey *ecdsa.PrivateKey
}

// ComparisonResult holds the result of comparing API response structures between evmd and geth
type ComparisonResult struct {
	Method         string            `json:"method"`
	EvmdType       string            `json:"evmd_type"`
	GethType       string            `json:"geth_type"`
	EvmdStructure  map[string]string `json:"evmd_structure,omitempty"` // Field names -> types
	GethStructure  map[string]string `json:"geth_structure,omitempty"` // Field names -> types
	TypeMatch      bool              `json:"type_match"`
	StructureMatch bool              `json:"structure_match"`
	ErrorsMatch    bool              `json:"errors_match"`
	EvmdError      string            `json:"evmd_error,omitempty"`
	GethError      string            `json:"geth_error,omitempty"`
	Differences    []string          `json:"differences,omitempty"`
}

type TestEthClient struct {
	*ethclient.Client

	Acc *Account // account

	ERC20Addr     common.Address //  contract address
	ERC20Abi      *abi.ABI       // contract ABI
	ERC20ByteCode []byte         // contract bytecode

	ProcessedTransactions []common.Hash // txHashes
	BlockNumsIncludingTx  []uint64      // Block numbers including txHashes

	// Filter
	FilterQuery   ethereum.FilterQuery //  filter query
	FilterID      string               //  filter ID
	BlockFilterID string               //  block filter ID
}

func (c *TestEthClient) RPCClient() *ethrpc.Client {
	return c.Client.Client()
}

type RPCContext struct {
	context.Context

	Conf                 *config.Config
	Geth                 *TestEthClient
	Evmd                 *TestEthClient
	ChainID              *big.Int
	MaxPriorityFeePerGas *big.Int
	GasPrice             *big.Int

	// Dual API testing fields
	EnableComparison  bool                // Enable dual API comparison
	ComparisonResults []*ComparisonResult // Store comparison results
}

func NewRPCContext(conf *config.Config) (*RPCContext, error) {
	// Connect to the primary Ethereum client (evmd)
	ethCli, err := ethclient.Dial(conf.EvmdHttpEndpoint)
	if err != nil {
		return nil, err
	}

	gethCli, err := ethclient.Dial(conf.GethHttpEndpoint)
	if err == nil {
		log.Printf("Connected to geth at %s", conf.GethHttpEndpoint)
	}

	ecdsaPrivKey, err := crypto.HexToECDSA(conf.RichPrivKey)
	if err != nil {
		return nil, err
	}

	ctx := &RPCContext{
		Context:           context.Background(),
		Conf:              conf,
		EnableComparison:  gethCli != nil,
		ComparisonResults: make([]*ComparisonResult, 0),
		Evmd: &TestEthClient{
			Client:                ethCli,
			Acc:                   &Account{Address: crypto.PubkeyToAddress(ecdsaPrivKey.PublicKey), PrivKey: ecdsaPrivKey},
			ERC20Addr:             common.Address{},
			ProcessedTransactions: make([]common.Hash, 0),
			BlockNumsIncludingTx:  make([]uint64, 0),
			FilterQuery:           ethereum.FilterQuery{},
			FilterID:              "",
			BlockFilterID:         "",
		},
		Geth: &TestEthClient{
			Client:                gethCli,
			Acc:                   &Account{Address: crypto.PubkeyToAddress(ecdsaPrivKey.PublicKey), PrivKey: ecdsaPrivKey},
			ERC20Addr:             common.Address{},
			ProcessedTransactions: make([]common.Hash, 0),
			BlockNumsIncludingTx:  make([]uint64, 0),
			FilterQuery:           ethereum.FilterQuery{},
			FilterID:              "",
			BlockFilterID:         "",
		},
	}

	return ctx, nil
}

// CompareRPCCall performs a dual API call and compares response structures
func (rCtx *RPCContext) CompareRPCCall(method string, params ...interface{}) *ComparisonResult {
	if !rCtx.EnableComparison {
		return nil // Comparison disabled
	}

	result := &ComparisonResult{
		Method: method,
	}

	// Call evmd
	var evmdResponse interface{}
	evmdErr := rCtx.Evmd.RPCClient().CallContext(context.Background(), &evmdResponse, method, params...)
	if evmdErr != nil {
		result.EvmdError = evmdErr.Error()
	}

	// Call geth
	var gethResponse interface{}
	gethErr := rCtx.Geth.RPCClient().CallContext(context.Background(), &gethResponse, method, params...)
	if gethErr != nil {
		result.GethError = gethErr.Error()
	}

	// Compare errors
	result.ErrorsMatch = (evmdErr == nil && gethErr == nil) ||
		(evmdErr != nil && gethErr != nil)

	// Compare structure and types if both succeeded
	if evmdErr == nil && gethErr == nil {
		result.EvmdType = rCtx.getTypeDescription(evmdResponse)
		result.GethType = rCtx.getTypeDescription(gethResponse)
		result.TypeMatch = result.EvmdType == result.GethType

		result.EvmdStructure = rCtx.analyzeStructure(evmdResponse)
		result.GethStructure = rCtx.analyzeStructure(gethResponse)
		result.StructureMatch = rCtx.compareStructures(result.EvmdStructure, result.GethStructure)

		if !result.StructureMatch || !result.TypeMatch {
			result.Differences = rCtx.findStructuralDifferences(result.EvmdType, result.GethType, result.EvmdStructure, result.GethStructure)
		}
	}

	// Store the result
	rCtx.ComparisonResults = append(rCtx.ComparisonResults, result)

	return result
}

// CompareRPCCallWithProvider performs a dual API call with different parameters for each client
func (rCtx *RPCContext) CompareRPCCallWithProvider(method string, paramProvider ParameterProvider) *ComparisonResult {
	if !rCtx.EnableComparison {
		return nil // Comparison disabled
	}

	result := &ComparisonResult{
		Method: method,
	}

	// Get parameters for each client
	evmdParams := paramProvider(false) // false = evmd
	gethParams := paramProvider(true)  // true = geth

	// Call evmd
	var evmdResponse interface{}
	evmdErr := rCtx.Evmd.RPCClient().CallContext(context.Background(), &evmdResponse, method, evmdParams...)
	if evmdErr != nil {
		result.EvmdError = evmdErr.Error()
	}

	// Call geth
	var gethResponse interface{}
	gethErr := rCtx.Geth.RPCClient().CallContext(context.Background(), &gethResponse, method, gethParams...)
	if gethErr != nil {
		result.GethError = gethErr.Error()
	}

	// Compare errors
	result.ErrorsMatch = (evmdErr == nil && gethErr == nil) ||
		(evmdErr != nil && gethErr != nil)

	// Only compare structure and types if BOTH succeeded
	if evmdErr == nil && gethErr == nil {
		result.EvmdType = rCtx.getTypeDescription(evmdResponse)
		result.GethType = rCtx.getTypeDescription(gethResponse)
		result.TypeMatch = result.EvmdType == result.GethType

		result.EvmdStructure = rCtx.analyzeStructure(evmdResponse)
		result.GethStructure = rCtx.analyzeStructure(gethResponse)
		result.StructureMatch = rCtx.compareStructures(result.EvmdStructure, result.GethStructure)

		if !result.StructureMatch || !result.TypeMatch {
			result.Differences = rCtx.findStructuralDifferences(result.EvmdType, result.GethType, result.EvmdStructure, result.GethStructure)
		}
	} else {
		// If either failed, we can't compare structures meaningfully
		// This is a request failure, not a structural difference
		if evmdErr != nil && gethErr == nil {
			result.Differences = []string{"evmd request failed, geth succeeded - cannot compare structures"}
		} else if evmdErr == nil && gethErr != nil {
			result.Differences = []string{"geth request failed, evmd succeeded - cannot compare structures"}
		} else {
			result.Differences = []string{"both requests failed - cannot compare structures"}
		}
		result.StructureMatch = false
		result.TypeMatch = false
	}

	// Store the result
	rCtx.ComparisonResults = append(rCtx.ComparisonResults, result)

	return result
}

// getTypeDescription returns a string description of the response type
func (rCtx *RPCContext) getTypeDescription(response interface{}) string {
	if response == nil {
		return "null"
	}

	t := reflect.TypeOf(response)
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "int"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "uint"
	case reflect.Float32, reflect.Float64:
		return "float"
	case reflect.Slice, reflect.Array:
		return fmt.Sprintf("[]%s", rCtx.getTypeDescription(reflect.New(t.Elem()).Interface()))
	case reflect.Map:
		return "object"
	case reflect.Interface:
		// For interface{}, look at the actual value
		return rCtx.getTypeDescription(reflect.ValueOf(response).Interface())
	default:
		return t.String()
	}
}

// analyzeStructure analyzes the structure of a response object
func (rCtx *RPCContext) analyzeStructure(response interface{}) map[string]string {
	structure := make(map[string]string)

	if response == nil {
		return structure
	}

	// Convert to JSON first to normalize the structure
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		return structure
	}

	var jsonObj interface{}
	if err := json.Unmarshal(jsonBytes, &jsonObj); err != nil {
		return structure
	}

	rCtx.analyzeValue("", jsonObj, structure)
	return structure
}

// analyzeValue recursively analyzes a value and adds fields to the structure map
func (rCtx *RPCContext) analyzeValue(prefix string, value interface{}, structure map[string]string) {
	if value == nil {
		structure[prefix] = "null"
		return
	}

	switch v := value.(type) {
	case map[string]interface{}:
		if prefix != "" {
			structure[prefix] = "object"
		}
		for key, val := range v {
			fieldName := key
			if prefix != "" {
				fieldName = prefix + "." + key
			}
			rCtx.analyzeValue(fieldName, val, structure)
		}
	case []interface{}:
		structure[prefix] = "array"
		if len(v) > 0 {
			// Analyze first element to understand array structure
			rCtx.analyzeValue(prefix+"[0]", v[0], structure)
		}
	case string:
		structure[prefix] = "string"
	case float64:
		structure[prefix] = "number"
	case bool:
		structure[prefix] = "boolean"
	default:
		structure[prefix] = fmt.Sprintf("%T", v)
	}
}

// compareStructures compares two structure maps
func (rCtx *RPCContext) compareStructures(struct1, struct2 map[string]string) bool {
	if len(struct1) != len(struct2) {
		return false
	}

	for key, type1 := range struct1 {
		if type2, exists := struct2[key]; !exists || type1 != type2 {
			return false
		}
	}

	return true
}

// findStructuralDifferences finds structural differences between responses
func (rCtx *RPCContext) findStructuralDifferences(evmdType, gethType string, evmdStruct, gethStruct map[string]string) []string {
	var differences []string

	// Type comparison
	if evmdType != gethType {
		differences = append(differences, fmt.Sprintf("Root type mismatch: evmd=%s, geth=%s", evmdType, gethType))
	}

	// Find missing fields in evmd
	for field, fieldType := range gethStruct {
		if _, exists := evmdStruct[field]; !exists {
			differences = append(differences, fmt.Sprintf("Missing field in evmd: %s (%s)", field, fieldType))
		}
	}

	// Find extra fields in evmd
	for field, fieldType := range evmdStruct {
		if _, exists := gethStruct[field]; !exists {
			differences = append(differences, fmt.Sprintf("Extra field in evmd: %s (%s)", field, fieldType))
		}
	}

	// Find type mismatches
	for field, evmdFieldType := range evmdStruct {
		if gethFieldType, exists := gethStruct[field]; exists && evmdFieldType != gethFieldType {
			differences = append(differences, fmt.Sprintf("Type mismatch for %s: evmd=%s, geth=%s", field, evmdFieldType, gethFieldType))
		}
	}

	return differences
}

// ParameterProvider is a function that provides parameters for evmd and geth separately
type ParameterProvider func(isGeth bool) []interface{}

// PerformComparison performs dual API comparison with logging if enabled
func (rCtx *RPCContext) PerformComparison(methodName RpcName, params ...interface{}) {
	if !rCtx.EnableComparison {
		return
	}

	comparisonResult := rCtx.CompareRPCCall(string(methodName), params...)
	if comparisonResult != nil {
		log.Printf("Structure Comparison for %s:", methodName)
		log.Printf("  Structure Match: %v", comparisonResult.StructureMatch)
		log.Printf("  Type Match: %v (%s vs %s)", comparisonResult.TypeMatch, comparisonResult.EvmdType, comparisonResult.GethType)
		log.Printf("  Errors Match: %v", comparisonResult.ErrorsMatch)
		if len(comparisonResult.Differences) > 0 {
			log.Printf("  Structural Differences: %v", comparisonResult.Differences)
		}
	}
}

// PerformComparisonWithProvider performs dual API comparison using different parameters for each client
func (rCtx *RPCContext) PerformComparisonWithProvider(methodName RpcName, paramProvider ParameterProvider) {
	if !rCtx.EnableComparison {
		return
	}

	comparisonResult := rCtx.CompareRPCCallWithProvider(string(methodName), paramProvider)
	if comparisonResult != nil {
		log.Printf("Structure Comparison for %s:", methodName)
		log.Printf("  Structure Match: %v", comparisonResult.StructureMatch)
		log.Printf("  Type Match: %v (%s vs %s)", comparisonResult.TypeMatch, comparisonResult.EvmdType, comparisonResult.GethType)
		log.Printf("  Errors Match: %v", comparisonResult.ErrorsMatch)
		if len(comparisonResult.Differences) > 0 {
			log.Printf("  Structural Differences: %v", comparisonResult.Differences)
		}
	}
}

// GetComparisonSummary returns a summary of all comparison results
func (rCtx *RPCContext) GetComparisonSummary() map[string]int {
	if !rCtx.EnableComparison {
		return nil
	}

	summary := map[string]int{
		"total":             len(rCtx.ComparisonResults),
		"structure_matches": 0,
		"type_matches":      0,
		"error_matches":     0,
		"differences":       0,
	}

	for _, result := range rCtx.ComparisonResults {
		if result.StructureMatch {
			summary["structure_matches"]++
		}
		if result.TypeMatch {
			summary["type_matches"]++
		}
		if result.ErrorsMatch {
			summary["error_matches"]++
		}
		if len(result.Differences) > 0 {
			summary["differences"]++
		}
	}

	return summary
}

// LoadGethState populates geth with equivalent transactions for comparison using existing utilities
func (rCtx *RPCContext) LoadGethState() error {
	if !rCtx.EnableComparison || rCtx.Geth == nil {
		return nil
	}

	log.Println("Populating geth blockchain state for comparison...")

	// First, check if geth already has transactions (maybe from previous runs)
	blockNumber, err := rCtx.Geth.BlockNumber(context.Background())
	if err != nil {
		log.Printf("Warning: Could not get geth block number: %v", err)
		return nil
	}

	log.Printf("Geth current block number: %d", blockNumber)

	// If geth has transactions, scan them first
	if blockNumber > 0 {
		if err := rCtx.scanExistingGethTransactions(blockNumber); err != nil {
			log.Printf("Warning: Could not scan existing geth transactions: %v", err)
		}
	}

	// If we don't have enough geth transactions, create them using existing utilities
	if len(rCtx.Geth.ProcessedTransactions) < 3 {
		log.Printf("Creating equivalent transactions in geth using ExecuteTransactionBatch...")
		if err := rCtx.populateGethStateWithBatch(); err != nil {
			log.Printf("Warning: Could not populate geth state: %v", err)
			return nil // Don't fail completely, just limit comparison
		}
	}

	log.Printf("Geth state populated: %d transactions, contract at %s",
		len(rCtx.Geth.ProcessedTransactions), rCtx.Geth.ERC20Addr.Hex())
	return nil
}

// scanExistingGethTransactions scans existing geth blocks for transactions
func (rCtx *RPCContext) scanExistingGethTransactions(blockNumber uint64) error {
	startBlock := uint64(1)
	if blockNumber > 50 {
		startBlock = blockNumber - 50
	}

	log.Printf("Scanning existing geth blocks %d to %d...", startBlock, blockNumber)

	for i := startBlock; i <= blockNumber; i++ {
		block, err := rCtx.Geth.BlockByNumber(context.Background(), big.NewInt(int64(i)))
		if err != nil {
			continue
		}

		for _, tx := range block.Transactions() {
			txHash := tx.Hash()
			receipt, err := rCtx.Geth.TransactionReceipt(context.Background(), txHash)
			if err != nil {
				continue
			}

			if receipt.Status == 1 {
				rCtx.Geth.ProcessedTransactions = append(rCtx.Geth.ProcessedTransactions, txHash)
				rCtx.Geth.BlockNumsIncludingTx = append(rCtx.Geth.BlockNumsIncludingTx, receipt.BlockNumber.Uint64())

				if receipt.ContractAddress != (common.Address{}) {
					rCtx.Geth.ERC20Addr = receipt.ContractAddress
					log.Printf("Found existing geth contract: %s", receipt.ContractAddress.Hex())
				}
			}
		}
	}

	log.Printf("Found %d existing geth transactions", len(rCtx.Geth.ProcessedTransactions))
	return nil
}

// populateGethStateWithBatch creates equivalent transactions in geth using external utilities
// This method is designed to be called by external packages to avoid import cycles
func (rCtx *RPCContext) populateGethStateWithBatch() error {
	log.Println("Geth state population requires external ExecuteTransactionBatch call...")

	// This method serves as a placeholder - the actual population should be done
	// by calling ExecuteTransactionBatch from outside this package to avoid import cycles
	// The caller should then use UpdateGethStateFromBatch to update this context

	log.Printf("Geth state population deferred to external caller to avoid import cycles")
	return nil
}

// UpdateGethStateFromBatch updates the geth state fields from a transaction batch result
// This allows external packages to populate geth state without import cycles
func (rCtx *RPCContext) UpdateGethStateFromBatch(gethHashes []common.Hash, gethContract common.Address, gethBlocks []uint64) {
	if !rCtx.EnableComparison {
		return
	}

	// Update geth transaction hashes
	rCtx.Geth.ProcessedTransactions = append(rCtx.Geth.ProcessedTransactions, gethHashes...)

	// Update geth contract address if provided
	if gethContract != (common.Address{}) {
		rCtx.Geth.ERC20Addr = gethContract
		log.Printf("Geth contract address updated: %s", rCtx.Geth.ERC20Addr.Hex())
	}

	// Update geth block numbers
	rCtx.Geth.BlockNumsIncludingTx = append(rCtx.Geth.BlockNumsIncludingTx, gethBlocks...)

	log.Printf("Successfully updated geth state with %d transactions", len(gethHashes))
}
