# Working History - JSON-RPC Simulator Refactoring

## Overview
Complete refactoring of the Cosmos EVM JSON-RPC compatibility testing framework, focusing on code organization, separation of concerns, and preparation for dual API testing (evmd vs geth comparison).

## Phase 1: Error Handling Fixes ✅

### Issue: Incorrect EthSign and EthSendTransaction Error Handling
- **Problem**: Functions treated "key not found" errors as success when keys should be available
- **Root Cause**: Missing keyring backend configuration in evmd startup script
- **Solution**: Added `--keyring-backend test` to `scripts/evmd/start-evmd.sh`
- **Files Modified**:
  - `scripts/evmd/start-evmd.sh`: Added keyring backend flag
  - `rpc/namespaces/eth.go`: Updated error handling for proper key management validation

## Phase 2: Major Code Refactoring ✅

### Issue: Poor Separation of Concerns in main.go (776 lines)
- **Problem**: Single 776-line main.go file with testCategories taking 370+ lines
- **Solution**: Systematic extraction and reorganization

#### Key Transformations:
1. **main.go**: 776 lines → 57 lines (92.6% reduction)
2. **testCategories**: Moved to `rpc/test_config.go` (370+ lines)
3. **Contract functions**: Moved to `utils/setup.go`
4. **Test execution logic**: Moved to `rpc/test_executor.go`

#### Files Created/Modified:
- `rpc/test_config.go`: Contains `GetTestCategories()` with all test configurations
- `rpc/test_executor.go`: Contains `ExecuteAllTests()` and test execution logic
- `utils/setup.go`: Contains `RunSetup()` and `RunTransactionGeneration()`
- `main.go`: Now focuses only on CLI orchestration

## Phase 3: Directory Reorganization & Cyclic Import Resolution ✅

### Initial Approach: Common vs Namespaces Split
- **Attempted**: Split into `rpc/common/` and `rpc/namespaces/` directories
- **Problem**: Circular import dependency (`common` ↔ `namespaces`)
- **Result**: Compilation failures due to cyclic imports

### Final Solution: Elegant Namespace-Based Architecture
- **Strategy**: Co-locate constants with their implementations in namespace files
- **Architecture**: Clean one-way dependency flow (`rpc` → `rpc/namespaces`)

#### Directory Structure:
```
rpc/
├── test_config.go        # References ns.MethodName + ns.Function
├── test_executor.go      # Core execution logic
├── rpc.go               # Core context and utilities
└── namespaces/          # Namespace-specific implementations
    ├── eth.go           # Ethereum constants + functions
    ├── debug.go         # Debug constants + functions
    ├── web3.go          # Web3 constants + functions
    ├── net.go           # Net constants + functions
    ├── personal.go      # Personal constants + functions
    ├── txpool.go        # TxPool constants + functions
    └── websocket.go     # WebSocket constants + functions
```

#### Key Architectural Decisions:
1. **Context Creation**: Moved `NewContext()` to `types` package to break circular dependency
2. **Constant Placement**: Each namespace file contains its own method constants
3. **Import Strategy**: One-way import from `rpc` to `rpc/namespaces` using alias `ns`
4. **Package Naming**: All files maintain `package rpc` or `package namespaces` for clarity

#### Import Pattern:
```go
// In test_config.go
import (
    ns "github.com/cosmos/evm/tests/jsonrpc/simulator/rpc/namespaces"
    // ...
)

// Usage
{Name: ns.MethodNameEthBlockNumber, Handler: ns.EthBlockNumber}
```

## Phase 4: Further Refinements ✅

### Additional Improvements Observed:
- **Context Management**: `types.NewRPCContext()` instead of `rpc.NewContext()`
- **Runner Package**: Introduction of `runner` package for test execution
- **Config Management**: Enhanced configuration handling with `config.GethVersion`
- **Report Generation**: Improved reporting with better formatting and Excel export

### Files Recently Modified:
- `main.go`: Updated imports and context creation
- `report/report.go`: Enhanced reporting with config-based geth version
- `utils/setup.go`: Comprehensive setup and transaction generation functions

## Results & Metrics

### Code Reduction:
- **main.go**: 776 lines → 57 lines (92.6% reduction)
- **Separation achieved**: 370+ lines of testCategories properly organized
- **Maintainability**: Significant improvement in code organization

### Architecture Benefits:
- ✅ **No Circular Imports**: Clean one-way dependency flow
- ✅ **Logical Grouping**: Related constants and functions co-located
- ✅ **Scalability**: Easy to add new namespaces and methods
- ✅ **Maintainability**: Clear separation between framework and implementations

### Compilation Status:
- ✅ **Build Success**: `go build .` completes without errors
- ✅ **All Imports Resolved**: No missing dependencies
- ✅ **Functional Testing**: Application runs and builds correctly

## Next Phase: Dual API Testing Framework 🎯

### Planned Implementation:
1. **Dual Client Setup**: Configure both evmd (8545) and geth (8547) clients
2. **Response Comparison**: Implement parallel API calls with format validation
3. **Geth as Criterion**: Use geth responses as the compatibility standard
4. **Format Validation**: Compare response structures and data types

### Technical Requirements:
- Add geth client configuration to RPC context
- Create comparison utilities for response validation
- Implement parallel API execution
- Add format validation against geth responses

## Technical Notes

### Import Resolution Strategy:
The key insight was avoiding bidirectional imports by:
1. Moving shared dependencies (`NewContext`) to a neutral package (`types`)
2. Co-locating constants with their implementations
3. Establishing clear import hierarchy: `rpc` → `namespaces` (one-way)

### Package Organization Philosophy:
- **Namespace packages**: Own their constants and implementations
- **Core packages**: Handle orchestration and shared utilities  
- **Types package**: Contains shared data structures and context creation
- **Utils package**: Contains setup, deployment, and utility functions

This refactoring establishes a solid foundation for implementing the dual API testing framework while maintaining clean, scalable architecture.